// Copyright 2026 Andrew Werner.
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
	"iter"

	"github.com/ajwerner/btree/aug"
)

// Map is an ordered map from K to V which additionally offers rank
// queries. Its augmentation is the number of entries in each subtree.
type Map[K, V any] struct {
	*aug.Map[K, V, int]
}

// New constructs a Map with the provided comparison function. See
// aug.WithDegree and aug.WithFreeList for the options.
func New[K, V any](cmp func(K, K) int, opts ...aug.Option) *Map[K, V] {
	return &Map[K, V]{Map: aug.New[K, V, int](cmp, aug.MonoidUpdater[K, V, int](aug.Count[K, V]{}), opts...)}
}

// Iterator constructs a new Iterator for this Map.
func (t *Map[K, V]) Iterator() Iterator[K, V] {
	return Iterator[K, V]{Iterator: t.Map.Iterator()}
}

// Cursor constructs a new Cursor for this Map.
func (t *Map[K, V]) Cursor() Cursor[K, V] {
	return Cursor[K, V]{Cursor: t.Map.Cursor()}
}

// Clone clones the Map, lazily. It does so in constant time.
func (t *Map[K, V]) Clone() *Map[K, V] {
	return &Map[K, V]{Map: t.Map.Clone()}
}

// Rank returns the number of entries with keys less than k, and whether an
// entry with key k exists. When it does not, the rank is the position at
// which it would be inserted.
func (t *Map[K, V]) Rank(k K) (rank int, found bool) {
	return t.Map.Prefix(k)
}

// Count returns the number of entries with keys in [lo, hi).
func (t *Map[K, V]) Count(lo, hi K) int {
	return t.Map.Aggregate(lo, hi)
}

// Nth returns the entry with rank n, i.e. with exactly n entries before it.
func (t *Map[K, V]) Nth(n int) (k K, v V, ok bool) {
	it := t.Iterator()
	it.SeekNth(n)
	if !it.Valid() {
		return k, v, false
	}
	return it.Cur(), it.Value(), true
}

// Set is an ordered set with items of type T which additionally offers rank
// queries.
type Set[T any] Map[T, struct{}]

// NewSet constructs a Set with the provided comparison function. See
// aug.WithDegree and aug.WithFreeList for the options.
func NewSet[T any](cmp func(T, T) int, opts ...aug.Option) *Set[T] {
	return (*Set[T])(New[T, struct{}](cmp, opts...))
}

// Clone clones the Set, lazily. It does so in constant time.
func (t *Set[T]) Clone() *Set[T] {
	return (*Set[T])((*Map[T, struct{}])(t).Clone())
}

// Upsert inserts or updates the provided item. It returns
// the overwritten item if a previous value existed for the key.
func (t *Set[T]) Upsert(item T) (replaced T, overwrote bool) {
	replaced, _, overwrote = t.Map.Upsert(item, struct{}{})
	return replaced, overwrote
}

// Delete removes the provided item. It returns true if the item existed in
// the set.
func (t *Set[T]) Delete(item T) (removed bool) {
	_, _, removed = t.Map.Delete(item)
	return removed
}

// Contains returns true if the item exists in the set.
func (t *Set[T]) Contains(item T) bool {
	_, ok := t.Map.Get(item)
	return ok
}

// Iterator constructs an iterator for this set.
func (t *Set[T]) Iterator() Iterator[T, struct{}] {
	return (*Map[T, struct{}])(t).Iterator()
}

// Cursor constructs a cursor for this set.
func (t *Set[T]) Cursor() Cursor[T, struct{}] {
	return (*Map[T, struct{}])(t).Cursor()
}

// All returns an iterator over every item in order.
func (t *Set[T]) All() iter.Seq[T] {
	return keys(t.Map.All())
}

// Backward returns an iterator over every item in reverse order.
func (t *Set[T]) Backward() iter.Seq[T] {
	return keys(t.Map.Backward())
}

// Range returns an iterator over the items in [lo, hi) in order.
func (t *Set[T]) Range(lo, hi T) iter.Seq[T] {
	return keys(t.Map.Range(lo, hi))
}

// From returns an iterator over the items greater than or equal to lo in
// order.
func (t *Set[T]) From(lo T) iter.Seq[T] {
	return keys(t.Map.From(lo))
}

// Min returns the smallest item.
func (t *Set[T]) Min() (item T, ok bool) {
	item, _, ok = t.Map.Min()
	return item, ok
}

// Max returns the largest item.
func (t *Set[T]) Max() (item T, ok bool) {
	item, _, ok = t.Map.Max()
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

// Rank returns the number of items less than item, and whether item is in
// the set.
func (t *Set[T]) Rank(item T) (rank int, found bool) {
	return t.Map.Prefix(item)
}

// Count returns the number of items in [lo, hi).
func (t *Set[T]) Count(lo, hi T) int {
	return t.Map.Aggregate(lo, hi)
}

// Nth returns the item with rank n.
func (t *Set[T]) Nth(n int) (item T, ok bool) {
	item, _, ok = (*Map[T, struct{}])(t).Nth(n)
	return item, ok
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
	aug.Iterator[K, V, int]
}

// Rank returns the number of entries before the iterator's position. If the
// iterator is past the end that is the length of the collection; if it is
// before the beginning it is 0.
func (it *Iterator[K, V]) Rank() int {
	return it.Prefix()
}

// SeekNth seeks the iterator to the entry with rank nth, i.e. with exactly
// nth entries before it. If nth is out of range the iterator is left
// invalid.
func (it *Iterator[K, V]) SeekNth(nth int) {
	seekNth(aug.LowLevel(&it.Iterator), nth)
}

// seekNth descends by subtree counts. It is what SeekWhere does with a
// rank predicate, without the calls per child.
func seekNth[K, V any](ll *aug.LowLevelIterator[K, V, int], nth int) {
	it := (*aug.Iterator[K, V, int])(ll)
	it.Reset()
	n := ll.Node()
	if n == nil || nth < 0 || nth >= *n.GetA() {
		if n != nil && nth >= 0 {
			ll.SetPos(n.Count())
		}
		return
	}
	for {
		n = ll.Node()
		if n.IsLeaf() {
			ll.SetPos(int16(nth))
			return
		}
		pos := int16(0)
		for ; ; pos++ {
			c := *n.GetChild(pos)
			if nth < c {
				break
			}
			nth -= c
			if nth == 0 {
				ll.SetPos(pos)
				return
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
	aug.Cursor[K, V, int]
}

// Rank returns the number of entries before the cursor's position.
func (c *Cursor[K, V]) Rank() int {
	return c.Prefix()
}

// SeekNth seeks the cursor to the entry with rank nth. If nth is out of
// range the cursor is left invalid.
func (c *Cursor[K, V]) SeekNth(nth int) {
	seekNth(aug.LowLevel(&c.Iterator), nth)
}
