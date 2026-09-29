# btree

[![GoDoc](https://pkg.go.dev/badge/github.com/ajwerner/btree)](https://pkg.go.dev/github.com/ajwerner/btree)
![Beta](https://img.shields.io/badge/status-beta-yellow)

A Go generic library providing copy-on-write B-tree data structures including maps, sets, interval trees, and order-statistic trees. All variants share a common augmented B-tree implementation (package `aug`) and support O(1) lazy cloning, a configurable degree, and pluggable node free lists.

**Note:** This library is still in beta. Please report any issues on the [GitHub issue tracker](https://github.com/ajwerner/btree/issues).

Read more about the design in the [blog post](./blog/blog.md) (which predates the `aug` package and the `New` constructors).

```go
m := btree.New[string, int](strings.Compare, btree.WithDegree(16))
m.Upsert("foo", 1)
snapshot := m.Clone() // O(1); writes to either side copy on write
it := snapshot.Iterator()
for it.First(); it.Valid(); it.Next() {
	fmt.Println(it.Cur(), it.Value())
}
snapshot.Clear() // returns nodes only this tree references to the free list
```

Writes to a map must be serialized by the caller; any number of goroutines may read a map, or a clone of it, while no goroutine writes it. See the `aug` package documentation for details.

## Augmentations

The `aug` package exposes the tree with a per-node augmentation of type `A` maintained through an `Updater`, plus a `LowLevelIterator` for implementing searches guided by the augmentation. The `orderstat` and `interval` packages are built on it.

## Interval Trees

The `interval` package provides interval trees for efficiently finding all intervals that overlap a query range. The iterator supports `FirstOverlap()` and `NextOverlap()` methods for querying overlapping intervals.

## Order-Statistic Trees

The `orderstat` package provides order-statistic trees that support O(log n) rank queries and nth element selection. The iterator adds `Rank()` and `SeekNth()` methods for efficient positional queries.

## License

Copyright 2021 Andrew Werner. Licensed under the Apache License, Version 2.0.
