# Design

How the library is put together, for people changing it. The user-facing
contract is in the package documentation; this explains why it is shaped the
way it is.

## Layout

```
aug/         the B-tree: nodes, copy-on-write, iterators, cursor, augmentation
btree        Map[K, V] and Set[T] over aug with A = struct{}
orderstat    Map/Set with A = int (subtree entry counts): Rank, Nth, Count
interval     Map/Set with A = upper bound of the subtree: overlap scans
```

`aug` is the whole implementation; the other packages are thin wrappers that
fix the augmentation and add the queries it enables. Each wrapper embeds
`*aug.Map`, so the map methods are promoted and the wrapper only declares
what differs (constructors, `Clone` with the right return type, set
conveniences).

## Nodes

```go
type Node[K, V, A any] struct {
    ref      int32
    aug      A
    keys     []K              // cap 2*degree-1
    values   []V              // cap 2*degree-1; no allocation for a zero-size V
    children []*Node[K, V, A] // empty for leaves, cap 2*degree
}
```

A node of degree d holds d-1 to 2d-1 entries (the root may hold fewer).
Keys and values are separate slices so that a search touches only keys:
measured against an interleaved `{value, key}` layout, `Get` is 5-9% faster
with 8-byte values because keys are denser, and 20-30% faster with 64- and
256-byte values; the price is one more slice to copy per copied node,
about 2% on the clone-heavy round with pointer values. Copying or clearing
a node touches only the live prefix. Slices rather than arrays make the
degree a per-tree runtime setting; the earlier fixed-array layout copied
and zeroed every slot of a 127-entry node on every copy-on-write and could
not be sized per tree.

The degree default is 16. Deeper trees cost a little on point reads at
millions of entries (one more level than google/btree's default of 32) and
gain on every write after a clone, which copies one node per level.

## Ownership

Nodes are reference counted. A `Map` owns one reference to its root; an
interior node owns one reference to each child. `Clone` increments the
root's count and copies the `Map` struct: O(1), and from then on every node
is shared.

Every write descends from the root through `mut`, which returns a node the
caller may modify: the node itself when its count is 1, otherwise a copy
(`clone`) with count 1 whose children's counts have been incremented, after
which the shared original is released. A path is therefore copied at most
once per clone generation, and later writes down the same path mutate in
place. This is the CockroachDB interval B-tree scheme.

`Clear` releases the root recursively: a node whose count reaches zero
releases its children and goes back to the free list. Nodes still referenced
elsewhere stop the recursion. So clearing a clone returns exactly the nodes
that clone copied, and clearing the last tree of a family returns
everything.

The counts are atomic because clones may be used and cleared from other
goroutines. Measured, atomicity itself costs 2-7% on clone-heavy rounds;
the real cost of reference counting is touching every child's header when a
node is copied or freed, which is inherent to the scheme and the price of
recycling shared nodes (google/btree's context scheme skips the touching
and leaks shared nodes to the GC instead).

### Free lists

`FreeList` is an interface with `Get` and `Put`. The default is a bounded
mutex-guarded list of 256 nodes per tree family (clones share it); a
`sync.Pool`-backed one is available. Nodes from any degree may be reused by
any tree: `getNode` grows a slice whose capacity is too small. 256 was
chosen by measurement: a round of clone, a few hundred writes and clear
frees about one node per touched path, and 32 (google/btree's default)
spilled most of them to the GC.

## Augmentation

Each node carries an `A`. An `Updater` is called with the node and an
`UpdateInfo` after every structural change: `Insertion` and `Removal` of an
entry below the node (with the moved subtree's aggregate on rebalances),
`Split` (the node is the left half; the right half's aggregate is given),
`Replacement` of an entry with an equal key, and `Default` (recompute from
scratch). The return value says whether the node's aggregate changed and so
whether ancestors need updating; the write paths stop propagating as soon
as it is false.

Most augmentations are monoids: an aggregate is `Combine` folded over `Of`
of every entry. `MonoidUpdater` implements `Updater` for any `Monoid`, in
O(1) per level for `Group`s (which can `Uncombine`) and by recomputing the
node otherwise. An optional `Equaler` supplies the changed check and an
optional `Folder` folds a span of a node in one call, which matters because
`Of` and `Combine` are interface calls. `Count`, `Sum` and `PairOf` are the
building blocks; orderstat is `Count`.

With a monoid the map answers `Total`, `Prefix(key)` (aggregate of all
smaller keys, one descent) and `Aggregate(lo, hi)` (two boundary paths),
and the iterator answers `Prefix()` at its position and `SeekWhere(pred)`,
one descent to the first entry where a monotone predicate on the running
prefix flips. `SeekWhere` pays a predicate and a `Combine` per child
visited; orderstat's `SeekNth` is a native descent by counts for that
reason.

The interval bound is a monoid without an inverse whose useful queries are
not prefix-shaped, so `interval` implements `Updater` directly and its
overlap scan works on the `LowLevelIterator` (`Descend`, `Ascend`,
`ChildAug`, `Frame`).

Prefixes are computed lazily by walking the iterator's frames. An eager
per-frame prefix would make `Rank()` O(1) after `Next` at the cost of a
`Combine` on every step of every monoid map; measured at 20-40 ns per lazy
`Rank`, it was not worth it.

## Iterators and cursors

`Iterator` is a stateful cursor with an inline stack of ten frames (node,
position); it spills to the heap only for trees deeper than that, which
means degree 2 or 3 at large sizes. `Valid`, `Key`, `Value`, `Next`, `Prev`
and the seeks are the whole surface; `Next` past the end and `Prev` before
the beginning stay invalid, and each steps back in from the other end.

`Cursor` embeds `Iterator` and adds writes at the position. The first write
pins the path: `mut` on every frame from the root down, updating the child
pointers and the frames. `SetValue`, `Delete`, `Rekey` and a hinted `Upsert`
then act in place when the leaf can absorb the change (no underflow, no
overflow, key still between the neighbours, key within the leaf's separator
range) and run the `Updater` up the pinned path; otherwise they fall back to
the top-down algorithm and re-seek. This keeps one delete algorithm (the
top-down one) rather than adding a bottom-up rebalance.

`All`, `Backward`, `Range` and `From` are `iter.Seq2` views built on a
recursive node walk. They cost the same per entry as the iterator once the
loop body does work; with an empty body the range-over-func rewrite adds
about 2.7 ns per yield.

## Testing

Every package has a differential test against a reference model with
interleaved clones and clears across several trees, run at degrees 2, 3, 5,
16 and 64 so that splits, merges and rebalances happen constantly, and
`Map.Verify` recomputes every augmentation and checks node fill, key order
across boundaries, leaf depth, stale pointers and reference counts at every
checkpoint. Fuzz targets script the map and the cursor from bytes; a race
test runs a cursor writer against goroutines walking its snapshots. The
interval overlap scan is compared with a brute-force definition. Seeds are
random and logged.

Benchmarks compare against google/btree in `../scratch/btree-harness`
(outside the module, since it imports google/btree); the in-repo benchmarks
cover the rank operations, the cursor and iteration.
