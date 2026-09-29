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

// Package orderstat provides copy-on-write ordered maps and sets whose
// iterators additionally support rank queries and seeking by rank.
package orderstat

import (
	"fmt"

	"github.com/ajwerner/btree/aug"
)

// Map is an ordered map from K to V which additionally offers the methods
// of an order-statistic tree on its iterator.
type Map[K, V any] struct {
	*aug.Map[K, V, stat]
}

// New constructs a Map with the provided comparison function. See
// aug.WithDegree and aug.WithFreeList for the options.
func New[K, V any](cmp func(K, K) int, opts ...aug.Option) *Map[K, V] {
	return &Map[K, V]{Map: aug.New[K, V, stat](cmp, updater[K, V]{}, opts...)}
}

// Iterator constructs a new Iterator for this Map.
func (t *Map[K, V]) Iterator() Iterator[K, V] {
	return Iterator[K, V]{Iterator: t.Map.Iterator()}
}

// Clone clones the Map, lazily. It does so in constant time.
func (t *Map[K, V]) Clone() *Map[K, V] {
	return &Map[K, V]{Map: t.Map.Clone()}
}

// Set is an ordered set with items of type T which additionally offers the
// methods of an order-statistic tree on its iterator.
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

// FreeList recycles nodes between orderstat Maps and Sets with the same key
// and value types.
type FreeList[K, V any] = aug.FreeList[K, V, stat]

// NewFreeList returns a FreeList that retains up to size nodes. See
// aug.NewFreeList.
func NewFreeList[K, V any](size int) FreeList[K, V] {
	return aug.NewFreeList[K, V, stat](size)
}

// stat is the augmentation: the number of items in the subtree.
type stat struct {
	count int
}

type updater[K, V any] struct{}

func (u updater[K, V]) Update(
	n *aug.Node[K, V, stat],
	md aug.UpdateInfo[K, stat],
) (updated bool) {
	a := n.GetA()
	switch md.Action {
	case aug.Removal, aug.Split:
		a.count--
		if md.ModifiedOther != nil {
			a.count -= md.ModifiedOther.count
		}
		return true
	case aug.Insertion:
		a.count++
		if md.ModifiedOther != nil {
			a.count += md.ModifiedOther.count
		}
		return true
	case aug.Default:
		orig := a.count
		var count int
		if !n.IsLeaf() {
			for i := int16(0); i <= n.Count(); i++ {
				count += n.GetChild(i).count
			}
		}
		count += int(n.Count())
		a.count = count
		return a.count != orig
	default:
		panic(fmt.Errorf("unknown action %v", md.Action))
	}
}

// Iterator allows iteration through the collection. It offers all the usual
// iterator methods, plus it offers Rank() and SeekNth() which allow efficient
// rank operations.
type Iterator[K, V any] struct {
	aug.Iterator[K, V, stat]
}

// Rank returns the rank of the current iterator position, i.e. the number
// of items in the collection which are less than the current item. If the
// iterator is not valid, -1 is returned.
func (it *Iterator[K, V]) Rank() int {
	if !it.Valid() {
		return -1
	}
	ll := lowLevel(it)
	// Every ancestor frame contributes the keys and the subtrees to the
	// left of the child through which we descended.
	var before int
	for d, depth := 0, ll.Depth(); d < depth; d++ {
		n, pos := ll.Frame(d)
		for i := int16(0); i < pos; i++ {
			before += n.GetChild(i).count
		}
		before += int(pos)
	}
	// The current node contributes the keys to the left of the position and,
	// if it is not a leaf, the subtrees up to and including the one at the
	// position (which holds keys less than the key at the position).
	n, pos := ll.Node(), ll.Pos()
	if !n.IsLeaf() {
		for i := int16(0); i <= pos; i++ {
			before += n.GetChild(i).count
		}
	}
	before += int(pos)
	return before
}

// SeekNth seeks the iterator to the nth item in the collection (0-indexed).
// If nth is out of range the iterator is left invalid.
func (it *Iterator[K, V]) SeekNth(nth int) {
	it.Reset()
	ll := lowLevel(it)
	if ll.Node() == nil || nth < 0 || nth >= ll.Node().GetA().count {
		return
	}
	// Reset leaves the iterator at position -1 in the root. IncrementPos
	// moves it to the first child and item.
	ll.IncrementPos()
	n := 0
	for n <= nth {
		if ll.IsLeaf() {
			// If we're in the leaf, then, by construction, we can find
			// the relevant position and seek to it in constant time.
			ll.SetPos(int16(nth - n))
			return
		}
		a := ll.Child()
		if n+a.count > nth {
			ll.Descend()
			continue
		}
		n += a.count
		switch {
		case n < nth:
			// Consume the current value, move on to the next one.
			n++
			ll.IncrementPos()
		case n == nth:
			return // found it
		default:
			panic("orderstat: invariant violated")
		}
	}
}

func lowLevel[K, V any](
	it *Iterator[K, V],
) *aug.LowLevelIterator[K, V, stat] {
	return aug.LowLevel(&it.Iterator)
}
