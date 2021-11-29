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

type config[K any] struct {
	compareK func(K, K) int
}

// Map is a ordered map from I to V where I is an interval. Its iterator
// provides efficient overlap queries.
type Map[I Interval[K], V, K any] struct {
	abstract.Map[I, V, config[K], aug[K, I], *aug[K, I]]
}

// NewMap constructs a new map with the provided comparison functions
// for intervals and for their bounds.
func NewMap[I Interval[K], V, K any](cmpK Cmp[K], cmpI Cmp[I]) Map[I, V, K] {
	return Map[I, V, K]{
		Map: abstract.MakeMap[I, V, config[K], aug[K, I]](
			config[K]{compareK: cmpK}, cmpI),
	}
}

// Cmp is a comparison function for type T.
type Cmp[T any] func(T, T) int

// Iterator constructs a new Iterator for the Map.
func (t *Map[I, V, K]) Iterator() Iterator[K, V, I] {
	return Iterator[K, V, I]{
		Iterator: t.Map.Iterator(),
	}
}
