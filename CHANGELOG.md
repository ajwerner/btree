# Changelog

## Unreleased

### API (breaking)

- Every positioning method on iterators and cursors (`First`, `Last`,
  `Next`, `Prev`, the seeks, `SeekNth`) returns whether the iterator is now
  at an entry. `SeekGE` and `SeekLT` no longer report whether the sought
  key exists; `SeekExact` does. Code that compiled against `v0.2.0` and
  used a seek's result as "found" changes meaning: replace it with
  `SeekExact`.
- Sets: `Get(probe)` returns the stored item equal to a probe; `Delete`
  returns the removed item. Set iterators and cursors are their own types
  (`SetIterator`, `SetCursor`) with `Item` and no `Value`, `SetValue` or
  phantom value arguments.
- Cursors: `Upsert` returns the replaced key and value like `Map.Upsert`;
  `Rekey` returns the entry it displaced at the destination.
- interval: queries are `Span`s (`HalfOpen`, `Point`) instead of stored
  intervals; `Overlaps(span)` returns an `OverlapIterator` whose `Next` is
  always the next overlap and whose `Seek` restarts it for another query,
  and `Iterator` is a plain iterator with no overlap mode. `Bounds.HasEnd`
  defaults to "every interval has an end; one whose end is not after its
  start is a point" instead of the zero-end convention, and the default
  tie-break treats every representation of a point at a key as the same
  item. `Bounds.CompareIntervals` is optional again as a single-call
  comparator that must order by start, which `Verify` checks. `Set` has
  `Get`, an item-returning `Delete`, `SetIterator`, `SetCursor` and
  `SetOverlapIterator`.
- aug: `Monoid`/`Group` are `CommutativeMonoid`/`CommutativeGroup` and
  require an `Identity` method; `SeekWhere(prefix, contribution)` is
  `SeekPrefix(inclusivePrefix)` with a predicate that turns true at most
  once, and returns the prefix and whether an entry was found;
  `Node.Aug` and `ChildAug` return values and `Node.SetAug` is for
  Updaters; `MonoidMap` no longer exposes an embedded `Map`; `New` panics
  on a nil comparison function. The package documents one ownership
  contract for every collection.

### Fixed

- Splits, merges and rebalances are reported to Updaters that implement
  the new `Restructurer` interface, so augmentations that depend on the
  shape of the subtree (node counts, heights) can stay correct; deleting
  an absent key that merges nodes is covered. Augmentations that depend
  only on the entries pay nothing. The `Updater` documentation states
  what an Updater is told.
- Nodes start from the monoid's `Identity`, fresh and recycled, so a
  monoid whose identity is not the zero value aggregates correctly.
- Reference counts are 64-bit; clones dropped without `Clear` can no longer
  wrap them.
- An interval whose end is not after its start is a point, so leaf matching
  and subtree pruning agree on it wherever it sits in the tree.
- `Cursor.Upsert` replaced only the value when the key compared equal to
  an existing one; it now replaces the key too and reports the previous
  key to the Updater, so key-dependent augmentations stay correct.
- The interval augmentation treated the zero endpoint as its initial
  bound, which over-estimated bounds for negative endpoints and compared
  the zero value with comparators that cannot take it (pointer endpoints
  panicked). The bound now has an explicit unset state.
- `Next` and `Prev` on an interval iterator end an overlap scan, as the
  seeks already did, so `NextOverlap` after stepping does not apply stale
  constraints.
- `Verify` no longer copies a node's reference count non-atomically while
  clones may be changing it.

### Changed

- `interval.Bounds.TieBreak` orders intervals with equal start keys; the
  tree orders by start first. `CompareIntervals` remains for a
  single-call comparator and must order by start.
- `LowLevelIterator.Config` returns a copy and `Config.Compare` has a
  value receiver; a Map's Updater and comparison function are fixed at
  construction.
- `Verify` compares augmentations with the Updater's `Equal` when it
  implements `Equaler`; `MonoidUpdater` forwards the Monoid's.
- `interval.Cursor`, `interval.FreeList` and `interval.NewFreeList` name
  the types that were only reachable through inference.
- The interval package documents its overlap cost as O(log n) plus the
  ancestors of the k matches, up to O(k log(n/k)) when they are scattered,
  rather than O(log n + k).
- `SeekWhere` documents the contract its single descent needs: the
  predicate must be exact for spans, true for a span exactly when true for
  some entry of it.
- The `aug` documentation states that maps must not be copied by value and
  that keys, values and augmentations are shallow-copied.

## v0.2.0 (2026-09-29)

A rewrite of the core with the same design (PR #3, from `1cedbd5`).

### Fixed

- `Clone` dropped every value in a node on its first copy-on-write (the
  library had only ever been exercised as a set).
- `orderstat` `Rank()` was wrong for trees of height 3 or more.
- `Next` past the end and `Prev` before the beginning panicked on interior
  roots; they now stay put, and each steps back in from the other end.
- The orderstat test package did not build under Go 1.24 or newer.
- Upserting an existing key never updated value-derived augmentations.
- The interval iterator kept an overlap scan's constraints across a plain
  seek.

### Changed (breaking)

- `internal/abstract` is the public package `aug`. Constructors are
  `New`/`NewSet` (and `NewOrdered`/`NewOrderedSet` for keys supporting `<`)
  and return pointers; `MakeMap`/`MakeSet` are gone. `Reset` is `Clear`.
- The degree is a per-tree option (`WithDegree`, default 16, was a
  compile-time 64) and free lists are pluggable (`WithFreeList`,
  `NewFreeList`, `SyncPoolFreeList`; default a bounded list of 256).
- `Iterator.Cur` is `Key`; `Node` accessors are `Aug`, `Key`, `Value`,
  `ChildAug`; `LowLevelIterator.Child` is `ChildAug`.
- `interval.New` takes a `Bounds` struct (or `BoundsOf` for types with
  `Key`/`End` methods) instead of five functions; the `Cmp` type is gone.
- `Updater.Update` receives `UpdateInfo[K, V, A]` with the entry value and
  a new `Replacement` action.
- The aggregate queries live on `MonoidMap`, `MonoidIterator` and
  `MonoidCursor`, returned by `NewMonoid`/`NewOrderedMonoid`.
- `btree.Map`, `orderstat.Map` and `interval.Map` (and their sets) are
  defined types over the `aug` types rather than structs holding a pointer:
  one allocation, free conversion, no indirection.
- Requires Go 1.26. No dependencies.

### Added

- Monoid augmentations: `Monoid`, `Group`, `Equaler`, `Folder`,
  `MonoidUpdater`, `Count`, `Sum`, `PairOf`; `Total`, `Prefix(key)`,
  `Aggregate(lo, hi)`, `Iterator.Prefix()`, `SeekWhere(pred)`.
- orderstat: `Rank(key)`, `Nth`, `Count(lo, hi)`, `Cursor`.
- `Cursor`: `SetValue`, `Delete`, `Rekey`, hinted `Upsert`, in place when
  the leaf allows it.
- `SeekGE`/`SeekLT` report exact matches; `SeekLE`, `SeekGT`.
- `All`, `Backward`, `Range`, `From` range-over-func views; `Min`, `Max`;
  `Contains` on sets; `interval.Overlapping`.
- `Map.Verify`, an invariant checker for tests.
- Differential, property, fuzz, retention and race tests across degrees;
  CI with vet, staticcheck, race tests, fuzzing and govulncheck.

### Performance

Against google/btree v1.1.3 at equal degree: parity to 10% ahead on point
operations with a comparison function, 15-25% ahead with `NewOrdered`;
the scheduler-shaped clone-and-rekey round is 20-25% faster through the
cursor. Details and method in `docs/review-2026-09-29.md`.
