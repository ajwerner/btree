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

type Iterator[I, K, V any] struct {
	aug.Iterator[I, V, subtreeBound[K]]

	o overlapScan[I, K, V]
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
	bounds I
	set    bool

	// The "soft" lower-bound constraint.
	constrMinN       *aug.Node[I, V, subtreeBound[K]]
	constrMinPos     int16
	constrMinReached bool

	// The "hard" upper-bound constraint.
	constrMaxN   *aug.Node[I, V, subtreeBound[K]]
	constrMaxPos int16
}

func (o *overlapScan[I, K, V]) reset() {
	*o = overlapScan[I, K, V]{}
}

func (o *overlapScan[I, K, V]) empty() bool {
	return !o.set
}

// FirstOverlap seeks to the first interval in the tree that overlaps with the
// provided search interval.
func (i *Iterator[I, K, V]) FirstOverlap(bounds I) {
	i.Reset()
	it := lowLevel(i)
	it.IncrementPos()
	if !i.Valid() {
		return
	}
	i.o = overlapScan[I, K, V]{bounds: bounds, set: true}
	i.constrainMinSearchBounds()
	i.constrainMaxSearchBounds()
	i.findNextOverlap()
}

func lowLevel[I, K, V any](
	it *Iterator[I, K, V],
) *aug.LowLevelIterator[I, V, subtreeBound[K]] {
	return aug.LowLevel(&it.Iterator)
}

// Reset marks the iterator as invalid and clears any state, including an
// overlap scan in progress.
func (i *Iterator[I, K, V]) Reset() {
	i.o.reset()
	i.Iterator.Reset()
}

// First seeks to the first interval, ending any overlap scan.
func (i *Iterator[I, K, V]) First() {
	i.o.reset()
	i.Iterator.First()
}

// Last seeks to the last interval, ending any overlap scan.
func (i *Iterator[I, K, V]) Last() {
	i.o.reset()
	i.Iterator.Last()
}

// SeekGE seeks to the first interval greater than or equal to the
// provided one, ending any overlap scan.
func (i *Iterator[I, K, V]) SeekGE(bounds I) bool {
	i.o.reset()
	return i.Iterator.SeekGE(bounds)
}

// SeekGT seeks to the first interval greater than the provided one, ending
// any overlap scan.
func (i *Iterator[I, K, V]) SeekGT(bounds I) bool {
	i.o.reset()
	return i.Iterator.SeekGT(bounds)
}

// SeekLE seeks to the last interval less than or equal to the provided one,
// ending any overlap scan.
func (i *Iterator[I, K, V]) SeekLE(bounds I) bool {
	i.o.reset()
	return i.Iterator.SeekLE(bounds)
}

// SeekLT seeks to the last interval less than the provided one, ending any
// overlap scan.
func (i *Iterator[I, K, V]) SeekLT(bounds I) bool {
	i.o.reset()
	return i.Iterator.SeekLT(bounds)
}

// NextOverlap positions the iterator to the interval immediately following
// its current position that overlaps with the search interval.
func (i *Iterator[I, K, V]) NextOverlap() {
	if !i.Valid() {
		return
	}
	if i.o.empty() {
		// Invalid. Mixed overlap scan with non-overlap scan.
		i.Reset()
		return
	}
	lowLevel(i).IncrementPos()
	i.findNextOverlap()
}

func (i *Iterator[I, K, V]) constrainMinSearchBounds() {
	ll := lowLevel(i)
	cfg := ll.Config().Updater.(*updater[I, K, V])
	cmp := cfg.cmp
	k := cfg.key(i.o.bounds)
	n := ll.Node()
	j := sort.Search(int(n.Count()), func(j int) bool {
		return cmp(k, cfg.key(n.Key(int16(j)))) <= 0
	})
	i.o.constrMinN = n
	i.o.constrMinPos = int16(j)
}

func (i *Iterator[I, K, V]) constrainMaxSearchBounds() {
	ll := lowLevel(i)
	cfg := ll.Config().Updater.(*updater[I, K, V])
	cmp := cfg.cmp
	up := cfg.upperBound(i.o.bounds)
	n := ll.Node()
	j := sort.Search(int(n.Count()), func(j int) bool {
		return !up.contains(cmp, cfg.key(n.Key(int16(j))))
	})
	i.o.constrMaxN = n
	i.o.constrMaxPos = int16(j)
}

func (i *Iterator[I, K, V]) findNextOverlap() {
	ll := lowLevel(i)
	cfg := ll.Config().Updater.(*updater[I, K, V])
	cmp := cfg.cmp
	for {
		if ll.Pos() > ll.Node().Count() {
			// Iterate up tree.
			ll.Ascend()
		} else if !ll.Node().IsLeaf() {
			// Iterate down tree.
			if i.o.constrMinReached || ll.ChildAug().contains(cmp, cfg.key(i.o.bounds)) {
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
			i.Reset()
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
			if cfg.upperBound(i.Key()).contains(cmp, cfg.key(i.o.bounds)) {
				return
			}
		}
		ll.IncrementPos()
	}
}
