# Composing generic data structures in go

* Date: 11/29/2021
* Author: Andrew Werner 

## tl;dr
[`github.com/ajwerner/btree`](https://github.com/ajwerner/btree) provides
go1.18 generic implementations of CoW btree data structures including regular
a regular map/set, order-statistic trees, and interval trees all backed by a
shared generic augmented btree implementation. Given go 1.18 hasn't been
released yet and it hasn't been heavily tested, don't use it yet. Benchmarks
and increased testing discussions to follow.

## Potentially motivating context (feel free to skip)

Recently a colleague, Nathan, reflecting on CockroachDB, remarked (paraphrased
from memory) that the key data structure is the interval btree. The story of
Nathan’s [addition of the first interval btree to cockroach and the power of
copy-on-write data structures](https://github.com/cockroachdb/cockroach/pull/31997)
is worthy of its own blog post for another day. It’s Nathan’s [hand-
specialization of that data structure](https://github.com/cockroachdb/cockroach/pull/32165) 
that provided the basis for the generalization I’ll be presenting here.
The reason for this specialization was as much for the performance wins of
avoiding excessive allocations, pointer chasing, and cost of type assertions
when using interface boxing.

I won't quarrel with the idea that interval btrees are critical, but there's
other problems out there which benefit from different types of ordered
collection data structures, such as regular ol' sorted sets and maps. The
[interval tree](https://en.wikipedia.org/wiki/Interval_tree) is just one of a
family of augmented tree search data structures. Another useful augmented tree
data structure is the [order-statistic tree](
https://en.wikipedia.org/wiki/Order_statistic_tree). 

As we all know, [generics](https://github.com/golang/go/issues/45346) are 
coming in go1.18. So what better time for an experience report building some
real things with it? This post will explore [`github.com/ajwerner/btree`](
https://github.com/ajwerner/btree/tree), a library leveraging go1.18's 
parametric polymorphism to build an abstract augmented btree which offers
rich core functionality and easy extensibility. 

The post will go through a high-level overview of the library then some
minor complaints. Another post will go through benchmarks.

## The library

The root of the module is a vanilla sorted set and map library. Here's an
example:

```go
m := btree.NewMap[string, int](strings.Compare)
m.Upsert("foo", 1)
m.Upsert("bar", 2)
fmt.Println(m.Get("foo"))
fmt.Println(m.Get("baz"))
it := m.Iterator()
for it.First(); it.Valid(); it.Next() {
    fmt.Println(it.Key(), it.Value())
}
```

The above will print:
```
1 true
0 false
foo 1
bar 2
```

Immediately, you might be thinking that that iterator is a bit strange. It's
somewhat far afield from the usual functional `(A|De)scend*` methods you'll
find in `llrb` and `google/btree`, but it's a pattern that's grown on me
and is amenable to all manner of functions to bring back similar ergonomics.
We'll break down the iterator and how to use it when I talk about gripes. For
now, I want to look at the type signature of the vanilla btree and then its
extensions.