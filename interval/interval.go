// Copyright 2021 Andrew Werner.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package interval provides copy-on-write ordered maps and sets of
// intervals whose iterators find every interval overlapping a query.
//
// Intervals are ordered by their start key; a query walks the tree
// pruning subtrees whose largest end key precedes the query's start. That
// visits O(log n) nodes plus the ancestors of the k matching intervals,
// which is O(k) when matches are adjacent and up to O(k log(n/k)) when
// they are scattered.
//
// A stored interval is a range [Key, End) or a point at Key; Bounds.HasEnd
// tells them apart, and by default an interval whose End is not after its
// Key is a point. Queries are Spans: HalfOpen(start, end) or Point(key).
package interval

import (
	"fmt"
	"iter"

	"github.com/ajwerner/btree/aug"
)

// Map is an ordered map from I to V where I is an interval. Its iterator
// provides efficient overlap queries. It is an aug.Map whose augmentation
// is the upper bound of each subtree; the conversion between the two is
// free.
type Map[I, K, V any] aug.Map[I, V, subtreeBound[K]]

// Bounds describes how to read intervals of type I whose bounds have type
// K. Compare, Key and End are required.
type Bounds[I, K any] struct {

	// Compare orders bound keys.
	Compare func(K, K) int

	// Key returns the inclusive start of an interval.
	Key func(I) K

	// End returns the exclusive end of an interval. An interval without an
	// end (see HasEnd) is a point containing only its Key.
	End func(I) K

	// HasEnd reports whether an interval has an end; one without an end is
	// a point containing only its Key. It is optional: by default an
	// interval has an end when its End is after its Key, so an empty or
	// reversed range is a point. A type that marks points some other way
	// (a nil end, a flag) provides HasEnd, which then saves a comparison
	// per bound; an interval for which it returns true must end after it
	// starts, and inserting one that does not panics.
	HasEnd func(I) bool

	// TieBreak orders intervals with equal start keys and so defines which
	// intervals a Map treats as the same key. Overlap searches rely on
	// intervals being ordered by start, so it is consulted only after the
	// starts compare equal. It is optional: by default points sort before
	// ranges, then ranges by End, and intervals equal by those are the same
	// key; an interval whose end is not after its start is a point here as
	// everywhere else.
	TieBreak func(I, I) int

	// CompareIntervals, when set, replaces the Key-then-TieBreak comparison
	// with a single call, which saves two indirect calls per probe. It must
	// order intervals by Key first; Verify reports a tree that is not.
	CompareIntervals func(I, I) int
}

// Interval is implemented by interval types that expose their own bounds:
// Key is the inclusive start and End the exclusive end.
type Interval[K any] interface {
	Key() K
	End() K
}

// BoundsOf returns Bounds for an interval type that exposes its own bounds,
// with the default HasEnd and CompareIntervals. The interval type is given
// explicitly and K is inferred: BoundsOf[span](cmp.Compare[int]).
func BoundsOf[I Interval[K], K any](compare func(K, K) int) Bounds[I, K] {
	return Bounds[I, K]{
		Compare: compare,
		Key:     I.Key,
		End:     I.End,
	}
}

func (b Bounds[I, K]) withDefaults() Bounds[I, K] {
	if b.Compare == nil || b.Key == nil || b.End == nil {
		panic("interval: Bounds.Compare, Key and End are required")
	}
	if b.HasEnd == nil {
		b.HasEnd = func(i I) bool { return b.Compare(b.End(i), b.Key(i)) > 0 }
	}
	if b.TieBreak == nil {
		b.TieBreak = func(x, y I) int {
			xPoint, yPoint := b.isPoint(x), b.isPoint(y)
			switch {
			case !xPoint && !yPoint:
				return b.Compare(b.End(x), b.End(y))
			case xPoint && yPoint:
				return 0
			case xPoint:
				return -1
			default:
				return 1
			}
		}
	}
	if b.CompareIntervals == nil {
		b.CompareIntervals = func(x, y I) int {
			if c := b.Compare(b.Key(x), b.Key(y)); c != 0 {
				return c
			}
			return b.TieBreak(x, y)
		}
	}
	return b
}

// isPoint reports whether an interval covers only its start key.
func (b Bounds[I, K]) isPoint(i I) bool {
	return !b.HasEnd(i)
}

// New constructs a Map over intervals described by b. See aug.WithDegree
// and aug.WithFreeList for the options.
func New[I, K, V any](b Bounds[I, K], opts ...aug.Option) *Map[I, K, V] {
	b = b.withDefaults()
	return (*Map[I, K, V])(aug.New[I, V, subtreeBound[K]](
		b.CompareIntervals,
		&updater[I, K, V]{
			cmp:    b.Compare,
			key:    b.Key,
			end:    b.End,
			hasEnd: b.HasEnd,
		},
		opts...,
	))
}

func (m *Map[I, K, V]) a() *aug.Map[I, V, subtreeBound[K]] {
	return (*aug.Map[I, V, subtreeBound[K]])(m)
}

// Overlaps returns an OverlapIterator positioned at the first entry whose
// interval overlaps span.
func (m *Map[I, K, V]) Overlaps(span Span[K]) OverlapIterator[I, K, V] {
	return newOverlapIterator[I, K, V](m.a(), span)
}

// Overlapping returns an iterator over the entries whose intervals overlap
// span, in order of their start keys.
func (m *Map[I, K, V]) Overlapping(span Span[K]) iter.Seq2[I, V] {
	return func(yield func(I, V) bool) {
		for it := m.Overlaps(span); it.Valid(); it.Next() {
			if !yield(it.Key(), it.Value()) {
				return
			}
		}
	}
}

// Clear removes all entries, returning nodes no other map references to
// the free list. See aug.Map.Clear.
func (m *Map[I, K, V]) Clear() { m.a().Clear() }

// Clone clones the map, lazily. It does so in constant time.
func (m *Map[I, K, V]) Clone() *Map[I, K, V] { return (*Map[I, K, V])(m.a().Clone()) }

// Delete removes the entry with key k, returning it.
func (m *Map[I, K, V]) Delete(k I) (removedK I, v V, found bool) { return m.a().Delete(k) }

// Upsert inserts or replaces the entry with key k, returning any replaced
// entry.
func (m *Map[I, K, V]) Upsert(k I, v V) (replacedK I, replacedV V, replaced bool) {
	return m.a().Upsert(k, v)
}

// Get returns the value for k, if any.
func (m *Map[I, K, V]) Get(k I) (v V, ok bool) { return m.a().Get(k) }

// Lookup returns the stored entry whose key compares equal to k, if any.
func (m *Map[I, K, V]) Lookup(k I) (key I, v V, ok bool) { return m.a().Lookup(k) }

// Len returns the number of entries.
func (m *Map[I, K, V]) Len() int { return m.a().Len() }

// Height returns the height of the tree.
func (m *Map[I, K, V]) Height() int { return m.a().Height() }

// Degree returns the degree of the tree.
func (m *Map[I, K, V]) Degree() int { return m.a().Degree() }

// Compare compares two keys with the map's comparison function.
func (m *Map[I, K, V]) Compare(a, b I) int { return m.a().Compare(a, b) }

// String renders the tree in a Newick-like format.
func (m *Map[I, K, V]) String() string { return m.a().String() }

// Verify checks the tree's invariants (see aug.Map.Verify) and that the
// intervals are ordered by start key, which a custom CompareIntervals must
// guarantee.
func (m *Map[I, K, V]) Verify() error {
	if err := m.a().Verify(); err != nil {
		return err
	}
	it := m.a().Iterator()
	u := aug.LowLevel(&it).Config().Updater.(*updater[I, K, V])
	var prev K
	first := true
	for i := range m.a().All() {
		k := u.key(i)
		if !first && u.cmp(prev, k) > 0 {
			return fmt.Errorf("interval: CompareIntervals does not order by start key")
		}
		prev, first = k, false
	}
	return nil
}

// Iterator returns a new iterator positioned before the first entry.
func (m *Map[I, K, V]) Iterator() Iterator[I, K, V] { return m.a().Iterator() }

// Cursor returns a new cursor positioned before the first entry.
func (m *Map[I, K, V]) Cursor() Cursor[I, K, V] { return m.a().Cursor() }

// All returns an iterator over every entry in key order.
func (m *Map[I, K, V]) All() iter.Seq2[I, V] { return m.a().All() }

// Backward returns an iterator over every entry in reverse key order.
func (m *Map[I, K, V]) Backward() iter.Seq2[I, V] { return m.a().Backward() }

// Range returns an iterator over the entries with keys in [lo, hi).
func (m *Map[I, K, V]) Range(lo, hi I) iter.Seq2[I, V] { return m.a().Range(lo, hi) }

// From returns an iterator over the entries with keys >= lo.
func (m *Map[I, K, V]) From(lo I) iter.Seq2[I, V] { return m.a().From(lo) }

// Min returns the entry with the smallest key.
func (m *Map[I, K, V]) Min() (k I, v V, ok bool) { return m.a().Min() }

// Max returns the entry with the largest key.
func (m *Map[I, K, V]) Max() (k I, v V, ok bool) { return m.a().Max() }

// Cursor is a cursor over a Map; see aug.Cursor.
type Cursor[I, K, V any] = aug.Cursor[I, V, subtreeBound[K]]

// FreeList recycles nodes between interval Maps and Sets with the same
// type parameters.
type FreeList[I, K, V any] = aug.FreeList[I, V, subtreeBound[K]]

// NewFreeList returns a FreeList that retains up to size nodes. See
// aug.NewFreeList.
func NewFreeList[I, K, V any](size int) FreeList[I, K, V] {
	return aug.NewFreeList[I, V, subtreeBound[K]](size)
}

// Set is an ordered set of intervals of type I with bounds of type K whose
// iterator provides efficient overlap queries.
type Set[I, K any] Map[I, K, struct{}]

// NewSet constructs a Set over intervals described by b. See New.
func NewSet[I, K any](b Bounds[I, K], opts ...aug.Option) *Set[I, K] {
	return (*Set[I, K])(New[I, K, struct{}](b, opts...))
}

func (s *Set[I, K]) m() *Map[I, K, struct{}] { return (*Map[I, K, struct{}])(s) }

// Overlaps returns a SetOverlapIterator positioned at the first item that
// overlaps span.
func (s *Set[I, K]) Overlaps(span Span[K]) SetOverlapIterator[I, K] {
	return SetOverlapIterator[I, K]{o: s.m().Overlaps(span)}
}

// SetOverlapIterator visits the items of a Set that overlap a Span, in
// order of their start keys. See OverlapIterator.
type SetOverlapIterator[I, K any] struct {
	o OverlapIterator[I, K, struct{}]
}

// Seek restarts the iterator at the first item overlapping span and
// reports whether there is one.
func (i *SetOverlapIterator[I, K]) Seek(span Span[K]) bool { return i.o.Seek(span) }

// Valid reports whether the iterator is at an overlapping item.
func (i *SetOverlapIterator[I, K]) Valid() bool { return i.o.Valid() }

// Item returns the item at the iterator's position, which must be valid.
func (i *SetOverlapIterator[I, K]) Item() I { return i.o.Key() }

// Next positions the iterator at the following overlapping item and
// reports whether there is one.
func (i *SetOverlapIterator[I, K]) Next() bool { return i.o.Next() }

// Overlapping returns an iterator over the items that overlap span, in
// order of their start keys.
func (s *Set[I, K]) Overlapping(span Span[K]) iter.Seq[I] {
	return keys(s.m().Overlapping(span))
}

// Clear removes all items, returning nodes no other set references to the
// free list.
func (s *Set[I, K]) Clear() { s.m().Clear() }

// Clone clones the set, lazily. It does so in constant time.
func (s *Set[I, K]) Clone() *Set[I, K] { return (*Set[I, K])(s.m().Clone()) }

// Upsert inserts or replaces item, returning any replaced item.
func (s *Set[I, K]) Upsert(item I) (replaced I, overwrote bool) {
	replaced, _, overwrote = s.m().Upsert(item, struct{}{})
	return replaced, overwrote
}

// Delete removes the item equal to item and returns it.
func (s *Set[I, K]) Delete(item I) (removed I, ok bool) {
	removed, _, ok = s.m().Delete(item)
	return removed, ok
}

// Contains reports whether an item equal to item is in the set.
func (s *Set[I, K]) Contains(item I) bool {
	_, ok := s.m().a().Get(item)
	return ok
}

// Get returns the item in the set equal to probe, if any. Items that
// compare equal may differ in fields the comparison ignores.
func (s *Set[I, K]) Get(probe I) (item I, found bool) {
	item, _, found = s.m().a().Lookup(probe)
	return item, found
}

// Len returns the number of items.
func (s *Set[I, K]) Len() int { return s.m().Len() }

// Height returns the height of the tree.
func (s *Set[I, K]) Height() int { return s.m().Height() }

// Degree returns the degree of the tree.
func (s *Set[I, K]) Degree() int { return s.m().Degree() }

// Compare compares two items with the set's comparison function.
func (s *Set[I, K]) Compare(a, b I) int { return s.m().Compare(a, b) }

// String renders the tree in a Newick-like format.
func (s *Set[I, K]) String() string { return s.m().String() }

// Verify checks the tree's invariants; see aug.Map.Verify.
func (s *Set[I, K]) Verify() error { return s.m().Verify() }

// Iterator returns a new iterator positioned before the first item.
func (s *Set[I, K]) Iterator() SetIterator[I, K] { return SetIterator[I, K]{it: s.m().Iterator()} }

// Cursor returns a new cursor positioned before the first item.
func (s *Set[I, K]) Cursor() SetCursor[I, K] { return SetCursor[I, K]{c: s.m().Cursor()} }

// All returns an iterator over every item in order.
func (s *Set[I, K]) All() iter.Seq[I] { return keys(s.m().All()) }

// Backward returns an iterator over every item in reverse order.
func (s *Set[I, K]) Backward() iter.Seq[I] { return keys(s.m().Backward()) }

// Range returns an iterator over the items in [lo, hi).
func (s *Set[I, K]) Range(lo, hi I) iter.Seq[I] { return keys(s.m().Range(lo, hi)) }

// From returns an iterator over the items >= lo.
func (s *Set[I, K]) From(lo I) iter.Seq[I] { return keys(s.m().From(lo)) }

// Min returns the smallest item.
func (s *Set[I, K]) Min() (item I, ok bool) {
	item, _, ok = s.m().Min()
	return item, ok
}

// Max returns the largest item.
func (s *Set[I, K]) Max() (item I, ok bool) {
	item, _, ok = s.m().Max()
	return item, ok
}

func keys[T any](seq iter.Seq2[T, struct{}]) iter.Seq[T] {
	return func(yield func(T) bool) {
		for k := range seq {
			if !yield(k) {
				return
			}
		}
	}
}

// SetIterator iterates over a set. Every positioning method reports whether
// the iterator is now at an item.
type SetIterator[I, K any] struct {
	it Iterator[I, K, struct{}]
}

// Reset marks the iterator invalid, before the first item.
func (i *SetIterator[I, K]) Reset() { i.it.Reset() }

// First positions the iterator at the smallest item.
func (i *SetIterator[I, K]) First() bool { return i.it.First() }

// Last positions the iterator at the largest item.
func (i *SetIterator[I, K]) Last() bool { return i.it.Last() }

// Next positions the iterator at the following item.
func (i *SetIterator[I, K]) Next() bool { return i.it.Next() }

// Prev positions the iterator at the preceding item.
func (i *SetIterator[I, K]) Prev() bool { return i.it.Prev() }

// SeekGE positions the iterator at the first item >= item.
func (i *SetIterator[I, K]) SeekGE(item I) bool { return i.it.SeekGE(item) }

// SeekGT positions the iterator at the first item > item.
func (i *SetIterator[I, K]) SeekGT(item I) bool { return i.it.SeekGT(item) }

// SeekLE positions the iterator at the last item <= item.
func (i *SetIterator[I, K]) SeekLE(item I) bool { return i.it.SeekLE(item) }

// SeekLT positions the iterator at the last item < item.
func (i *SetIterator[I, K]) SeekLT(item I) bool { return i.it.SeekLT(item) }

// SeekExact positions the iterator at item and reports whether it is in
// the set; if not, the iterator is at the first greater item.
func (i *SetIterator[I, K]) SeekExact(item I) bool { return i.it.SeekExact(item) }

// Valid reports whether the iterator is at an item.
func (i *SetIterator[I, K]) Valid() bool { return i.it.Valid() }

// Item returns the item at the iterator's position, which must be valid.
func (i *SetIterator[I, K]) Item() I { return i.it.Key() }

// Compare compares two items with the set's comparison function.
func (i *SetIterator[I, K]) Compare(a, b I) int { return i.it.Compare(a, b) }

// SetCursor is a SetIterator that can also mutate the set at its position;
// see aug.Cursor for the rules.
type SetCursor[I, K any] struct {
	c Cursor[I, K, struct{}]
}

// Reset marks the cursor invalid, before the first item.
func (c *SetCursor[I, K]) Reset() { c.c.Reset() }

// First positions the cursor at the smallest item.
func (c *SetCursor[I, K]) First() bool { return c.c.First() }

// Last positions the cursor at the largest item.
func (c *SetCursor[I, K]) Last() bool { return c.c.Last() }

// Next positions the cursor at the following item.
func (c *SetCursor[I, K]) Next() bool { return c.c.Next() }

// Prev positions the cursor at the preceding item.
func (c *SetCursor[I, K]) Prev() bool { return c.c.Prev() }

// SeekGE positions the cursor at the first item >= item.
func (c *SetCursor[I, K]) SeekGE(item I) bool { return c.c.SeekGE(item) }

// SeekGT positions the cursor at the first item > item.
func (c *SetCursor[I, K]) SeekGT(item I) bool { return c.c.SeekGT(item) }

// SeekLE positions the cursor at the last item <= item.
func (c *SetCursor[I, K]) SeekLE(item I) bool { return c.c.SeekLE(item) }

// SeekLT positions the cursor at the last item < item.
func (c *SetCursor[I, K]) SeekLT(item I) bool { return c.c.SeekLT(item) }

// SeekExact positions the cursor at item and reports whether it is in the
// set; if not, the cursor is at the first greater item.
func (c *SetCursor[I, K]) SeekExact(item I) bool { return c.c.SeekExact(item) }

// Valid reports whether the cursor is at an item.
func (c *SetCursor[I, K]) Valid() bool { return c.c.Valid() }

// Item returns the item at the cursor's position, which must be valid.
func (c *SetCursor[I, K]) Item() I { return c.c.Key() }

// Compare compares two items with the set's comparison function.
func (c *SetCursor[I, K]) Compare(a, b I) int { return c.c.Compare(a, b) }

// Delete removes the current item and returns it, leaving the cursor on
// the following item or past the end; see aug.Cursor.Delete.
func (c *SetCursor[I, K]) Delete() I {
	item, _ := c.c.Delete()
	return item
}

// Rekey replaces the current item with item, which may sort elsewhere,
// and returns any other item equal to it that was displaced. Afterwards
// the cursor is on item.
func (c *SetCursor[I, K]) Rekey(item I) (displaced I, ok bool) {
	displaced, _, ok = c.c.Rekey(item)
	return displaced, ok
}

// Upsert inserts item, or replaces the equal item already present and
// returns it, using the cursor's position as a hint. Afterwards the cursor
// is on item.
func (c *SetCursor[I, K]) Upsert(item I) (replaced I, ok bool) {
	replaced, _, ok = c.c.Upsert(item, struct{}{})
	return replaced, ok
}
