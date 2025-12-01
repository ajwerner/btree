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

	"github.com/anacrolix/btree/internal/abstract"
)

type Iterator[I, K, V any] struct {
	abstract.Iterator[I, V, aug[K]]

	o overlapScan[I, K, V]
}

// An overlap scan is a scan over all latches that overlap with the provided
// latch in order of the overlapping latches' start keys. The goal of the scan
// is to minimize the number of key comparisons performed in total. The
// algorithm operates based on the following two invariants maintained by
// augmented interval abstract:
// 1. all latches are sorted in the abstract based on their start key.
// 2. all abstract nodes maintain the upper bound end key of all latches
//    in their subtree.
//
// The scan algorithm starts in "unconstrained minimum" and "unconstrained
// maximum" states. To enter a "constrained minimum" state, the scan must reach
// latches in the tree with start keys above the search range's start key.
// Because latches in the tree are sorted by start key, once the scan enters the
// "constrained minimum" state it will remain there. To enter a "constrained
// maximum" state, the scan must determine the first child abstract node in a given
// subtree that can have latches with start keys above the search range's end
// key. The scan then remains in the "constrained maximum" state until it
// traverse into this child node, at which point it moves to the "unconstrained
// maximum" state again.
//
// The scan algorithm works like a standard abstract forward scan with the
// following augmentations:
// 1. before tranversing the tree, the scan performs a binary search on the
//    root node's items to determine a "soft" lower-bound constraint position
//    and a "hard" upper-bound constraint position in the root's children.
// 2. when tranversing into a child node in the lower or upper bound constraint
//    position, the constraint is refined by searching the child's items.
// 3. the initial traversal down the tree follows the left-most children
//    whose upper bound end keys are equal to or greater than the start key
//    of the search range. The children followed will be equal to or less
//    than the soft lower bound constraint.
// 4. once the initial tranversal completes and the scan is in the left-most
//    abstract node whose upper bound overlaps the search range, key comparisons
//    must be performed with each latch in the tree. This is necessary because
//    any of these latches may have end keys that cause them to overlap with the
//    search range.
// 5. once the scan reaches the lower bound constraint position (the first latch
//    with a start key equal to or greater than the search range's start key),
//    it can begin scaning without performing key comparisons. This is allowed
//    because all latches from this point forward will have end keys that are
//    greater than the search range's start key.
// 6. once the scan reaches the upper bound constraint position, it terminates.
//    It does so because the latch at this position is the first latch with a
//    start key larger than the search range's end key.
type overlapScan[I, K, V any] struct {
	bounds I
	set    bool

	// The "soft" lower-bound constraint.
	constrMinN       *abstract.Node[I, V, aug[K]]
	constrMinPos     int16
	constrMinReached bool

	// The "hard" upper-bound constraint.
	constrMaxN   *abstract.Node[I, V, aug[K]]
	constrMaxPos int16
}

func (o *overlapScan[I, K, V]) reset() {
	*o = overlapScan[I, K, V]{}
}

func (o *overlapScan[I, K, V]) empty() bool {
	return !o.set
}

// FirstOverlap seeks to the first latch in the abstract that overlaps with the
// provided search latch.
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
) *abstract.LowLevelIterator[I, V, aug[K]] {
	return abstract.LowLevel(&it.Iterator)
}

func (i *Iterator[I, K, V]) Reset() {
	i.o.reset()
	i.Iterator.Reset()
}

// NextOverlap positions the iterator to the latch immediately following
// its current position that overlaps with the search latch.
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
		return cmp(k, cfg.key(n.GetKey(int16(j)))) <= 0
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
		return !up.contains(cmp, cfg.key(n.GetKey(int16(j))))
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
			if i.o.constrMinReached || ll.Child().contains(cmp, cfg.key(i.o.bounds)) {
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
			// Check for overlapping latch.
			if i.o.constrMinReached {
				// Fast-path to avoid span comparison. i.o.constrMinReached
				// tells us that all latches have end keys above our search
				// span's start key.
				return
			}
			if cfg.upperBound(i.Cur()).contains(cmp, cfg.key(i.o.bounds)) {
				return
			}
		}
		ll.IncrementPos()
	}
}
