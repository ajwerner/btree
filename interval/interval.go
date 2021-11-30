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

import "github.com/ajwerner/btree/internal/abstract"

// Interval represents an interval with bounds from [Key(), End()) where
// Key() is inclusive and End() is exclusive. If Key() == End(), then the
// Inteval represents a point that only includes that value. Intervals with
// Key() which is larger than End() are invalid and may result in panics
// upon insertion.
type Interval[K any] interface {
	Key() K
	End() K
}

type config[I, K any] struct {
	getKey, getEndKey func(I) K
	compareK          func(K, K) int
}

func (m *Map[I, K, V]) Clone() Map[I, K, V] {
	return Map[I, K, V]{Map: m.Map.Clone()}
}

// Map is a ordered map from I to V where I is an interval. Its iterator
// provides efficient overlap queries.
type Map[I, K, V any] struct {
	abstract.Map[I, V, config[I, K], aug[I, K], *aug[I, K]]
}

// NewMap constructs a new map with the provided comparison functions
// for intervals and for their bounds.
func NewMap[I, K, V any](cmpK Cmp[K], cmpI Cmp[I], key, endKey func(I) K) Map[I, K, V] {
	return Map[I, K, V]{
		Map: abstract.MakeMap[I, V, config[I, K], aug[I, K]](
			config[I, K]{
				compareK:  cmpK,
				getKey:    key,
				getEndKey: endKey,
			},
			cmpI,
		),
	}
}

// Cmp is a comparison function for type T.
type Cmp[T any] func(T, T) int

// Iterator constructs a new Iterator for the Map.
func (t *Map[I, K, V]) Iterator() Iterator[I, K, V] {
	return Iterator[I, K, V]{
		Iterator: t.Map.Iterator(),
	}
}
