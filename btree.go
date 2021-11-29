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

package btree

import "github.com/ajwerner/btree/internal/abstract"

// Map is a ordered map from K to V.
type Map[K, V any] struct {
	abstract.Map[K, V, struct{}, aug[K], *aug[K]]
}

// NewMap constructs a new Map with the provided comparison function.
func NewMap[K, V any](cmp func(K, K) int) *Map[K, V] {
	return &Map[K, V]{
		Map: abstract.MakeMap[K, V, struct{}, aug[K]](struct{}{}, cmp),
	}
}

// Set is an ordered set of items of type T.
type Set[T any] Map[T, struct{}]

// NewSet constructs a new Set with the provided comparison function.
func NewSet[T any](cmp func(T, T) int) *Set[T] {
	return (*Set[T])(NewMap[T, struct{}](cmp))
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

type aug[K any] struct{}

func (a *aug[T]) Update(
	*abstract.Config[T, struct{}],
	abstract.Node[T, *aug[T]],
	abstract.UpdateMeta[T, aug[T]],
) bool {
	return false
}
