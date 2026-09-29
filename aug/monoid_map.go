// Copyright 2026 Andrew Werner.
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

package aug

import "cmp"

// MonoidMap is a Map whose augmentation is a Monoid, which makes aggregate
// queries available: Total, Prefix and Aggregate on the map, Prefix and
// SeekWhere on its iterators and cursors. It embeds *Map and so offers
// everything a Map does.
type MonoidMap[K, V, A any] struct {
	*Map[K, V, A]
}

// NewMonoid constructs a MonoidMap maintaining the augmentation described
// by m (see MonoidUpdater). See WithDegree and WithFreeList for the
// options.
func NewMonoid[K, V, A any](cmp func(K, K) int, m Monoid[K, V, A], opts ...Option) *MonoidMap[K, V, A] {
	return &MonoidMap[K, V, A]{Map: New(cmp, MonoidUpdater(m), opts...)}
}

// NewOrderedMonoid is NewMonoid for keys that support <; see NewOrdered.
func NewOrderedMonoid[K cmp.Ordered, V, A any](m Monoid[K, V, A], opts ...Option) *MonoidMap[K, V, A] {
	return &MonoidMap[K, V, A]{Map: NewOrdered(MonoidUpdater(m), opts...)}
}

// Clone clones the MonoidMap, lazily. It does so in constant time.
func (t *MonoidMap[K, V, A]) Clone() *MonoidMap[K, V, A] {
	return &MonoidMap[K, V, A]{Map: t.Map.Clone()}
}

// Total returns the aggregate over every entry in the map.
func (t *MonoidMap[K, V, A]) Total() A {
	return t.Map.total()
}

// Prefix returns the aggregate over every entry whose key is less than k,
// and whether an entry with key k exists. It runs one descent.
func (t *MonoidMap[K, V, A]) Prefix(k K) (prefix A, found bool) {
	return t.Map.prefix(k)
}

// Aggregate returns the aggregate over every entry whose key is in
// [lo, hi). It runs in O(degree * height).
func (t *MonoidMap[K, V, A]) Aggregate(lo, hi K) A {
	return t.Map.aggregateRange(lo, hi)
}

// Iterator returns a new MonoidIterator.
func (t *MonoidMap[K, V, A]) Iterator() MonoidIterator[K, V, A] {
	return MonoidIterator[K, V, A]{Iterator: t.Map.Iterator()}
}

// Cursor returns a new MonoidCursor positioned before the first entry.
func (t *MonoidMap[K, V, A]) Cursor() MonoidCursor[K, V, A] {
	return MonoidCursor[K, V, A]{Cursor: t.Map.Cursor()}
}

// MonoidIterator is an Iterator over a MonoidMap.
type MonoidIterator[K, V, A any] struct {
	Iterator[K, V, A]
}

// Prefix returns the aggregate over every entry before the iterator's
// position. If the iterator is past the end it returns the total; if it is
// before the beginning (as after Reset) it returns the zero A. It runs in
// O(degree * height).
func (i *MonoidIterator[K, V, A]) Prefix() A {
	return i.Iterator.prefix()
}

// SeekWhere positions the iterator at the first entry, in key order, for
// which pred(prefix, contribution) is true, where prefix is the aggregate
// over every entry before it and contribution is Of that entry. It returns
// that prefix. If no entry qualifies the iterator is left past the end and
// the total is returned.
//
// pred must be monotone: if pred(p, x) is false for the aggregate x of a
// span, it must be false for (p', x') of every entry in the span, where p'
// is the prefix of that entry. Predicates of the form "prefix combined with
// x reaches a threshold" have this property, so SeekWhere implements
// selection by rank (see orderstat) and by cumulative sum. It runs one
// descent, calling pred and Combine once per child or entry visited.
func (i *MonoidIterator[K, V, A]) SeekWhere(pred func(prefix, contribution A) bool) A {
	return i.Iterator.seekWhere(pred)
}

// MonoidCursor is a Cursor over a MonoidMap.
type MonoidCursor[K, V, A any] struct {
	Cursor[K, V, A]
}

// Prefix returns the aggregate over every entry before the cursor's
// position. See MonoidIterator.Prefix.
func (c *MonoidCursor[K, V, A]) Prefix() A {
	return c.Cursor.Iterator.prefix()
}

// SeekWhere positions the cursor as MonoidIterator.SeekWhere does.
func (c *MonoidCursor[K, V, A]) SeekWhere(pred func(prefix, contribution A) bool) A {
	return c.Cursor.Iterator.seekWhere(pred)
}
