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

// Package btree provides copy-on-write ordered maps and sets backed by a
// B-tree. See package aug for the ownership and concurrency rules, and the
// orderstat and interval packages for augmented variants.
package btree

import "github.com/ajwerner/btree/aug"

// Map is an ordered map from K to V.
type Map[K, V any] struct {
	*aug.Map[K, V, struct{}]
}

// New constructs a Map with the provided comparison function. See
// WithDegree and WithFreeList for the options.
func New[K, V any](cmp func(K, K) int, opts ...Option) *Map[K, V] {
	return &Map[K, V]{Map: aug.New[K, V, struct{}](cmp, nil, opts...)}
}

// Clone clones the Map, lazily. It does so in constant time.
func (m *Map[K, V]) Clone() *Map[K, V] {
	return &Map[K, V]{Map: m.Map.Clone()}
}

// Set is an ordered set of items of type T.
type Set[T any] Map[T, struct{}]

// NewSet constructs a Set with the provided comparison function. See
// WithDegree and WithFreeList for the options.
func NewSet[T any](cmp func(T, T) int, opts ...Option) *Set[T] {
	return (*Set[T])(New[T, struct{}](cmp, opts...))
}

// Clone clones the Set, lazily. It does so in constant time.
func (s *Set[T]) Clone() *Set[T] {
	return (*Set[T])((*Map[T, struct{}])(s).Clone())
}

// Upsert inserts or updates the provided item. It returns
// the overwritten item if a previous value existed for the key.
func (s *Set[T]) Upsert(item T) (replaced T, overwrote bool) {
	replaced, _, overwrote = s.Map.Upsert(item, struct{}{})
	return replaced, overwrote
}

// Delete removes the provided item. It returns true if the item existed in
// the set.
func (s *Set[T]) Delete(item T) (removed bool) {
	_, _, removed = s.Map.Delete(item)
	return removed
}

// Contains returns true if the item exists in the set.
func (s *Set[T]) Contains(item T) bool {
	_, ok := s.Map.Get(item)
	return ok
}

// Option configures a Map or Set at construction.
type Option = aug.Option

// WithDegree sets the degree of the tree. See aug.WithDegree.
func WithDegree(degree int) Option {
	return aug.WithDegree(degree)
}

// FreeList recycles nodes between Maps and Sets with the same key and value
// types.
type FreeList[K, V any] = aug.FreeList[K, V, struct{}]

// NewFreeList returns a FreeList that retains up to size nodes. See
// aug.NewFreeList.
func NewFreeList[K, V any](size int) FreeList[K, V] {
	return aug.NewFreeList[K, V, struct{}](size)
}

// WithFreeList sets the free list a Map or Set allocates from. See
// aug.WithFreeList.
func WithFreeList[K, V any](fl FreeList[K, V]) Option {
	return aug.WithFreeList(fl)
}

// MapIterator is an iterator for a Map.
type MapIterator[K, V any] = aug.Iterator[K, V, struct{}]

// SetIterator is an iterator for a Set.
type SetIterator[T any] = MapIterator[T, struct{}]

// MapCursor is a cursor for a Map: an iterator that can mutate the entry it
// is positioned on. See aug.Cursor.
type MapCursor[K, V any] = aug.Cursor[K, V, struct{}]

// SetCursor is a cursor for a Set. See aug.Cursor.
type SetCursor[T any] = MapCursor[T, struct{}]
