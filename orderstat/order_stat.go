package orderstat

import "github.com/ajwerner/btree/new/abstract"

type OrderStatTree[K, V any] struct {
	t abstract.AugBTree[K, V, aug[K], *aug[K]]
}

func MakeOrderStatTree[K, V any](cmp func(K, K) int) *OrderStatTree[K, V] {
	return &OrderStatTree[K, V]{
		t: abstract.MakeBTree[K, V, aug[K]](cmp),
	}
}

func (t *OrderStatTree[K, V]) Set(k K, v V) {
	t.t.Set(k, v)
}

func (t *OrderStatTree[K, V]) Remove(k K) (removed bool) {
	return t.t.Delete(k)
}

type OrderStatIterator[K, V any] struct {
	it abstract.Iterator[K, V, aug[K], *aug[K]]
}

func (t *OrderStatTree[K, V]) MakeIter() OrderStatIterator[K, V] {
	return OrderStatIterator[K, V]{
		it: t.t.MakeIter(),
	}
}

func (it *OrderStatIterator[K, V]) Nth(i int) {
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

func (it *OrderStatIterator[K, V]) First()      { it.it.First() }
func (it *OrderStatIterator[K, V]) Last()       { it.it.First() }
func (it *OrderStatIterator[K, V]) Next()       { it.it.Next() }
func (it *OrderStatIterator[K, V]) Valid() bool { return it.it.Valid() }
func (it *OrderStatIterator[K, V]) Key() K      { return it.it.Key() }
