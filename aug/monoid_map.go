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

import (
	"cmp"
	"iter"
)

// MonoidMap is a Map whose augmentation is a CommutativeMonoid, which
// makes aggregate queries available: Total, Prefix and Aggregate on the
// map, Prefix and SeekPrefix on its iterators and cursors. It has the same
// representation as Map; the conversion between the two is free.
type MonoidMap[K, V, A any] Map[K, V, A]

// NewMonoid constructs a MonoidMap maintaining the augmentation described
// by m (see MonoidUpdater). See WithDegree and WithFreeList for the
// options.
func NewMonoid[K, V, A any](cmp func(K, K) int, m CommutativeMonoid[K, V, A], opts ...Option) *MonoidMap[K, V, A] {
	return (*MonoidMap[K, V, A])(New(cmp, MonoidUpdater(m), opts...))
}

// NewOrderedMonoid is NewMonoid for keys that support <; see NewOrdered.
func NewOrderedMonoid[K cmp.Ordered, V, A any](m CommutativeMonoid[K, V, A], opts ...Option) *MonoidMap[K, V, A] {
	return (*MonoidMap[K, V, A])(NewOrdered(MonoidUpdater(m), opts...))
}

func (t *MonoidMap[K, V, A]) a() *Map[K, V, A] { return (*Map[K, V, A])(t) }

// Total returns the aggregate over every entry in the map.
func (t *MonoidMap[K, V, A]) Total() A { return t.a().total() }

// Prefix returns the aggregate over every entry whose key is less than k,
// and whether an entry with key k exists. It runs one descent.
func (t *MonoidMap[K, V, A]) Prefix(k K) (prefix A, found bool) { return t.a().prefix(k) }

// Aggregate returns the aggregate over every entry whose key is in
// [lo, hi). It runs in O(degree * height).
func (t *MonoidMap[K, V, A]) Aggregate(lo, hi K) A { return t.a().aggregateRange(lo, hi) }

// Clear removes all entries; see Map.Clear.
func (t *MonoidMap[K, V, A]) Clear() { t.a().Clear() }

// Clone clones the map, lazily. It does so in constant time.
func (t *MonoidMap[K, V, A]) Clone() *MonoidMap[K, V, A] {
	return (*MonoidMap[K, V, A])(t.a().Clone())
}

// Delete removes the entry with key k, returning it.
func (t *MonoidMap[K, V, A]) Delete(k K) (removedK K, v V, found bool) { return t.a().Delete(k) }

// Upsert inserts or replaces the entry with key k, returning any replaced
// entry.
func (t *MonoidMap[K, V, A]) Upsert(k K, v V) (replacedK K, replacedV V, replaced bool) {
	return t.a().Upsert(k, v)
}

// Get returns the value for k, if any.
func (t *MonoidMap[K, V, A]) Get(k K) (v V, ok bool) { return t.a().Get(k) }

// Len returns the number of entries.
func (t *MonoidMap[K, V, A]) Len() int { return t.a().Len() }

// Height returns the height of the tree.
func (t *MonoidMap[K, V, A]) Height() int { return t.a().Height() }

// Degree returns the degree of the tree.
func (t *MonoidMap[K, V, A]) Degree() int { return t.a().Degree() }

// Compare compares two keys with the map's comparison function.
func (t *MonoidMap[K, V, A]) Compare(a, b K) int { return t.a().Compare(a, b) }

// String renders the tree in a Newick-like format.
func (t *MonoidMap[K, V, A]) String() string { return t.a().String() }

// Verify checks the tree's invariants; see Map.Verify.
func (t *MonoidMap[K, V, A]) Verify() error { return t.a().Verify() }

// Iterator returns a new MonoidIterator positioned before the first entry.
func (t *MonoidMap[K, V, A]) Iterator() MonoidIterator[K, V, A] {
	return MonoidIterator[K, V, A]{Iterator: t.a().Iterator()}
}

// Cursor returns a new MonoidCursor positioned before the first entry.
func (t *MonoidMap[K, V, A]) Cursor() MonoidCursor[K, V, A] {
	return MonoidCursor[K, V, A]{Cursor: t.a().Cursor()}
}

// All returns an iterator over every entry in key order.
func (t *MonoidMap[K, V, A]) All() iter.Seq2[K, V] { return t.a().All() }

// Backward returns an iterator over every entry in reverse key order.
func (t *MonoidMap[K, V, A]) Backward() iter.Seq2[K, V] { return t.a().Backward() }

// Range returns an iterator over the entries with keys in [lo, hi).
func (t *MonoidMap[K, V, A]) Range(lo, hi K) iter.Seq2[K, V] { return t.a().Range(lo, hi) }

// From returns an iterator over the entries with keys >= lo.
func (t *MonoidMap[K, V, A]) From(lo K) iter.Seq2[K, V] { return t.a().From(lo) }

// Min returns the entry with the smallest key.
func (t *MonoidMap[K, V, A]) Min() (k K, v V, ok bool) { return t.a().Min() }

// Max returns the entry with the largest key.
func (t *MonoidMap[K, V, A]) Max() (k K, v V, ok bool) { return t.a().Max() }

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

// SeekPrefix positions the iterator at the first entry, in key order,
// whose inclusive prefix satisfies pred, where an entry's inclusive prefix
// is the aggregate over it and every entry before it. It returns the
// aggregate over the entries before that one and whether one was found;
// when none was, the iterator is past the end and the total is returned.
//
// pred must change from false to true at most once as the prefix grows:
// "the running count exceeds n" or "the running sum reaches s" qualify, so
// SeekPrefix implements selection by rank (see orderstat) and by
// cumulative weight in one descent, calling pred and Combine once per
// child or entry visited. Other predicates are not supported.
func (i *MonoidIterator[K, V, A]) SeekPrefix(pred func(inclusivePrefix A) bool) (prefix A, ok bool) {
	return i.Iterator.seekPrefix(pred)
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

// SeekPrefix positions the cursor as MonoidIterator.SeekPrefix does.
func (c *MonoidCursor[K, V, A]) SeekPrefix(pred func(inclusivePrefix A) bool) (prefix A, ok bool) {
	return c.Cursor.Iterator.seekPrefix(pred)
}
