package orderstat

import (
	"fmt"

	"github.com/ajwerner/btree/new/abstract"
)

type Map[K, V any] struct {
	t abstract.Map[K, V, aug[K], *aug[K]]
}

func NewMap[K, V any](cmp func(K, K) int) *Map[K, V] {
	return &Map[K, V]{
		t: abstract.MakeMap[K, V, aug[K]](cmp),
	}
}

type Set[K any] struct {
	t abstract.Map[K, struct{}, aug[K], *aug[K]]
}

func NewSet[K any](cmp func(K, K) int) *Set[K] {
	return &Set[K]{
		t: abstract.MakeMap[K, struct{}, aug[K]](cmp),
	}
}

func (t *Set[K]) Upsert(k K) (replaced K, overwrote bool) {
	replaced, _, overwrote = t.t.Upsert(k, struct{}{})
	return replaced, overwrote
}

func (t *Set[K]) Delete(k K) (removed bool) {
	_, _, removed = t.t.Delete(k)
	return removed
}

func (t *Map[K, V]) Upsert(k K, v V) (replacedV V, overwrote bool) {
	_, replacedV, overwrote = t.t.Upsert(k, v)
	return replacedV, overwrote
}

func (t *Map[K, V]) Delete(k K) (removedVal V, removed bool) {
	_, removedVal, removed = t.t.Delete(k)
	return removedVal, removed
}

type Iterator[K, V any] struct {
	it abstract.Iterator[K, V, aug[K], *aug[K]]
}

func (t *Map[K, V]) Iterator() Iterator[K, V] {
	return Iterator[K, V]{it: t.t.MakeIter()}
}

func (t *Set[K]) Iterator() Iterator[K, struct{}] {
	return Iterator[K, struct{}]{it: t.t.MakeIter()}
}

func (it *Iterator[K, V]) Nth(i int) {
	seekNth[K](&it.it, i)
}

type iteratorForSeek[K any] interface {
	Reset()
	IsLeaf() bool
	IncrementPos() bool
	SetPos(int16) bool
	Child() (aug[K], bool)
	Descend()
}

var onErrorf = func(format string, args ...interface{}) {
	panic(fmt.Errorf(format, args...))
}

func seekNth[K any, It iteratorForSeek[K]](it It, nth int) {
	// Reset has bizarre semantics in that it initializes the iterator to
	// an invalid position (-1) at the root of the tree. IncrementPos moves it
	// to the first child and item of the
	it.Reset()
	it.IncrementPos()
	n := 0
	for n <= nth {
		if it.IsLeaf() {
			// If we're in the leaf, then, by construction, we can find
			// the relevant position and seek to it in constant time.
			//
			// TODO(ajwerner): Add more invariant checking.
			it.SetPos(int16(nth - n))
			return
		}
		a, ok := it.Child()
		if !ok {
			onErrorf("failed to visit child")
		}
		if n+a.children > nth {
			it.Descend()
			continue
		}

		n += a.children
		switch {
		case n < nth:
			// Consume the current value, move on to the next one.
			n++
			it.IncrementPos()
		case n == nth:
			return // found it
		default:
			onErrorf("invariant violated")
		}
	}
}

func (it *Iterator[K, V]) First()      { it.it.First() }
func (it *Iterator[K, V]) Last()       { it.it.First() }
func (it *Iterator[K, V]) Next()       { it.it.Next() }
func (it *Iterator[K, V]) Prev()       { it.it.Prev() }
func (it *Iterator[K, V]) SeekGE(k K)  { it.it.SeekGE(k) }
func (it *Iterator[K, V]) SeekLT(k K)  { it.it.SeekLT(k) }
func (it *Iterator[K, V]) Valid() bool { return it.it.Valid() }
func (it *Iterator[K, V]) Key() K      { return it.it.Key() }
func (it *Iterator[K, V]) Value() V    { return it.it.Value() }
