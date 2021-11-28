package btree

import "github.com/ajwerner/btree/new/abstract"

type noopAug[T abstract.Item[T]] struct{}

func (a *noopAug[T]) CopyInto(*noopAug[T]) {
}

func (a *noopAug[T]) Update(n abstract.Node[*noopAug[T]]) {
}

func (a *noopAug[T]) UpdateOnInsert(item T, n, child abstract.Node[*noopAug[T]]) (updated bool) {
	return false
}
func (a *noopAug[T]) UpdateOnRemoval(item T, n, child abstract.Node[*noopAug[T]]) (updated bool) {
	return false
}

type BTree[T abstract.Item[T]] struct {
	t abstract.AugBTree[T, noopAug[T], *noopAug[T]]
}

func MakeBTree[T abstract.Item[T]]() *BTree[T] {
	return &BTree[T]{
		abstract.AugBTree[T, noopAug[T], *noopAug[T]]{},
	}
}

func (t *BTree[T]) Set(v T) {
	t.t.Set(v)
}

type BTreeIterator[T abstract.Item[T]] struct {
	it abstract.Iterator[T, noopAug[T], *noopAug[T]]
}

func (t *BTree[T]) MakeIter() BTreeIterator[T] {
	return BTreeIterator[T]{t.t.MakeIter()}
}

func (it *BTreeIterator[T]) First() { it.it.First() }

func (it *BTreeIterator[T]) Next() { it.it.Next() }

func (it *BTreeIterator[T]) Valid() bool { return it.it.Valid() }

func (it *BTreeIterator[T]) Cur() T { return it.it.Cur() }
