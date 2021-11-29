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

	"github.com/ajwerner/btree/new/abstract"
)

type Map[K, V any] struct {
	t abstract.Map[K, V, struct{}, aug[K], *aug[K]]
}

func NewMap[K, V any](cmp func(K, K) int) *Map[K, V] {
	return &Map[K, V]{
		t: abstract.MakeMap[K, V, struct{}, aug[K]](struct{}{}, cmp),
	}
}

type Set[K any] struct {
	t abstract.Map[K, struct{}, struct{}, aug[K], *aug[K]]
}

func NewSet[K any](cmp func(K, K) int) *Set[K] {
	return &Set[K]{
		t: abstract.MakeMap[K, struct{}, struct{}, aug[K]](struct{}{}, cmp),
	}
}

func (t *Set[K]) Upsert(k K) (replaced K, overwrote bool) {
	replaced, _, overwrote = t.t.Upsert(k, struct{}{})
	return replaced, overwrote
}

func (t *Set[K]) Delete(k K) (removed bool) {
	_, _, removed = t.t.Delete(k)
	return removed
}

func (t *Map[K, V]) Upsert(k K, v V) (replacedV V, overwrote bool) {
	_, replacedV, overwrote = t.t.Upsert(k, v)
	return replacedV, overwrote
}

func (t *Map[K, V]) Delete(k K) (removedVal V, removed bool) {
	_, removedVal, removed = t.t.Delete(k)
	return removedVal, removed
}

type Iterator[K, V any] struct {
	abstract.Iterator[K, V, struct{}, aug[K], *aug[K]]
}

func (t *Map[K, V]) Iterator() Iterator[K, V] {
	return Iterator[K, V]{Iterator: t.t.MakeIter()}
}

func (t *Set[K]) Iterator() Iterator[K, struct{}] {
	return Iterator[K, struct{}]{Iterator: t.t.MakeIter()}
}

func lowLevel[K, V any](
	it *Iterator[K, V],
) *abstract.LowLevelIterator[K, V, struct{}, aug[K], *aug[K]] {
	return abstract.LowLevel(&it.Iterator)
}

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
			before += ll.GetChild(i).children
		}
		before += int(ll.Pos())
		ll.Descend()
		ll.SetPos(pos)
	}
	if !ll.IsLeaf() {
		for i, pos := int16(0), ll.Pos(); i <= pos; i++ {
			before += ll.GetChild(i).children
		}
	}
	before += int(ll.Pos())
	return before
}

func (it *Iterator[K, V]) Nth(nth int) {
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

var onErrorf = func(format string, args ...interface{}) {
	panic(fmt.Errorf(format, args...))
}
