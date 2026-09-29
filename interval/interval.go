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

package interval

import "github.com/ajwerner/btree/aug"

// Map is an ordered map from I to V where I is an interval. Its iterator
// provides efficient overlap queries.
type Map[I, K, V any] struct {
	*aug.Map[I, V, subtreeBound[K]]
}

// New constructs a Map with the provided comparison functions for
// intervals and for their bounds. key and endKey extract the bounds of an
// interval; hasEnd reports whether an interval has an end key, and may be
// nil, in which case an interval whose end key is the zero K is treated as
// a point. See aug.WithDegree and aug.WithFreeList for the options.
func New[I, K, V any](
	cmpK Cmp[K],
	cmpI Cmp[I],
	key, endKey func(I) K,
	hasEnd func(I) bool,
	opts ...aug.Option,
) *Map[I, K, V] {
	if hasEnd == nil {
		hasEnd = func(i I) bool {
			return !isZero(cmpK, endKey(i))
		}
	}
	return &Map[I, K, V]{
		Map: aug.New[I, V, subtreeBound[K]](
			cmpI,
			&updater[I, K, V]{
				cmp:    cmpK,
				key:    key,
				end:    endKey,
				hasEnd: hasEnd,
			},
			opts...,
		),
	}
}

// Clone clones the Map, lazily. It does so in constant time.
func (m *Map[I, K, V]) Clone() *Map[I, K, V] {
	return &Map[I, K, V]{Map: m.Map.Clone()}
}

// Cmp is a comparison function for type T.
type Cmp[T any] func(T, T) int

// Iterator constructs a new Iterator for the Map.
func (t *Map[I, K, V]) Iterator() Iterator[I, K, V] {
	return Iterator[I, K, V]{
		Iterator: t.Map.Iterator(),
	}
}

// Set is an ordered set of intervals of type I with bounds of type T whose
// iterator provides efficient overlap queries.
type Set[I, T any] Map[I, T, struct{}]

// NewSet constructs a Set with the provided comparison functions. See New.
func NewSet[I, T any](
	cmpT Cmp[T],
	cmpI Cmp[I],
	key, endKey func(I) T,
	hasEnd func(I) bool,
	opts ...aug.Option,
) *Set[I, T] {
	return (*Set[I, T])(New[I, T, struct{}](cmpT, cmpI, key, endKey, hasEnd, opts...))
}

// Clone clones the Set, lazily. It does so in constant time.
func (t *Set[I, T]) Clone() *Set[I, T] {
	return (*Set[I, T])((*Map[I, T, struct{}])(t).Clone())
}

// Upsert inserts or updates the provided item. It returns
// the overwritten item if a previous value existed for the key.
func (t *Set[I, T]) Upsert(item I) (replaced I, overwrote bool) {
	replaced, _, overwrote = t.Map.Upsert(item, struct{}{})
	return replaced, overwrote
}

// Delete removes the provided item. It returns true if the item existed in
// the set.
func (t *Set[I, T]) Delete(item I) (removed bool) {
	_, _, removed = t.Map.Delete(item)
	return removed
}

// Contains returns true if the item exists in the set.
func (t *Set[I, T]) Contains(item I) bool {
	_, ok := t.Map.Get(item)
	return ok
}

// Iterator constructs an iterator for this set.
func (t *Set[I, T]) Iterator() Iterator[I, T, struct{}] {
	return (*Map[I, T, struct{}])(t).Iterator()
}
