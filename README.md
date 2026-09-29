# btree

[![GoDoc](https://pkg.go.dev/badge/github.com/ajwerner/btree)](https://pkg.go.dev/github.com/ajwerner/btree)
![Beta](https://img.shields.io/badge/status-beta-yellow)

A Go generic library providing copy-on-write B-tree data structures including maps, sets, interval trees, and order-statistic trees. All variants share a common augmented B-tree implementation (package `aug`) and support O(1) lazy cloning, a configurable degree, and pluggable node free lists.

**Note:** This library is still in beta. Please report any issues on the [GitHub issue tracker](https://github.com/ajwerner/btree/issues).

Read about the design in [docs/design.md](./docs/design.md); the original [blog post](./blog/blog.md) predates the `aug` package and the `New` constructors.

```go
m := btree.NewOrdered[string, int](btree.WithDegree(16)) // or New(strings.Compare, ...)
m.Upsert("foo", 1)
snapshot := m.Clone() // O(1); writes to either side copy on write
it := snapshot.Iterator()
for it.First(); it.Valid(); it.Next() {
	fmt.Println(it.Key(), it.Value())
}
for k, v := range snapshot.Range("a", "n") { // also All, Backward, From
	fmt.Println(k, v)
}
snapshot.Clear() // returns nodes only this tree references to the free list
```

The stateful `Iterator` and the range-over-func views cost about the same per entry when the loop body does real work; the iterator is the one to use for seeking and stepping, the views for whole-range walks.

A `Cursor` is an iterator that can also mutate the entry it is on and stay valid: `SetValue`, `Delete` (leaves the cursor on the successor), `Rekey` (moves the entry to a new key) and a hinted `Upsert`, each returning the displaced entry as the map methods do. Each acts in place when the leaf allows it and otherwise falls back to the top-down algorithm plus a re-seek, so "seek, then move this entry" costs one descent instead of three.

Sets are keyed by a comparison function, so two items that compare equal are the same item; `Set.Get(probe)` returns the stored one. Set iterators and cursors have no phantom values.

Writes to a map must be serialized by the caller; any number of goroutines may read a map, or a clone of it, while no goroutine writes it. See the `aug` package documentation for details.

## Augmentations

The `aug` package exposes the tree with a per-node augmentation of type `A` maintained through an `Updater`, plus a `LowLevelIterator` for implementing searches guided by the augmentation. The `orderstat` and `interval` packages are built on it.

## Performance

Against google/btree v1.1.3 at equal degree, point operations (insert, get, delete, seek and scan) are at parity or ahead by 5-15% with a comparison function, and ahead by 20-25% with `NewOrdered` for keys that support `<` (ints, strings, floats). On a scheduler-shaped workload (clone a 20k-entry map with 24-byte keys, move 200 entries to new keys, release the clone) a round takes 50 µs with delete + upsert and 42 µs through a `Cursor`, against 53 µs for google/btree at degree 16, with two allocations per round. Numbers and method are in `docs/review-2026-09-29.md`.

## Interval Trees

The `interval` package provides interval trees for efficiently finding all intervals that overlap a query. Stored intervals are described by `Bounds` (how to read a start and end from your type); queries are `Span`s built with `HalfOpen(start, end)` or `Point(key)`, so asking about a range never requires building a stored object. `Overlaps(span)` returns an `OverlapIterator` whose `Next` always means the next overlap and whose `Seek(span)` reuses it for the next query; `Overlapping(span)` is the range-over-func form.

## Order-Statistic Trees

The `orderstat` package provides order-statistic trees: `Rank(key)` (the number of keys less than a key, and whether the key exists), `Nth(rank)`, and `Count(lo, hi)`, each in one descent. The iterator adds `Rank()` for its current position and `SeekNth(rank)`.

## Custom augmentations

Describe an augmentation as a `CommutativeMonoid` (an `Identity`, a per-entry contribution and a commutative, associative `Combine`; add `Uncombine` for O(1) removals) and build the map with `aug.NewMonoid(cmp, m)`. Ready-made pieces: `aug.Count`, `aug.Sum(f)`, and `aug.PairOf(a, b)` to keep several side by side. A `MonoidMap` answers `Total()`, `Prefix(key)` and `Aggregate(lo, hi)`, and its iterators and cursors answer `Prefix()` and `SeekPrefix(pred)`, which descends once to the first entry whose inclusive prefix satisfies a predicate that turns from false to true at most once (selection by rank, by cumulative weight, and so on). See `Example_customAugmentation` in package `aug`. Augmentations that are not monoids (the interval tree's upper bound) implement `aug.Updater` directly with `aug.New`.

## License

Copyright 2021 Andrew Werner. Licensed under the Apache License, Version 2.0.
