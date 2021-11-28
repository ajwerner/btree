package orderstat

import "github.com/ajwerner/btree/new/abstract"

type OrderStatTree[T abstract.Item[T]] struct {
	t abstract.AugBTree[T, aug[T], *aug[T]]
}

func MakeOrderStatTree[T abstract.Item[T]]() *OrderStatTree[T] {
	return &OrderStatTree[T]{
		abstract.AugBTree[T, aug[T], *aug[T]]{},
	}
}

func (t *OrderStatTree[T]) Set(v T) {
	t.t.Set(v)
}

func (t *OrderStatTree[T]) Remove(v T) (removed bool) {
	return t.t.Delete(v)
}

type OrderStatIterator[T abstract.Item[T]] struct {
	it abstract.Iterator[T, aug[T], *aug[T]]
}

func (t *OrderStatTree[T]) MakeIter() OrderStatIterator[T] {
	return OrderStatIterator[T]{
		it: t.t.MakeIter(),
	}
}

func (it *OrderStatIterator[T]) Nth(i int) {
	// Reset has bizarre semantics in that it initializes the iterator to
	// an invalid position (-1) at the root of the tree. IncrementPos moves it
	// to the first child and item of the
	it.it.Reset()
	it.it.IncrementPos()
	n := 0
	for n <= i {
		if it.it.IsLeaf() {
			it.it.SetPos(int16(i - n))
			break
		} else {
			curChild := it.it.CurChild()
			if children := curChild.GetA().children; n+children <= i {
				n += children
				if n < i {
					n++
					it.it.IncrementPos()
				} else if i == n {
					break
				} else if n > i {
					panic("invariant violated")
				}
			} else {
				it.it.Descend()
			}
		}
	}
}

func (it *OrderStatIterator[T]) First()      { it.it.First() }
func (it *OrderStatIterator[T]) Last()       { it.it.First() }
func (it *OrderStatIterator[T]) Next()       { it.it.Next() }
func (it *OrderStatIterator[T]) Valid() bool { return it.it.Valid() }
func (it *OrderStatIterator[T]) Cur() T      { return it.it.Cur() }
