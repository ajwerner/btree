package btree

import "github.com/ajwerner/btree/new/abstract"

type noopAug[K any] struct{}

func (a *noopAug[T]) Update(n abstract.Node[*noopAug[T]]) {
}

func (a *noopAug[T]) UpdateOnInsert(item T, n, child abstract.Node[*noopAug[T]]) (updated bool) {
	return false
}
func (a *noopAug[T]) UpdateOnRemoval(item T, n, child abstract.Node[*noopAug[T]]) (updated bool) {
	return false
}

type BTree[K, V any] struct {
	t abstract.AugBTree[K, V, noopAug[K], *noopAug[K]]
}

func New[K, V any](cmp func(K, K) int) *BTree[K, V] {
	return &BTree[K, V]{
		t: abstract.MakeBTree[K, V, noopAug[K], *noopAug[K]](cmp),
	}
}

func (t *BTree[K, V]) Insert(k K, v V) {
	t.t.Set(k, v)
}

type BTreeIterator[K, V any] struct {
	it abstract.Iterator[K, V, noopAug[K], *noopAug[K]]
}

func (t *BTree[K, V]) MakeIter() BTreeIterator[K, V] {
	return BTreeIterator[K, V]{t.t.MakeIter()}
}

func (it *BTreeIterator[K, V]) First() { it.it.First() }

func (it *BTreeIterator[K, V]) Next() { it.it.Next() }

func (it *BTreeIterator[K, V]) Valid() bool { return it.it.Valid() }

func (it *BTreeIterator[K, V]) Key() K   { return it.it.Key() }
func (it *BTreeIterator[K, V]) Value() V { return it.it.Value() }
