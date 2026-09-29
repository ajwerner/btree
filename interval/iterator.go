// Copyright 2018 The Cockroach Authors.
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

import (
	"sort"

	"github.com/ajwerner/btree/aug"
)

// Span is a query: the keys a search asks about. It is separate from the
// stored interval type so that asking about a range does not require
// building a stored object.
type Span[K any] struct {
	start, end K
	point      bool
}

// HalfOpen returns the span of keys in [start, end). A span whose end is
// not after its start is empty and overlaps nothing.
func HalfOpen[K any](start, end K) Span[K] {
	return Span[K]{start: start, end: end}
}

// Point returns the span containing only key.
func Point[K any](key K) Span[K] {
	return Span[K]{start: key, point: true}
}

// upperBound is the span's bound, in the augmentation's terms.
func (s Span[K]) upperBound() keyBound[K] {
	if s.point {
		return keyBound[K]{k: s.start, inclusive: true}
	}
	return keyBound[K]{k: s.end}
}

// empty reports whether the span covers no key.
func (s Span[K]) empty(cmp func(K, K) int) bool {
	return !s.point && cmp(s.end, s.start) <= 0
}

// Iterator is an iterator over a Map in key order; see aug.Iterator.
type Iterator[I, K, V any] = aug.Iterator[I, V, subtreeBound[K]]

// OverlapIterator visits the entries whose intervals overlap a Span, in
// order of their start keys. Map.Overlaps returns one positioned at the
// first overlap; Next moves to the following one. It borrows the map like
// an Iterator does.
type OverlapIterator[I, K, V any] struct {
	it aug.Iterator[I, V, subtreeBound[K]]
	o  overlapScan[I, K, V]
}

// An overlap scan is a scan over all intervals that overlap with the provided
// interval in order of the overlapping intervals' start keys. The goal of the scan
// is to minimize the number of key comparisons performed in total. The
// algorithm operates based on the following two invariants maintained by
// augmented interval tree:
//  1. all intervals are sorted in the tree based on their start key.
//  2. all tree nodes maintain the upper bound end key of all intervals
//     in their subtree.
//
// The scan algorithm starts in "unconstrained minimum" and "unconstrained
// maximum" states. To enter a "constrained minimum" state, the scan must reach
// intervals in the tree with start keys above the search range's start key.
// Because intervals in the tree are sorted by start key, once the scan enters the
// "constrained minimum" state it will remain there. To enter a "constrained
// maximum" state, the scan must determine the first child node in a given
// subtree that can have intervals with start keys above the search range's end
// key. The scan then remains in the "constrained maximum" state until it
// traverse into this child node, at which point it moves to the "unconstrained
// maximum" state again.
//
// The scan algorithm works like a standard B-tree forward scan with the
// following augmentations:
//  1. before tranversing the tree, the scan performs a binary search on the
//     root node's items to determine a "soft" lower-bound constraint position
//     and a "hard" upper-bound constraint position in the root's children.
//  2. when tranversing into a child node in the lower or upper bound constraint
//     position, the constraint is refined by searching the child's items.
//  3. the initial traversal down the tree follows the left-most children
//     whose upper bound end keys are equal to or greater than the start key
//     of the search range. The children followed will be equal to or less
//     than the soft lower bound constraint.
//  4. once the initial tranversal completes and the scan is in the left-most
//     node whose upper bound overlaps the search range, key comparisons
//     must be performed with each interval in the tree. This is necessary because
//     any of these intervals may have end keys that cause them to overlap with the
//     search range.
//  5. once the scan reaches the lower bound constraint position (the first interval
//     with a start key equal to or greater than the search range's start key),
//     it can begin scaning without performing key comparisons. This is allowed
//     because all intervals from this point forward will have end keys that are
//     greater than the search range's start key.
//  6. once the scan reaches the upper bound constraint position, it terminates.
//     It does so because the interval at this position is the first interval with a
//     start key larger than the search range's end key.
type overlapScan[I, K, V any] struct {
	span Span[K]

	// The "soft" lower-bound constraint.
	constrMinN       *aug.Node[I, V, subtreeBound[K]]
	constrMinPos     int16
	constrMinReached bool

	// The "hard" upper-bound constraint.
	constrMaxN   *aug.Node[I, V, subtreeBound[K]]
	constrMaxPos int16
}

func newOverlapIterator[I, K, V any](m *aug.Map[I, V, subtreeBound[K]], span Span[K]) OverlapIterator[I, K, V] {
	i := OverlapIterator[I, K, V]{it: m.Iterator(), o: overlapScan[I, K, V]{span: span}}
	ll := aug.LowLevel(&i.it)
	cfg := i.cfg()
	if span.empty(cfg.cmp) {
		return i
	}
	ll.IncrementPos()
	if !i.it.Valid() {
		return i
	}
	i.constrainMinSearchBounds()
	i.constrainMaxSearchBounds()
	i.findNextOverlap()
	return i
}

func (i *OverlapIterator[I, K, V]) cfg() *updater[I, K, V] {
	return aug.LowLevel(&i.it).Config().Updater.(*updater[I, K, V])
}

// Valid reports whether the iterator is at an overlapping entry.
func (i *OverlapIterator[I, K, V]) Valid() bool { return i.it.Valid() }

// Key returns the interval at the iterator's position, which must be
// valid.
func (i *OverlapIterator[I, K, V]) Key() I { return i.it.Key() }

// Value returns the value at the iterator's position, which must be valid.
func (i *OverlapIterator[I, K, V]) Value() V { return i.it.Value() }

// Next positions the iterator at the following overlapping entry and
// reports whether there is one.
func (i *OverlapIterator[I, K, V]) Next() bool {
	if !i.it.Valid() {
		return false
	}
	aug.LowLevel(&i.it).IncrementPos()
	i.findNextOverlap()
	return i.it.Valid()
}

func (i *OverlapIterator[I, K, V]) constrainMinSearchBounds() {
	ll := aug.LowLevel(&i.it)
	cfg := i.cfg()
	cmp := cfg.cmp
	k := i.o.span.start
	n := ll.Node()
	j := sort.Search(int(n.Count()), func(j int) bool {
		return cmp(k, cfg.key(n.Key(int16(j)))) <= 0
	})
	i.o.constrMinN = n
	i.o.constrMinPos = int16(j)
}

func (i *OverlapIterator[I, K, V]) constrainMaxSearchBounds() {
	ll := aug.LowLevel(&i.it)
	cfg := i.cfg()
	cmp := cfg.cmp
	up := i.o.span.upperBound()
	n := ll.Node()
	j := sort.Search(int(n.Count()), func(j int) bool {
		return !up.contains(cmp, cfg.key(n.Key(int16(j))))
	})
	i.o.constrMaxN = n
	i.o.constrMaxPos = int16(j)
}

func (i *OverlapIterator[I, K, V]) findNextOverlap() {
	ll := aug.LowLevel(&i.it)
	cfg := i.cfg()
	cmp := cfg.cmp
	start := i.o.span.start
	for {
		if ll.Pos() > ll.Node().Count() {
			// Iterate up tree.
			ll.Ascend()
		} else if !ll.Node().IsLeaf() {
			// Iterate down tree.
			if i.o.constrMinReached || ll.ChildAug().contains(cmp, start) {
				par := ll.Node()
				pos := ll.Pos()
				ll.Descend()

				// Refine the constraint bounds, if necessary.
				if par == i.o.constrMinN && pos == i.o.constrMinPos {
					i.constrainMinSearchBounds()
				}
				if par == i.o.constrMaxN && pos == i.o.constrMaxPos {
					i.constrainMaxSearchBounds()
				}
				continue
			}
		}

		// Check search bounds.
		if ll.Node() == i.o.constrMaxN && ll.Pos() == i.o.constrMaxPos {
			// Invalid. Past possible overlaps.
			i.it.Reset()
			return
		}
		if ll.Node() == i.o.constrMinN && ll.Pos() == i.o.constrMinPos {
			// The scan reached the soft lower-bound constraint.
			i.o.constrMinReached = true
		}

		// Iterate across node.
		if ll.Pos() < ll.Node().Count() {
			// Check for overlapping interval.
			if i.o.constrMinReached {
				// Fast-path to avoid span comparison. i.o.constrMinReached
				// tells us that all intervals have end keys above our search
				// span's start key.
				return
			}
			if cfg.upperBound(i.it.Key()).contains(cmp, start) {
				return
			}
		}
		ll.IncrementPos()
	}
}
