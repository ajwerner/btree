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

// Package orderstat provides copy-on-write ordered maps and sets with
// order-statistic queries: the rank of a key, the entry at a rank, and the
// number of entries in a key range, each in O(log n).
package orderstat

import (
	"cmp"
	"iter"

	"github.com/ajwerner/btree/aug"
)

// Map is an ordered map from K to V which additionally offers rank
// queries. It is an aug.MonoidMap counting the entries in each subtree;
// the conversion between the two is free.
type Map[K, V any] aug.MonoidMap[K, V, int]

// New constructs a Map with the provided comparison function. See
// aug.WithDegree and aug.WithFreeList for the options.
func New[K, V any](cmp func(K, K) int, opts ...aug.Option) *Map[K, V] {
	return (*Map[K, V])(aug.NewMonoid[K, V, int](cmp, aug.Count[K, V]{}, opts...))
}

// NewOrdered constructs a Map keyed by a type that supports <, ordered that
// way. It is faster than New with cmp.Compare because nodes are searched
// with < directly.
func NewOrdered[K cmp.Ordered, V any](opts ...aug.Option) *Map[K, V] {
	return (*Map[K, V])(aug.NewOrderedMonoid[K, V, int](aug.Count[K, V]{}, opts...))
}

func (m *Map[K, V]) a() *aug.MonoidMap[K, V, int] { return (*aug.MonoidMap[K, V, int])(m) }

// Rank returns the number of entries with keys less than k, and whether an
// entry with key k exists. When it does not, the rank is the position at
// which it would be inserted.
func (m *Map[K, V]) Rank(k K) (rank int, found bool) { return m.a().Prefix(k) }

// Count returns the number of entries with keys in [lo, hi).
func (m *Map[K, V]) Count(lo, hi K) int { return m.a().Aggregate(lo, hi) }

// Nth returns the entry with rank n, i.e. with exactly n entries before it.
func (m *Map[K, V]) Nth(n int) (k K, v V, ok bool) {
	it := m.Iterator()
	if !it.SeekNth(n) {
		return k, v, false
	}
	return it.Key(), it.Value(), true
}

// Clear removes all entries, returning nodes no other map references to
// the free list. See aug.Map.Clear.
func (m *Map[K, V]) Clear() { m.a().Clear() }

// Clone clones the map, lazily. It does so in constant time.
func (m *Map[K, V]) Clone() *Map[K, V] { return (*Map[K, V])(m.a().Clone()) }

// Delete removes the entry with key k, returning it.
func (m *Map[K, V]) Delete(k K) (removedK K, v V, found bool) { return m.a().Delete(k) }

// Upsert inserts or replaces the entry with key k, returning any replaced
// entry.
func (m *Map[K, V]) Upsert(k K, v V) (replacedK K, replacedV V, replaced bool) {
	return m.a().Upsert(k, v)
}

// Get returns the value for k, if any.
func (m *Map[K, V]) Get(k K) (v V, ok bool) { return m.a().Get(k) }

// Len returns the number of entries.
func (m *Map[K, V]) Len() int { return m.a().Len() }

// Height returns the height of the tree.
func (m *Map[K, V]) Height() int { return m.a().Height() }

// Degree returns the degree of the tree.
func (m *Map[K, V]) Degree() int { return m.a().Degree() }

// Compare compares two keys with the map's comparison function.
func (m *Map[K, V]) Compare(a, b K) int { return m.a().Compare(a, b) }

// String renders the tree in a Newick-like format.
func (m *Map[K, V]) String() string { return m.a().String() }

// Verify checks the tree's invariants; see aug.Map.Verify.
func (m *Map[K, V]) Verify() error { return m.a().Verify() }

// Iterator returns a new iterator positioned before the first entry.
func (m *Map[K, V]) Iterator() Iterator[K, V] {
	return Iterator[K, V]{MonoidIterator: m.a().Iterator()}
}

// Cursor returns a new cursor positioned before the first entry.
func (m *Map[K, V]) Cursor() Cursor[K, V] { return Cursor[K, V]{MonoidCursor: m.a().Cursor()} }

// All returns an iterator over every entry in key order.
func (m *Map[K, V]) All() iter.Seq2[K, V] { return m.a().All() }

// Backward returns an iterator over every entry in reverse key order.
func (m *Map[K, V]) Backward() iter.Seq2[K, V] { return m.a().Backward() }

// Range returns an iterator over the entries with keys in [lo, hi).
func (m *Map[K, V]) Range(lo, hi K) iter.Seq2[K, V] { return m.a().Range(lo, hi) }

// From returns an iterator over the entries with keys >= lo.
func (m *Map[K, V]) From(lo K) iter.Seq2[K, V] { return m.a().From(lo) }

// Min returns the entry with the smallest key.
func (m *Map[K, V]) Min() (k K, v V, ok bool) { return m.a().Min() }

// Max returns the entry with the largest key.
func (m *Map[K, V]) Max() (k K, v V, ok bool) { return m.a().Max() }

// Set is an ordered set with items of type T which additionally offers rank
// queries.
type Set[T any] Map[T, struct{}]

// NewSet constructs a Set with the provided comparison function. See
// aug.WithDegree and aug.WithFreeList for the options.
func NewSet[T any](cmp func(T, T) int, opts ...aug.Option) *Set[T] {
	return (*Set[T])(New[T, struct{}](cmp, opts...))
}

// NewOrderedSet constructs a Set of a type that supports <, ordered that
// way. See NewOrdered.
func NewOrderedSet[T cmp.Ordered](opts ...aug.Option) *Set[T] {
	return (*Set[T])(NewOrdered[T, struct{}](opts...))
}

func (s *Set[T]) m() *Map[T, struct{}] { return (*Map[T, struct{}])(s) }

// Rank returns the number of items less than item, and whether item is in
// the set.
func (s *Set[T]) Rank(item T) (rank int, found bool) { return s.m().Rank(item) }

// Count returns the number of items in [lo, hi).
func (s *Set[T]) Count(lo, hi T) int { return s.m().Count(lo, hi) }

// Nth returns the item with rank n.
func (s *Set[T]) Nth(n int) (item T, ok bool) {
	item, _, ok = s.m().Nth(n)
	return item, ok
}

// Clear removes all items, returning nodes no other set references to the
// free list.
func (s *Set[T]) Clear() { s.m().Clear() }

// Clone clones the set, lazily. It does so in constant time.
func (s *Set[T]) Clone() *Set[T] { return (*Set[T])(s.m().Clone()) }

// Upsert inserts or replaces item, returning any replaced item.
func (s *Set[T]) Upsert(item T) (replaced T, overwrote bool) {
	replaced, _, overwrote = s.m().Upsert(item, struct{}{})
	return replaced, overwrote
}

// Delete removes the item equal to item and returns it.
func (s *Set[T]) Delete(item T) (removed T, ok bool) {
	removed, _, ok = s.m().Delete(item)
	return removed, ok
}

// Contains reports whether an item equal to item is in the set.
func (s *Set[T]) Contains(item T) bool {
	_, found := s.Get(item)
	return found
}

// Get returns the item in the set equal to probe, if any. Items that
// compare equal may differ in fields the comparison ignores.
func (s *Set[T]) Get(probe T) (item T, found bool) {
	it := s.m().a().Iterator()
	if it.SeekExact(probe) {
		return it.Key(), true
	}
	return item, false
}

// Len returns the number of items.
func (s *Set[T]) Len() int { return s.m().Len() }

// Height returns the height of the tree.
func (s *Set[T]) Height() int { return s.m().Height() }

// Degree returns the degree of the tree.
func (s *Set[T]) Degree() int { return s.m().Degree() }

// Compare compares two items with the set's comparison function.
func (s *Set[T]) Compare(a, b T) int { return s.m().Compare(a, b) }

// String renders the tree in a Newick-like format.
func (s *Set[T]) String() string { return s.m().String() }

// Verify checks the tree's invariants; see aug.Map.Verify.
func (s *Set[T]) Verify() error { return s.m().Verify() }

// Iterator returns a new iterator positioned before the first item.
func (s *Set[T]) Iterator() SetIterator[T] { return SetIterator[T]{it: s.m().Iterator()} }

// Cursor returns a new cursor positioned before the first item.
func (s *Set[T]) Cursor() SetCursor[T] { return SetCursor[T]{c: s.m().Cursor()} }

// All returns an iterator over every item in order.
func (s *Set[T]) All() iter.Seq[T] { return keys(s.m().All()) }

// Backward returns an iterator over every item in reverse order.
func (s *Set[T]) Backward() iter.Seq[T] { return keys(s.m().Backward()) }

// Range returns an iterator over the items in [lo, hi).
func (s *Set[T]) Range(lo, hi T) iter.Seq[T] { return keys(s.m().Range(lo, hi)) }

// From returns an iterator over the items >= lo.
func (s *Set[T]) From(lo T) iter.Seq[T] { return keys(s.m().From(lo)) }

// Min returns the smallest item.
func (s *Set[T]) Min() (item T, ok bool) {
	item, _, ok = s.m().Min()
	return item, ok
}

// Max returns the largest item.
func (s *Set[T]) Max() (item T, ok bool) {
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

// FreeList recycles nodes between orderstat Maps and Sets with the same key
// and value types.
type FreeList[K, V any] = aug.FreeList[K, V, int]

// NewFreeList returns a FreeList that retains up to size nodes. See
// aug.NewFreeList.
func NewFreeList[K, V any](size int) FreeList[K, V] {
	return aug.NewFreeList[K, V, int](size)
}

// Iterator allows iteration through the collection. It offers all the usual
// iterator methods, plus Rank and SeekNth.
type Iterator[K, V any] struct {
	aug.MonoidIterator[K, V, int]
}

// Rank returns the number of entries before the iterator's position. If the
// iterator is past the end that is the length of the collection; if it is
// before the beginning it is 0.
func (it *Iterator[K, V]) Rank() int {
	return it.Prefix()
}

// SeekNth positions the iterator at the entry with rank nth, i.e. with
// exactly nth entries before it, and reports whether there is one.
func (it *Iterator[K, V]) SeekNth(nth int) bool {
	return seekNth(aug.LowLevel(&it.Iterator), nth)
}

// seekNth descends by subtree counts. It is what SeekPrefix does with a
// rank predicate, without the calls per child.
func seekNth[K, V any](ll *aug.LowLevelIterator[K, V, int], nth int) bool {
	it := (*aug.Iterator[K, V, int])(ll)
	it.Reset()
	n := ll.Node()
	if n == nil || nth < 0 || nth >= n.Aug() {
		if n != nil && nth >= 0 {
			ll.SetPos(n.Count())
		}
		return false
	}
	for {
		n = ll.Node()
		if n.IsLeaf() {
			ll.SetPos(int16(nth))
			return true
		}
		pos := int16(0)
		for ; ; pos++ {
			c := n.ChildAug(pos)
			if nth < c {
				break
			}
			nth -= c
			if nth == 0 {
				ll.SetPos(pos)
				return true
			}
			nth--
		}
		ll.SetPos(pos)
		ll.Descend()
	}
}

// Cursor is an Iterator that can also mutate the collection at its
// position. See aug.Cursor.
type Cursor[K, V any] struct {
	aug.MonoidCursor[K, V, int]
}

// Rank returns the number of entries before the cursor's position.
func (c *Cursor[K, V]) Rank() int {
	return c.Prefix()
}

// SeekNth positions the cursor at the entry with rank nth and reports
// whether there is one.
func (c *Cursor[K, V]) SeekNth(nth int) bool {
	return seekNth(aug.LowLevel(&c.Iterator), nth)
}

// SetIterator iterates over a set. Every positioning method reports whether
// the iterator is now at an item.
type SetIterator[T any] struct {
	it Iterator[T, struct{}]
}

// Reset marks the iterator invalid, before the first item.
func (i *SetIterator[T]) Reset() { i.it.Reset() }

// First positions the iterator at the smallest item.
func (i *SetIterator[T]) First() bool { return i.it.First() }

// Last positions the iterator at the largest item.
func (i *SetIterator[T]) Last() bool { return i.it.Last() }

// Next positions the iterator at the following item.
func (i *SetIterator[T]) Next() bool { return i.it.Next() }

// Prev positions the iterator at the preceding item.
func (i *SetIterator[T]) Prev() bool { return i.it.Prev() }

// SeekGE positions the iterator at the first item >= item.
func (i *SetIterator[T]) SeekGE(item T) bool { return i.it.SeekGE(item) }

// SeekGT positions the iterator at the first item > item.
func (i *SetIterator[T]) SeekGT(item T) bool { return i.it.SeekGT(item) }

// SeekLE positions the iterator at the last item <= item.
func (i *SetIterator[T]) SeekLE(item T) bool { return i.it.SeekLE(item) }

// SeekLT positions the iterator at the last item < item.
func (i *SetIterator[T]) SeekLT(item T) bool { return i.it.SeekLT(item) }

// SeekExact positions the iterator at item and reports whether it is in
// the set; if not, the iterator is at the first greater item.
func (i *SetIterator[T]) SeekExact(item T) bool { return i.it.SeekExact(item) }

// Valid reports whether the iterator is at an item.
func (i *SetIterator[T]) Valid() bool { return i.it.Valid() }

// Item returns the item at the iterator's position, which must be valid.
func (i *SetIterator[T]) Item() T { return i.it.Key() }

// Compare compares two items with the set's comparison function.
func (i *SetIterator[T]) Compare(a, b T) int { return i.it.Compare(a, b) }

// Rank returns the number of items before the iterator's position.
func (i *SetIterator[T]) Rank() int { return i.it.Rank() }

// SeekNth positions the iterator at the item with rank nth.
func (i *SetIterator[T]) SeekNth(nth int) bool { return i.it.SeekNth(nth) }

// SetCursor is a SetIterator that can also mutate the set at its position;
// see aug.Cursor for the rules.
type SetCursor[T any] struct {
	c Cursor[T, struct{}]
}

// Reset marks the cursor invalid, before the first item.
func (c *SetCursor[T]) Reset() { c.c.Reset() }

// First positions the cursor at the smallest item.
func (c *SetCursor[T]) First() bool { return c.c.First() }

// Last positions the cursor at the largest item.
func (c *SetCursor[T]) Last() bool { return c.c.Last() }

// Next positions the cursor at the following item.
func (c *SetCursor[T]) Next() bool { return c.c.Next() }

// Prev positions the cursor at the preceding item.
func (c *SetCursor[T]) Prev() bool { return c.c.Prev() }

// SeekGE positions the cursor at the first item >= item.
func (c *SetCursor[T]) SeekGE(item T) bool { return c.c.SeekGE(item) }

// SeekGT positions the cursor at the first item > item.
func (c *SetCursor[T]) SeekGT(item T) bool { return c.c.SeekGT(item) }

// SeekLE positions the cursor at the last item <= item.
func (c *SetCursor[T]) SeekLE(item T) bool { return c.c.SeekLE(item) }

// SeekLT positions the cursor at the last item < item.
func (c *SetCursor[T]) SeekLT(item T) bool { return c.c.SeekLT(item) }

// SeekExact positions the cursor at item and reports whether it is in the
// set; if not, the cursor is at the first greater item.
func (c *SetCursor[T]) SeekExact(item T) bool { return c.c.SeekExact(item) }

// Valid reports whether the cursor is at an item.
func (c *SetCursor[T]) Valid() bool { return c.c.Valid() }

// Item returns the item at the cursor's position, which must be valid.
func (c *SetCursor[T]) Item() T { return c.c.Key() }

// Compare compares two items with the set's comparison function.
func (c *SetCursor[T]) Compare(a, b T) int { return c.c.Compare(a, b) }

// Delete removes the current item and returns it, leaving the cursor on
// the following item or past the end; see aug.Cursor.Delete.
func (c *SetCursor[T]) Delete() T {
	item, _ := c.c.Delete()
	return item
}

// Rekey replaces the current item with item, which may sort elsewhere,
// and returns any other item equal to it that was displaced. Afterwards
// the cursor is on item.
func (c *SetCursor[T]) Rekey(item T) (displaced T, ok bool) {
	displaced, _, ok = c.c.Rekey(item)
	return displaced, ok
}

// Upsert inserts item, or replaces the equal item already present and
// returns it, using the cursor's position as a hint. Afterwards the cursor
// is on item.
func (c *SetCursor[T]) Upsert(item T) (replaced T, ok bool) {
	replaced, _, ok = c.c.Upsert(item, struct{}{})
	return replaced, ok
}

// Rank returns the number of items before the cursor's position.
func (c *SetCursor[T]) Rank() int { return c.c.Rank() }

// SeekNth positions the cursor at the item with rank nth.
func (c *SetCursor[T]) SeekNth(nth int) bool { return c.c.SeekNth(nth) }
