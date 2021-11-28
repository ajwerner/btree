package interval

import "github.com/ajwerner/btree/new/abstract"

// Note this is totally incomplete

type Interval[K any] interface {
	Key() K
	End() K
}

type IntervalWithID[K, ID any] interface {
	Interval[K]
	ID() ID
}

type kBound[K any] struct {
	k         K
	inclusive bool
}

type aug[K any, I Interval[K]] struct {
	kBound[K]
}

func (a *aug[K, I]) Update(n abstract.Node[*aug[K, I]], md abstract.UpdateMeta[I, CompareFn[K], aug[K, I]]) (updated bool) {
	return false
}

type IntervalTree[K, V any, I Interval[K]] struct {
	t abstract.Map[I, V, CompareFn[K], aug[K, I], *aug[K, I]]
}

func NewMap[I Interval[K], V, K any](cmpK CompareFn[K], cmpI CompareFn[I]) IntervalTree[K, V, I] {
	return IntervalTree[K, V, I]{
		t: abstract.MakeMap[I, V, CompareFn[K], aug[K, I]](cmpK, cmpI),
	}
}

type CompareFn[T any] func(T, T) int

func IntervalIDCompare[K, ID any, I IntervalWithID[K, ID]](cmpK CompareFn[K], cmpID CompareFn[ID]) CompareFn[I] {
	return func(a, b I) int {
		if c := cmpK(a.Key(), b.Key()); c != 0 {
			return c
		}
		if c := cmpK(a.End(), b.End()); c != 0 {
			return c
		}
		return cmpID(a.ID(), b.ID())
	}
}

func IntervalCompare[I Interval[K], K any](f func(K, K) int) func(I, I) int {
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
	it abstract.Iterator[I, V, CompareFn[K], aug[K, I], *aug[K, I]]

	// The "soft" lower-bound constraint.
	constrMinN       abstract.Node[aug[K, I]]
	constrMinPos     int16
	constrMinReached bool

	// The "hard" upper-bound constraint.
	constrMaxN   abstract.Node[aug[K, I]]
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
