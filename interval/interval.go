package interval

import "github.com/ajwerner/btree/new/abstract"

// Interval represents an interval with bounds from [Key(), End()) where
// Key() is inclusive and End() is exclusive. If Key() == End(), then the
// Inteval represents a point that only includes that value. Intervals with
// Key() which is larger than End() are invalid and may result in panics
// upon insertion.
type Interval[K any] interface {
	Key() K
	End() K
}

type IntervalWithID[K, ID any] interface {
	Interval[K]
	ID() ID
}

type Map[K, V any, I Interval[K]] struct {
	t abstract.Map[I, V, CompareFn[K], aug[K, I], *aug[K, I]]
}

func NewMap[I Interval[K], V, K any](cmpK CompareFn[K], cmpI CompareFn[I]) Map[K, V, I] {
	return Map[K, V, I]{
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

func (t *Map[K, V, I]) Upsert(k I, v V) {
	t.t.Upsert(k, v)
}
