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

// Package interval provides copy-on-write ordered maps and sets of
// intervals whose iterators find every interval overlapping a query in
// O(log n + k).
package interval

import (
	"iter"

	"github.com/ajwerner/btree/aug"
)

// Map is an ordered map from I to V where I is an interval. Its iterator
// provides efficient overlap queries.
type Map[I, K, V any] struct {
	*aug.Map[I, V, subtreeBound[K]]
}

// Bounds describes how to read intervals of type I whose bounds have type
// K. Compare, Key and End are required.
type Bounds[I, K any] struct {

	// Compare orders bound keys.
	Compare func(K, K) int

	// Key returns the inclusive start of an interval.
	Key func(I) K

	// End returns the exclusive end of an interval. An interval without an
	// end (see HasEnd) is a point containing only its Key.
	End func(I) K

	// HasEnd reports whether an interval has an end. It is optional: by
	// default an interval whose End is the zero K has none.
	HasEnd func(I) bool

	// CompareIntervals orders intervals and so defines which intervals a
	// Map treats as the same key. It is optional: by default intervals are
	// ordered by Key, then points before ranges, then by End.
	CompareIntervals func(I, I) int
}

// Interval is implemented by interval types that expose their own bounds:
// Key is the inclusive start and End the exclusive end.
type Interval[K any] interface {
	Key() K
	End() K
}

// BoundsOf returns Bounds for an interval type that exposes its own bounds,
// with the default HasEnd and CompareIntervals. The interval type is given
// explicitly and K is inferred: BoundsOf[span](cmp.Compare[int]).
func BoundsOf[I Interval[K], K any](compare func(K, K) int) Bounds[I, K] {
	return Bounds[I, K]{
		Compare: compare,
		Key:     I.Key,
		End:     I.End,
	}
}

func (b Bounds[I, K]) withDefaults() Bounds[I, K] {
	if b.Compare == nil || b.Key == nil || b.End == nil {
		panic("interval: Bounds.Compare, Key and End are required")
	}
	if b.HasEnd == nil {
		b.HasEnd = func(i I) bool {
			var zero K
			return b.Compare(b.End(i), zero) != 0
		}
	}
	if b.CompareIntervals == nil {
		b.CompareIntervals = func(x, y I) int {
			if c := b.Compare(b.Key(x), b.Key(y)); c != 0 {
				return c
			}
			xEnd, yEnd := b.HasEnd(x), b.HasEnd(y)
			switch {
			case xEnd && yEnd:
				return b.Compare(b.End(x), b.End(y))
			case xEnd:
				return 1
			case yEnd:
				return -1
			default:
				return 0
			}
		}
	}
	return b
}

// New constructs a Map over intervals described by b. See aug.WithDegree
// and aug.WithFreeList for the options.
func New[I, K, V any](b Bounds[I, K], opts ...aug.Option) *Map[I, K, V] {
	b = b.withDefaults()
	return &Map[I, K, V]{
		Map: aug.New[I, V, subtreeBound[K]](
			b.CompareIntervals,
			&updater[I, K, V]{
				cmp:    b.Compare,
				key:    b.Key,
				end:    b.End,
				hasEnd: b.HasEnd,
			},
			opts...,
		),
	}
}

// Clone clones the Map, lazily. It does so in constant time.
func (m *Map[I, K, V]) Clone() *Map[I, K, V] {
	return &Map[I, K, V]{Map: m.Map.Clone()}
}

// Iterator constructs a new Iterator for the Map.
func (t *Map[I, K, V]) Iterator() Iterator[I, K, V] {
	return Iterator[I, K, V]{
		Iterator: t.Map.Iterator(),
	}
}

// Overlapping returns an iterator over the entries whose intervals overlap
// bounds, in order of their start keys.
func (t *Map[I, K, V]) Overlapping(bounds I) iter.Seq2[I, V] {
	return func(yield func(I, V) bool) {
		it := t.Iterator()
		for it.FirstOverlap(bounds); it.Valid(); it.NextOverlap() {
			if !yield(it.Key(), it.Value()) {
				return
			}
		}
	}
}

// Set is an ordered set of intervals of type I with bounds of type K whose
// iterator provides efficient overlap queries.
type Set[I, K any] Map[I, K, struct{}]

// NewSet constructs a Set over intervals described by b. See New.
func NewSet[I, K any](b Bounds[I, K], opts ...aug.Option) *Set[I, K] {
	return (*Set[I, K])(New[I, K, struct{}](b, opts...))
}

// Clone clones the Set, lazily. It does so in constant time.
func (t *Set[I, K]) Clone() *Set[I, K] {
	return (*Set[I, K])((*Map[I, K, struct{}])(t).Clone())
}

// Upsert inserts or updates the provided item. It returns
// the overwritten item if a previous value existed for the key.
func (t *Set[I, K]) Upsert(item I) (replaced I, overwrote bool) {
	replaced, _, overwrote = t.Map.Upsert(item, struct{}{})
	return replaced, overwrote
}

// Delete removes the provided item. It returns true if the item existed in
// the set.
func (t *Set[I, K]) Delete(item I) (removed bool) {
	_, _, removed = t.Map.Delete(item)
	return removed
}

// Contains returns true if the item exists in the set.
func (t *Set[I, K]) Contains(item I) bool {
	_, ok := t.Map.Get(item)
	return ok
}

// Iterator constructs an iterator for this set.
func (t *Set[I, K]) Iterator() Iterator[I, K, struct{}] {
	return (*Map[I, K, struct{}])(t).Iterator()
}

// Overlapping returns an iterator over the items that overlap bounds, in
// order of their start keys.
func (t *Set[I, K]) Overlapping(bounds I) iter.Seq[I] {
	return func(yield func(I) bool) {
		for i := range (*Map[I, K, struct{}])(t).Overlapping(bounds) {
			if !yield(i) {
				return
			}
		}
	}
}
