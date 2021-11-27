package orderstat

import btree "github.com/ajwerner/btree/new/abstract"

type orderStatAug[T btree.Item[T]] struct {
	// children is the number of items rooted at the current subtree.
	children int
}

func (a *orderStatAug[T]) CopyInto(dest *orderStatAug[T]) {
	*dest = *a
}

// Update will update the count for the current node.
func (a *orderStatAug[T]) Update(n btree.Node[*orderStatAug[T]]) {
	var children int
	if !n.IsLeaf() {
		var it btree.NodeIterator[T, orderStatAug[T], *orderStatAug[T]]
		btree.InitNodeIterator(&it, n)
		N := int(n.Count())
		for i := 0; i <= N; i++ {
			if child := it.Child(i); child != nil {
				children += child.GetA().children
			}
		}
	}
	children += int(n.Count())
	a.children = children
}

func (a *orderStatAug[T]) UpdateOnInsert(
	item T,
	n, child btree.Node[*orderStatAug[T]],
) (updated bool) {
	// TODO(ajwerner): optimize this.
	a.Update(n)
	return true
}

func (a *orderStatAug[T]) UpdateOnRemoval(
	item T,
	n, child btree.Node[*orderStatAug[T]],
) (updated bool) {
	// TODO(ajwerner): optimize this.
	a.Update(n)
	return true
}

type OrderStatTree[T btree.Item[T]] struct {
	t btree.AugBTree[T, orderStatAug[T], *orderStatAug[T]]
}

func MakeOrderStatTree[T btree.Item[T]]() *OrderStatTree[T] {
	return &OrderStatTree[T]{
		btree.AugBTree[T, orderStatAug[T], *orderStatAug[T]]{},
	}
}

func (t *OrderStatTree[T]) Set(v T) {
	t.t.Set(v)
}

type OrderStatIterator[T btree.Item[T]] struct {
	it btree.Iterator[T, orderStatAug[T], *orderStatAug[T]]
}

func (t *OrderStatTree[T]) MakeIter() OrderStatIterator[T] {
	return OrderStatIterator[T]{
		it: t.t.MakeIter(),
	}
}

func (it *OrderStatIterator[T]) Nth(i int) {
	it.it.Reset()
	it.it.Pos = 0
	n := 0
	var nit btree.NodeIterator[T, orderStatAug[T], *orderStatAug[T]]
	for n <= i {
		btree.InitNodeIterator(&nit, it.it.Node())
		if !it.it.n.IsLeaf() && it.it.N.asd[it.it.Pos] != nil {
			it.it.Descend(it.it.N, it.it.Pos)
			if n+it.it.n.n.aug.children <= i {
				n += it.it.n.n.aug.children
				it.it.Ascend()
				if n == i {
					return
				}
				n++
				it.it.pos++
			} else if it.it.pos > 0 {
				// the result is in this subtree...
				n++
			}
		} else {
			// We're in the correct leaf, should be able to use sort.Search.
			if n == i {
				return
			}
			n++
			it.it.pos++
		}
	}
}

func (it *OrderStatIterator[T]) First() {
	it.it.First()
}

func (it *OrderStatIterator[T]) Next() {
	it.it.Next()
}

func (it *OrderStatIterator[T]) Valid() bool {
	return it.it.Valid()
}

func (it *OrderStatIterator[T]) Cur() T {
	return T(it.it.Cur())
}
