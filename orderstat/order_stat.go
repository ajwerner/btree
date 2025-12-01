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

package orderstat

import (
	"fmt"

	"github.com/anacrolix/btree/internal/abstract"
)

// Map is a ordered map from K to V which additionally offers the methods
// of a order-statistic tree on its iterator.
type Map[K, V any] struct {
	abstract.Map[K, V, aug]
}

// MakeMap constructs a new Map with the provided comparison function.
func MakeMap[K, V any](cmp func(K, K) int) Map[K, V] {
	return Map[K, V]{
		Map: abstract.MakeMap[K, V, aug](cmp, &updater[K, V]{}),
	}
}

// Iterator constructs a new Iterator for this Map.
func (t *Map[K, V]) Iterator() Iterator[K, V] {
	return Iterator[K, V]{Iterator: t.Map.Iterator()}
}

// Clone clones the Map, lazily. It does so in constant time.
func (t *Map[K, V]) Clone() Map[K, V] {
	return Map[K, V]{Map: t.Map.Clone()}
}

// Set is an ordered set with items of type T which additionally offers the
// methods of an order-statistic tree on its iterator.
type Set[T any] Map[T, struct{}]

// MakeSet constructs a new Set with the provided comparison function.
func MakeSet[T any](cmp func(T, T) int) Set[T] {
	return (Set[T])(MakeMap[T, struct{}](cmp))
}

// Clone clones the Set, lazily. It does so in constant time.
func (t *Set[T]) Clone() Set[T] {
	return (Set[T])((*Map[T, struct{}])(t).Clone())
}

// Upsert inserts or updates the provided item. It returns
// the overwritten item if a previous value existed for the key.
func (t *Set[T]) Upsert(item T) (replaced T, overwrote bool) {
	replaced, _, overwrote = t.Map.Upsert(item, struct{}{})
	return replaced, overwrote
}

// Delete removes the value with the provided key. It returns true if the
// item existed in the set.
func (t *Set[K]) Delete(item K) (removed bool) {
	_, _, removed = t.Map.Delete(item)
	return removed
}

// Iterator constructs an iterator for this set.
func (t *Set[K]) Iterator() Iterator[K, struct{}] {
	return (*Map[K, struct{}])(t).Iterator()
}

type aug struct {
	// children is the number of items rooted at the current subtree.
	children int
}

type updater[K, V any] struct{}

func (u updater[K, V]) Update(
	n *abstract.Node[K, V, aug],
	md abstract.UpdateInfo[K, aug],
) (updated bool) {
	a := n.GetA()
	switch md.Action {
	case abstract.Removal, abstract.Split:
		a.children--
		if md.ModifiedOther != nil {
			a.children -= md.ModifiedOther.children
		}
		return true
	case abstract.Insertion:
		a.children++
		if md.ModifiedOther != nil {
			a.children += md.ModifiedOther.children
		}
		return true
	case abstract.Default:
		orig := a.children
		var children int
		if !n.IsLeaf() {
			N := n.Count()
			for i := int16(0); i <= N; i++ {
				if child := n.GetChild(i); child != nil {
					children += child.children
				}
			}
		}
		children += int(n.Count())
		a.children = children
		return a.children != orig
	default:
		panic(fmt.Errorf("unknown action %v", md.Action))
	}
}

// Iterator allows iteration through the collection. It offers all the usual
// iterator methods, plus it offers Rank() and SeekNth() which allow efficient
// rank operations.
type Iterator[K, V any] struct {
	abstract.Iterator[K, V, aug]
}

// Rank returns the rank of the current iterator position. If the iterator
// is not valid, -1 is returned.
func (it *Iterator[K, V]) Rank() int {
	if !it.Valid() {
		return -1
	}
	ll := lowLevel(it)
	// If this is the root, then we want to figure out how many children are
	// below the current point.

	// Otherwise, we need to go up to the current parent, calculate everything
	// less and then drop back down to the current node and add everything less.
	var before int
	if ll.Depth() > 0 {
		pos := ll.Pos()
		ll.Ascend()
		for i, parentPos := int16(0), ll.Pos(); i < parentPos; i++ {
			before += ll.Node().GetChild(i).children
		}
		before += int(ll.Pos())
		ll.Descend()
		ll.SetPos(pos)
	}
	if !ll.IsLeaf() {
		for i, pos := int16(0), ll.Pos(); i <= pos; i++ {
			before += ll.Node().GetChild(i).children
		}
	}
	before += int(ll.Pos())
	return before
}

// SeekNth seeks the iterator to the nth item in the collection (0-indexed).
func (it *Iterator[K, V]) SeekNth(nth int) {
	it.Reset()
	// Reset has bizarre semantics in that it initializes the iterator to
	// an invalid position (-1) at the root of the tree. IncrementPos moves it
	// to the first child and item of the
	ll := lowLevel(it)
	ll.IncrementPos()
	n := 0
	for n <= nth {
		if ll.IsLeaf() {
			// If we're in the leaf, then, by construction, we can find
			// the relevant position and seek to it in constant time.
			//
			// TODO(ajwerner): Add more invariant checking.
			ll.SetPos(int16(nth - n))
			return
		}
		a := ll.Child()
		if a == nil {
			onErrorf("failed to visit child")
		}
		if n+a.children > nth {
			ll.Descend()
			continue
		}

		n += a.children
		switch {
		case n < nth:
			// Consume the current value, move on to the next one.
			n++
			ll.IncrementPos()
		case n == nth:
			return // found it
		default:
			onErrorf("invariant violated")
		}
	}
}

func lowLevel[K, V any](
	it *Iterator[K, V],
) *abstract.LowLevelIterator[K, V, aug] {
	return abstract.LowLevel(&it.Iterator)
}

var onErrorf = func(format string, args ...interface{}) {
	panic(fmt.Errorf(format, args...))
}
