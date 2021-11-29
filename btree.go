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

type Map[K, V any] struct {
	t abstract.Map[K, V, struct{}, aug[K], *aug[K]]
}

func NewMap[K, V any](cmp func(K, K) int) *Map[K, V] {
	return &Map[K, V]{
		t: abstract.MakeMap[K, V, struct{}, aug[K]](struct{}{}, cmp),
	}
}

func (t *Map[K, V]) Upsert(k K, v V) (replacedV V, replaced bool) {
	_, replacedV, replaced = t.t.Upsert(k, v)
	return replacedV, replaced
}

func (t *Map[K, V]) Delete(k K) (removedV V, removed bool) {
	_, removedV, removed = t.t.Delete(k)
	return removedV, removed
}

func (t *Map[K, V]) Iterator() Iterator[K, V] {
	return Iterator[K, V]{Iterator: t.t.MakeIter()}
}

type Set[K any] struct {
	t abstract.Map[K, struct{}, struct{}, aug[K], *aug[K]]
}

func NewSet[K any](cmp func(K, K) int) *Set[K] {
	return &Set[K]{
		t: abstract.MakeMap[K, struct{}, struct{}, aug[K]](struct{}{}, cmp),
	}
}

func (t *Set[K]) Upsert(k K) (replacedK K, replaced bool) {
	replacedK, _, replaced = t.t.Upsert(k, struct{}{})
	return replacedK, replaced
}

func (t *Set[K]) Delete(k K) (removedK K, removed bool) {
	removedK, _, removed = t.t.Delete(k)
	return removedK, removed
}

func (t *Set[K]) Iterator() Iterator[K, struct{}] {
	return Iterator[K, struct{}]{Iterator: t.t.MakeIter()}
}

type Iterator[K, V any] struct {
	abstract.Iterator[K, V, struct{}, aug[K], *aug[K]]
}

type aug[K any] struct{}

func (a *aug[T]) Update(n abstract.Node[T, *aug[T]], md abstract.UpdateMeta[T, struct{}, aug[T]]) bool {
	return false
}
