package interval

import "github.com/ajwerner/btree/new/abstract"

// Note this is totally incomplete

type Interval[K any] interface {
	Key() K
	End() K
}

type intervalAug[K any, I Interval[K]] struct{}

func (a *intervalAug[K, I]) Update(n abstract.Node[*intervalAug[K, I]]) {
}

func (a *intervalAug[K, I]) UpdateOnInsert(
	item I,
	n, child abstract.Node[*intervalAug[K, I]],
) (updated bool) {

	return false
}

func (a *intervalAug[K, I]) UpdateOnRemoval(
	item I,
	n, child abstract.Node[*intervalAug[K, I]],
) (updated bool) {
	return false
}

type IntervalTree[K, V any, I Interval[K]] struct {
	t abstract.Map[I, V, intervalAug[K, I], *intervalAug[K, I]]
}

func NewMap[I Interval[K], V, K any](cmp func(K, K) int) IntervalTree[K, V, I] {
	return IntervalTree[K, V, I]{
		t: abstract.MakeMap[I, V, intervalAug[K, I]](intervalCmp[K, I](cmp)),
	}
}

func intervalCmp[K any, I Interval[K]](f func(K, K) int) func(I, I) int {
	return func(a, b I) int {
		if c := f(a.Key(), b.Key()); c != 0 {
			return c
		}
		return f(a.End(), b.End())
	}
}

func (t *IntervalTree[K, V, I]) Upsert(k I, v V) {
	t.t.Upsert(k, v)
}

type Iterator[K, V any, I Interval[K]] struct {
	it abstract.Iterator[I, V, intervalAug[K, I], *intervalAug[K, I]]

	// The "soft" lower-bound constraint.
	constrMinN       abstract.Node[intervalAug[K, I]]
	constrMinPos     int16
	constrMinReached bool

	// The "hard" upper-bound constraint.
	constrMaxN   abstract.Node[intervalAug[K, I]]
	constrMaxPos int16
}

func (t *IntervalTree[K, V, I]) MakeIter() Iterator[K, V, I] {
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
