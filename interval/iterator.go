package interval

import (
	"sort"

	"github.com/ajwerner/btree/new/abstract"
)

type Iterator[K, V any, I Interval[K]] struct {
	it abstract.Iterator[I, V, CompareFn[K], aug[K, I], *aug[K, I]]

	o overlapScan[I, K]
}

func (t *Map[K, V, I]) MakeIter() Iterator[K, V, I] {
	return Iterator[K, V, I]{
		it: t.t.MakeIter(),
	}
}

func (it *Iterator[K, V, I]) First() {
	it.it.First()
}

func (it *Iterator[K, V, I]) Next() {
	it.it.Next()
}

func (it *Iterator[K, V, I]) Valid() bool {
	return it.it.Valid()
}

func (it *Iterator[K, V, I]) Key() I {
	return it.it.Key()
}

// An overlap scan is a scan over all latches that overlap with the provided
// latch in order of the overlapping latches' start keys. The goal of the scan
// is to minimize the number of key comparisons performed in total. The
// algorithm operates based on the following two invariants maintained by
// augmented interval btree:
// 1. all latches are sorted in the btree based on their start key.
// 2. all btree nodes maintain the upper bound end key of all latches
//    in their subtree.
//
// The scan algorithm starts in "unconstrained minimum" and "unconstrained
// maximum" states. To enter a "constrained minimum" state, the scan must reach
// latches in the tree with start keys above the search range's start key.
// Because latches in the tree are sorted by start key, once the scan enters the
// "constrained minimum" state it will remain there. To enter a "constrained
// maximum" state, the scan must determine the first child btree node in a given
// subtree that can have latches with start keys above the search range's end
// key. The scan then remains in the "constrained maximum" state until it
// traverse into this child node, at which point it moves to the "unconstrained
// maximum" state again.
//
// The scan algorithm works like a standard btree forward scan with the
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
//    btree node whose upper bound overlaps the search range, key comparisons
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
type overlapScan[I Interval[K], K any] struct {
	bounds I

	// The "soft" lower-bound constraint.
	constrMinN       abstract.Node[I, *aug[K, I]]
	constrMinPos     int16
	constrMinReached bool
	set              bool

	// The "hard" upper-bound constraint.
	constrMaxN   abstract.Node[I, *aug[K, I]]
	constrMaxPos int16
}

func (o *overlapScan[I, K]) reset() {
	*o = overlapScan[I, K]{}
}

func (o *overlapScan[I, K]) empty() bool {
	return !o.set
}

// FirstOverlap seeks to the first latch in the btree that overlaps with the
// provided search latch.
func (i *Iterator[K, V, I]) FirstOverlap(bounds I) {
	i.Reset()
	i.it.IncrementPos()
	if !i.it.Valid() {
		return
	}
	i.o = overlapScan[I, K]{bounds: bounds, set: true}
	i.constrainMinSearchBounds()
	i.constrainMaxSearchBounds()
	i.findNextOverlap()
}

func (i *Iterator[K, V, I]) Reset() {
	i.o.reset()
	i.it.Reset()
}

// NextOverlap positions the iterator to the latch immediately following
// its current position that overlaps with the search latch.
func (i *Iterator[K, V, I]) NextOverlap() {
	if !i.it.Valid() {
		return
	}
	if i.o.empty() {
		// Invalid. Mixed overlap scan with non-overlap scan.
		i.Reset()
		return
	}
	i.it.IncrementPos()
	i.findNextOverlap()
}

func (i *Iterator[K, V, I]) constrainMinSearchBounds() {
	k := i.o.bounds.Key()
	n := i.it.Node()
	cmp := i.it.Aux()
	j := sort.Search(int(n.Count()), func(j int) bool {
		return cmp(k, n.GetKey(int16(j)).Key()) <= 0
	})
	i.o.constrMinN = n
	i.o.constrMinPos = int16(j)
}

func (i *Iterator[K, V, I]) constrainMaxSearchBounds() {
	cmp := i.it.Aux()
	up := upperBound(i.o.bounds, cmp)
	n := i.it.Node()
	j := sort.Search(int(n.Count()), func(j int) bool {
		return !up.contains(cmp, n.GetKey(int16(j)).Key())
	})
	i.o.constrMaxN = n
	i.o.constrMaxPos = int16(j)
}

func (i *Iterator[K, V, I]) findNextOverlap() {
	cmp := i.it.Aux()
	for {
		if i.it.Pos() > i.it.Node().Count() {
			// Iterate up tree.
			i.it.Ascend()
		} else if !i.it.Node().IsLeaf() {
			// Iterate down tree.
			if i.o.constrMinReached || i.it.Child().contains(cmp, i.o.bounds.Key()) {
				par := i.it.Node()
				pos := i.it.Pos()
				i.it.Descend()

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
		if i.it.Node() == i.o.constrMaxN && i.it.Pos() == i.o.constrMaxPos {
			// Invalid. Past possible overlaps.
			i.Reset()
			return
		}
		if i.it.Node() == i.o.constrMinN && i.it.Pos() == i.o.constrMinPos {
			// The scan reached the soft lower-bound constraint.
			i.o.constrMinReached = true
		}

		// Iterate across node.
		if i.it.Pos() < i.it.Node().Count() {
			// Check for overlapping latch.
			if i.o.constrMinReached {
				// Fast-path to avoid span comparison. i.o.constrMinReached
				// tells us that all latches have end keys above our search
				// span's start key.
				return
			}
			if upperBound(i.it.Key(), cmp).contains(cmp, i.o.bounds.Key()) {
				return
			}
		}
		i.it.IncrementPos()
	}
}