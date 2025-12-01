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

import "github.com/anacrolix/btree/internal/abstract"

// Map is a ordered map from I to V where I is an interval. Its iterator
// provides efficient overlap queries.
type Map[I, K, V any] struct {
	abstract.Map[I, V, aug[K]]
}

type config[I, K any] struct {
	getKey, getEndKey func(I) K
	cmp               func(K, K) int
}

// MakeMap constructs a new map with the provided comparison functions
// for intervals and for their bounds.
func MakeMap[I, K, V any](
	cmpK Cmp[K],
	cmpI Cmp[I],
	key, endKey func(I) K,
	hasEnd func(I) bool,
) Map[I, K, V] {
	if hasEnd == nil {
		hasEnd = func(i I) bool {
			return !isZero(cmpK, endKey(i))
		}
	}
	return Map[I, K, V]{
		Map: abstract.MakeMap[I, V, aug[K]](
			cmpI,
			&updater[I, K, V]{
				cmp:    cmpK,
				key:    key,
				end:    endKey,
				hasEnd: hasEnd,
			},
		),
	}
}

// Clone clones the Map, lazily. It does so in constant time.
func (m *Map[I, K, V]) Clone() Map[I, K, V] {
	return Map[I, K, V]{Map: m.Map.Clone()}
}

// Cmp is a comparison function for type T.
type Cmp[T any] func(T, T) int

// Iterator constructs a new Iterator for the Map.
func (t *Map[I, K, V]) Iterator() Iterator[I, K, V] {
	return Iterator[I, K, V]{
		Iterator: t.Map.Iterator(),
	}
}

// Set is an ordered set with items of type T which additionally offers the
// methods of an order-statistic tree on its iterator.
type Set[I, T any] Map[I, T, struct{}]

// MakeSet constructs a new Set with the provided comparison function.
func MakeSet[I, T any](
	cmpT Cmp[T],
	cmpI Cmp[I],
	key, endKey func(I) T,
	hasEnd func(I) bool,
) Set[I, T] {
	return (Set[I, T])(MakeMap[I, T, struct{}](cmpT, cmpI, key, endKey, hasEnd))
}

// Clone clones the Set, lazily. It does so in constant time.
func (t *Set[I, T]) Clone() Set[I, T] {
	return (Set[I, T])((*Map[I, T, struct{}])(t).Clone())
}

// Upsert inserts or updates the provided item. It returns
// the overwritten item if a previous value existed for the key.
func (t *Set[I, T]) Upsert(item I) (replaced I, overwrote bool) {
	replaced, _, overwrote = t.Map.Upsert(item, struct{}{})
	return replaced, overwrote
}

// Delete removes the value with the provided key. It returns true if the
// item existed in the set.
func (t *Set[I, T]) Delete(item I) (removed bool) {
	_, _, removed = t.Map.Delete(item)
	return removed
}

// Iterator constructs an iterator for this set.
func (t *Set[I, T]) Iterator() Iterator[I, T, struct{}] {
	return (*Map[I, T, struct{}])(t).Iterator()
}
