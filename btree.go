package btree

import "github.com/ajwerner/btree/new/abstract"

type Map[K, V any] struct {
	t abstract.Map[K, V, struct{}, aug[K], *aug[K]]
}

func NewMap[K, V any](cmp func(K, K) int) *Map[K, V] {
	return &Map[K, V]{
		t: abstract.MakeMap[K, V, struct{}, aug[K]](struct{}{}, cmp),
	}
}

func (t *Map[K, V]) Upsert(k K, v V) (replacedV V, replaced bool) {
	_, replacedV, replaced = t.t.Upsert(k, v)
	return replacedV, replaced
}

func (t *Map[K, V]) Delete(k K) (removedV V, removed bool) {
	_, removedV, removed = t.t.Delete(k)
	return removedV, removed
}

func (t *Map[K, V]) Iterator() Iterator[K, V] {
	return Iterator[K, V]{t.t.MakeIter()}
}

type Set[K any] struct {
	t abstract.Map[K, struct{}, struct{}, aug[K], *aug[K]]
}

func NewSet[K any](cmp func(K, K) int) *Set[K] {
	return &Set[K]{
		t: abstract.MakeMap[K, struct{}, struct{}, aug[K]](struct{}{}, cmp),
	}
}

func (t *Set[K]) Upsert(k K) (replacedK K, replaced bool) {
	replacedK, _, replaced = t.t.Upsert(k, struct{}{})
	return replacedK, replaced
}

func (t *Set[K]) Delete(k K) (removedK K, removed bool) {
	removedK, _, removed = t.t.Delete(k)
	return removedK, removed
}

func (t *Set[K]) Iterator() Iterator[K, struct{}] {
	return Iterator[K, struct{}]{t.t.MakeIter()}
}

type Iterator[K, V any] struct {
	it abstract.Iterator[K, V, struct{}, aug[K], *aug[K]]
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

type aug[K any] struct{}

func (a *aug[T]) Update(n abstract.Node[*aug[T]], md abstract.UpdateMeta[T, struct{}, aug[T]]) bool {
	return false
}
