package interval

import "github.com/ajwerner/btree/new/abstract"

// Note this is totally incomplete

type Interval[T abstract.Item[T], I any] interface {
	abstract.Item[I]
	Key() T
	End() T
}

type intervalAug[T abstract.Item[T], I Interval[T, I]] struct{}

func (a *intervalAug[T, I]) CopyInto(dest *intervalAug[T, I]) {
	*dest = *a
}

func (a *intervalAug[T, I]) Update(n abstract.Node[*intervalAug[T, I]]) {
}

func (a *intervalAug[T, I]) UpdateOnInsert(
	item I,
	n, child abstract.Node[*intervalAug[T, I]],
) (updated bool) {

	return false
}

func (a *intervalAug[T, I]) UpdateOnRemoval(
	item I,
	n, child abstract.Node[*intervalAug[T, I]],
) (updated bool) {
	return false
}

type IntervalTree[T abstract.Item[T], I Interval[T, I]] struct {
	t abstract.AugBTree[I, intervalAug[T, I], *intervalAug[T, I]]
}

func MakeIntervalTree[T abstract.Item[T], I Interval[T, I]]() IntervalTree[T, I] {
	return IntervalTree[T, I]{
		abstract.AugBTree[I, intervalAug[T, I], *intervalAug[T, I]]{},
	}
}

func (t *IntervalTree[T, I]) Set(v I) {
	t.t.Set(v)
}

type IntervalIterator[T abstract.Item[T], I Interval[T, I]] struct {
	it abstract.Iterator[I, intervalAug[T, I], *intervalAug[T, I]]

	// The "soft" lower-bound constraint.
	constrMinN       abstract.Node[intervalAug[T, I]]
	constrMinPos     int16
	constrMinReached bool

	// The "hard" upper-bound constraint.
	constrMaxN   abstract.Node[intervalAug[T, I]]
	constrMaxPos int16
}

func (t *IntervalTree[T, I]) MakeIter() IntervalIterator[T, I] {
	return IntervalIterator[T, I]{
		it: t.t.MakeIter(),
	}
}

func (it *IntervalIterator[T, I]) First() {
	it.it.First()
}

func (it *IntervalIterator[T, I]) Next() {
	it.it.Next()
}

func (it *IntervalIterator[T, I]) Valid() bool {
	return it.it.Valid()
}

func (it *IntervalIterator[T, I]) Cur() I {
	return I(it.it.Cur())
}
