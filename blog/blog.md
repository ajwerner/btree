# Composing generic data structures in go

* Date: 11/29/2021
* Author: Andrew Werner 

## tl;dr

[`github.com/ajwerner/btree`](https://github.com/ajwerner/btree) provides go1.18
generic implementations of CoW btree data structures including regular a regular
map/set, order-statistic trees, and interval trees all backed by a shared,
generic augmented btree implementation. Given go 1.18 hasn't been released yet
and it hasn't been heavily tested, don't use it yet. Benchmarks and a deeper
discussion on the patterns and experience to follow.

## Potentially motivating context (feel free to skip)

Recently a colleague, Nathan, reflecting on CockroachDB, remarked (paraphrased
from memory) that the key data structure is the interval btree. The story of
Nathan’s [addition of the first interval btree to cockroach and the power of
copy-on-write data structures](
https://github.com/cockroachdb/cockroach/pull/31997) is worthy of its own blog
post for another day. It’s Nathan’s [hand-specialization of that data
structure](https://github.com/cockroachdb/cockroach/pull/32165) that provided
the basis (and tests) for the generalization I’ll be presenting here. The reason
for this specialization was as much for the performance wins of avoiding
excessive allocations, pointer chasing, and cost of type assertions when using
interface boxing.

I won't quarrel with the idea that interval btrees are critical, but there's
other problems out there which benefit from different types of ordered
collection data structures, like regular ol' sorted sets and maps. I'm not
pleased with what's currently on offer for go in that space. Anyway, an
[interval tree](https://en.wikipedia.org/wiki/Interval_tree) is just one of a
family of augmented tree search data structures. Another useful augmented tree
data structure is the [order-statistic tree](
https://en.wikipedia.org/wiki/Order_statistic_tree). Wouldn't it be great if we
could have a solid implementation of all the various search trees?

As we all know, [generics](https://github.com/golang/go/issues/45346) are coming
in go1.18. So what better time for an experience report building some real
things with it? This post will explore [`github.com/ajwerner/btree`](
https://github.com/ajwerner/btree), a library leveraging go1.18's  parametric
polymorphism to build an abstract augmented btree which offers rich core
functionality and easy extensibility.

The post will go through a high-level overview of the library. Some follow-up
posts will cover benchmarks, iterator patterns, and some gripes.

## The library

The root of the module is a vanilla sorted set and map library. Here's an
example:

```go
m := btree.MakeMap[string, int](strings.Compare)
m.Upsert("foo", 1)
m.Upsert("bar", 2)
fmt.Println(m.Get("foo"))
fmt.Println(m.Get("baz"))
it := m.Iterator()
for it.First(); it.Valid(); it.Next() {
    fmt.Println(it.Cur(), it.Value())
}
// Output:
// 1 true
// 0 false
// foo 1
// bar 2
```


Let's have a look at this `Map`:

```go
// Map is a ordered map from K to V.
type Map[K, V any] struct {
	abstract.Map[K, V, struct{}]
}
```

Here we see that we embed something called abstract.Map. This `abstract.Map` is
a generalization of a sorted map that also allows for extra state as specified
by its final type parameter. In this case, for a regular btree, it's set to
`struct{}` is the auxiliary configuration state for the tree.

This augmentation parameter allows for implementers to provide some extra state
which will get plumbed down into both the augmentation and the iterator. This is
useful for, say, comparison functions or other functions to manipulate the key
data. The last two type parameters refer to the augmentation which lives in each
key node. Let's have a look at the implementation:

### Package `orderstat`

Now, how do we use this augmentation? Armed with this generalization, we can
build fancy things like an order-statistic tree with very little effort. This
[`ordered.Compare`](https://github.com/ajwerner/btree/blob/192bbdace38ee872480f4fb861d67bd2bafb740e/internal/ordered/compare.go#L21-L30)
thing is just a little generic helper to make a comparison function for any
type.

```go
s := orderstat.MakeSet(ordered.Compare[int])
for _, i := range rand.Perm(100) {
    s.Upsert(i)
}
fmt.Println(s.Len())
it := m.Iterator()
it.SeekNth(90)
it.Println(s.Cur())
// Output:
// 100
// 90
```

So, what does this augmentation look like? This:

```go
// Set is an ordered set with items of type T which additionally offers the
// methods of an order-statistic tree on its iterator.
type Set[T any] Map[T, struct{}]

// Map is a ordered map from K to V which additionally offers the methods
// of a order-statistic tree on its iterator.
type Map[K, V any] struct {
	abstract.Map[K, V, aug]
}

type aug struct {
	// children is the number of items rooted at the current subtree.
	children int
}
```

Now, in order to make this useful, we implement the `Updater[K, A]` interface
which can be used to update the `aug` method on the `aug` and we add some
special behavior to the `Iterator`. In order to empower the `orderstat` library
to do something useful with its iterator, we need to expose low-level iterator
operations to it. This is done through a constructor in the `internal/abstract`
library which can be used to trade a regular `Iterator` for a
`LowLevelIterator`. You can see this in action
[here](https://github.com/ajwerner/btree/blob/192bbdace38ee872480f4fb861d67bd2bafb740e/orderstat/order_stat.go#L102-L132).

The `Updater` interface is used to update the augmented data when the node is
modified. The `UpdateMeta` has extra information which can be used to optimize
the update.

```go
// Updater is used to update the augmentation of the node when the subtree
// changes.
type Updater[K, A any] interface {

	// Update should update the augmentation of the passed node, optionally
	// using the data in the UpdataMeta to optimize the update. If the
	// augmentation changed, and thus, changes should occur in the ancestors
	// of the subtree rooted at this node, return true.
	Update(Node[K, A], UpdateMeta[K, A]) (changed bool)
}


// Node represents an abstraction of a node exposed to the
// augmentation and low-level iteration primitives.
type Node[K, A any] interface {
	GetA() *A

	// IsLeaf returns whether this node is a leaf.
	IsLeaf() bool

	// Count returns the number of keys and entries in this node.
	Count() int16

	// GetKey returns the key at the given position. It may be
	// called with values in [0, Count()) if Count() is greater than 0.
	GetKey(i int16) K

	// GetChild returns the augmentation of the child at the given position. It
	// may be called on non-leaf nodes with values in [0, Count()] if Count()
	// is greater than 0.
	GetChild(i int16) *A
}
```

### Package `interval`

Another cool augmented search tree is the interval tree which lets you search
for overlaps. 

```go
type pair[T constraints.Ordered] [2]T
func (p pair[T]) first() T
func (p pair[T]) second() T
func (p pair[T]) compare(o pair[T]) int {
    if c := ordered.Compare(p[0], o[0]); c != 0 {
        return c
    }
    return ordered.Compare(p[1], o[1])
}
```
```go
m := interval.MakeSet(
    ordered.Compare[int],
    pair[int].compare,
    pair[int].first,
    pair[int].second,
)
for _, p := range []pair{
    {1, 2}, {2, 3}, {1, 5}, {0, 6}, {2, 7},
} {
    m.Upsert(p)
}
it := m.Iterator()
for it.FirstOverlap(pair{4, 5}); it.Valid(); it.NextOverlap() {
    fmt.Println(it.Cur())
}
// Output:
// [0 6]
// [1 5]
// [2 7]
```

### Recap

We've now got a set of search tree structure which are generic, zero-allocation
and share a common core. Each of them offer an `O(1)` `Clone()` operation, which
is very handy. The business logic related to the augmented search doesn't get
mixed up with the business logic of the tree itself. The performance is on par
with the hand-specialized interval variant (benchmarks coming later). Expect a
follow-up post at some point about rough edges and patterns in the
implementation. All in all, I'm excited about the prospect of using this in 2022
in real code.

## Special Thanks

A big thank you to Nathan VanBenschoten
([@natevanben](https://twitter.com/natevanben)) who wrote the wonderful code
that inspired everything here.

Also, shout out to Roger Peppe ([@rogpeppe](https://twitter.com/rogpeppe)) who
helped me grapple with the type sets change in the `go2go` days and helped make
this library actually good after I published the first draft of it.