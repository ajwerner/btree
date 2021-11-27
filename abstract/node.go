package abstract

import (
	"fmt"
	"strings"
	"sync/atomic"
)

type node[T Item[T], A any, AP Aug[T, A]] struct {
	ref      int32
	count    int16
	leaf     bool
	aug      A
	items    [MaxItems]T
	children [MaxItems + 1]*node[T, A, AP]
}

/*
type node[T Item[T], A any, AP Aug[T, A]] struct {
	// TODO(ajwerner): embed the leafNode here to avoid all of the ugly `N.N`
	// calls when that all works in go2 generics.
	n        leafNode[T, A, AP]

}
*/

func (n *node[T, A, AP]) GetA() *A {
	return &n.aug
}

func (n *node[T, A, AP]) IsLeaf() bool {
	return n.leaf
}

func (n *node[T, A, AP]) Count() int16 {
	return n.count
}

func (n *node[T, A, AP]) IterateItems(f func(T)) {
	for i := int16(0); i < n.count; i++ {
		f(n.items[i])
	}
}

func (n *node[T, A, AP]) IterateChildren(f func(*A)) {
	if n.leaf {
		return
	}
	for i := int16(0); i <= n.count; i++ {
		f(&n.children[i].aug)
	}
}

/*

//go:nocheckptr casts a ptr to a smaller struct to a ptr to a larger struct.
func leafToNode[T Item[T], A any, AP Aug[T, A]](ln *leafNode[T, A, AP]) *node[T, A, AP] {
	return (*node[T, A, AP])(unsafe.Pointer(ln))
}

func nodeToLeaf[T Item[T], A any, AP Aug[T, A]](n *node[T, A, AP]) *leafNode[T, A, AP] {
	return (*leafNode[T, A, AP])(unsafe.Pointer(n))
}
*/

func newLeafNode[T Item[T], A any, AP Aug[T, A]]() *node[T, A, AP] {
	/*
		n := leafToNode(new(leafNode[T, A, AP]))
		n.leaf = true
		n.ref = 1
	*/
	n := newNode[T, A, AP]()
	n.leaf = true
	return n

}

func newNode[T Item[T], A any, AP Aug[T, A]]() *node[T, A, AP] {
	n := new(node[T, A, AP])
	n.ref = 1
	return n
}

// mut creates and returns a mutable node reference. If the node is not shared
// with any other trees then it can be modified in place. Otherwise, it must be
// cloned to ensure unique ownership. In this way, we enforce a copy-on-write
// policy which transparently incorporates the idea of local mutations, like
// Clojure's transients or Haskell's ST monad, where nodes are only copied
// during the first time that they are modified between Clone operations.
//
// When a node is cloned, the provided pointer will be redirected to the new
// mutable node.
func mut[T Item[T], A any, AP Aug[T, A]](n **node[T, A, AP]) *node[T, A, AP] {
	if atomic.LoadInt32(&(*n).ref) == 1 {
		// Exclusive ownership. Can mutate in place.
		return *n
	}
	// If we do not have unique ownership over the node then we
	// clone it to gain unique ownership. After doing so, we can
	// release our reference to the old node. We pass recursive
	// as true because even though we just observed the node's
	// reference count to be greater than 1, we might be racing
	// with another call to decRef on this node.
	c := (*n).clone()
	(*n).decRef(true /* recursive */)
	*n = c
	return *n
}

// incRef acquires a reference to the node.
func (n *node[T, A, AP]) incRef() {
	atomic.AddInt32(&n.ref, 1)
}

// decRef releases a reference to the node. If requested, the method
// will recurse into child nodes and decrease their refcounts as well.
func (n *node[T, A, AP]) decRef(recursive bool) {
	if atomic.AddInt32(&n.ref, -1) > 0 {
		// Other references remain. Can't free.
		return
	}
	// Clear and release node into memory pool.
	if n.leaf {
		// TODO(ajwerner): pooling
	} else {
		// Release child references first, if requested.
		if recursive {
			for i := int16(0); i <= n.count; i++ {
				n.children[i].decRef(true /* recursive */)
			}
		}
		// TODO(ajwerner): pooling
	}
}

// clone creates a clone of the receiver with a single reference count.
func (n *node[T, A, AP]) clone() *node[T, A, AP] {
	var c *node[T, A, AP]
	if n.leaf {
		c = newLeafNode[T, A, AP]()
	} else {
		c = newNode[T, A, AP]()
	}
	// NB: copy field-by-field without touching N.N.ref to avoid
	// triggering the race detector and looking like a data race.
	c.count = n.count
	AP(&n.aug).CopyInto(&c.aug)
	c.items = n.items
	if !c.leaf {
		// Copy children and increase each refcount.
		c.children = n.children
		for i := int16(0); i <= c.count; i++ {
			c.children[i].incRef()
		}
	}
	return c
}

func (n *node[T, A, AP]) insertAt(index int, item T, nd *node[T, A, AP]) {
	if index < int(n.count) {
		copy(n.items[index+1:n.count+1], n.items[index:n.count])
		if !n.leaf {
			copy(n.children[index+2:n.count+2], n.children[index+1:n.count+1])
		}
	}
	n.items[index] = item
	if !n.leaf {
		n.children[index+1] = nd
	}
	n.count++
}

func (n *node[T, A, AP]) pushBack(item T, nd *node[T, A, AP]) {
	n.items[n.count] = item
	if !n.leaf {
		n.children[n.count+1] = nd
	}
	n.count++
}

func (n *node[T, A, AP]) pushFront(item T, nd *node[T, A, AP]) {
	if !n.leaf {
		copy(n.children[1:n.count+2], n.children[:n.count+1])
		n.children[0] = nd
	}
	copy(n.items[1:n.count+1], n.items[:n.count])
	n.items[0] = item
	n.count++
}

// removeAt removes a value at a given index, pulling all subsequent values
// back.
func (n *node[T, A, AP]) removeAt(index int) (T, *node[T, A, AP]) {
	var child *node[T, A, AP]
	if !n.leaf {
		child = n.children[index+1]
		copy(n.children[index+1:n.count], n.children[index+2:n.count+1])
		n.children[n.count] = nil
	}
	n.count--
	out := n.items[index]
	copy(n.items[index:n.count], n.items[index+1:n.count+1])
	var r T
	n.items[n.count] = r
	return out, child
}

// popBack removes and returns the last element in the list.
func (n *node[T, A, AP]) popBack() (T, *node[T, A, AP]) {
	n.count--
	out := n.items[n.count]
	var r T
	n.items[n.count] = r
	if n.leaf {
		return out, nil
	}
	child := n.children[n.count+1]
	n.children[n.count+1] = nil
	return out, child
}

// popFront removes and returns the first element in the list.
func (n *node[T, A, AP]) popFront() (T, *node[T, A, AP]) {
	n.count--
	var child *node[T, A, AP]
	if !n.leaf {
		child = n.children[0]
		copy(n.children[:n.count+1], n.children[1:n.count+2])
		n.children[n.count+1] = nil
	}
	out := n.items[0]
	copy(n.items[:n.count], n.items[1:n.count+1])
	var r T
	n.items[n.count] = r
	return out, child
}

// find returns the index where the given item should be inserted into this
// list. 'found' is true if the item already exists in the list at the given
// index.
func (n *node[T, A, AP]) find(item T) (index int, found bool) {
	// Logic copied from sort.Search. Inlining this gave
	// an 11% speedup on BenchmarkBTreeDeleteInsert.
	i, j := 0, int(n.count)
	for i < j {
		h := int(uint(i+j) >> 1) // avoid overflow when computing h
		// i ≤ h < j
		if item.Less(n.items[h]) {
			j = h
		} else if n.items[h].Less(item) {
			i = h + 1
		} else {
			return h, true
		}
	}
	return i, false
}

// split splits the given node at the given index. The current node shrinks,
// and this function returns the item that existed at that index and a new
// node containing all items/children after it.
//
// Before:
//
//          +-----------+
//          |   x y z   |
//          +--/-/-\-\--+
//
// After:
//
//          +-----------+
//          |     y     |
//          +----/-\----+
//              /   \
//             v     v
// +-----------+     +-----------+
// |         x |     | z         |
// +-----------+     +-----------+
//
func (n *node[T, A, AP]) split(i int) (T, *node[T, A, AP]) {
	out := n.items[i]
	var next *node[T, A, AP]
	if n.leaf {
		next = newLeafNode[T, A, AP]()
	} else {
		next = newNode[T, A, AP]()
	}
	next.count = n.count - int16(i+1)
	copy(next.items[:], n.items[i+1:n.count])
	var r T
	for j := int16(i); j < n.count; j++ {
		n.items[j] = r
	}
	if !n.leaf {
		copy(next.children[:], n.children[i+1:n.count+1])
		for j := int16(i + 1); j <= n.count; j++ {
			n.children[j] = nil
		}
	}
	n.count = int16(i)
	fmt.Println("here")
	AP(&n.aug).Update(Node[*A](n))

	AP(&n.aug).Update(Node[*A](next))
	//AP(&N.N.aug).UpdateOnSplit(next)
	/*
		if N.max.compare(next.max) != 0 && N.max.compare(upperBound(out)) != 0 {
			// If upper bound wasn't from new node or item
			// at index i, it must still be from old node.
		} else {
			N.max = N.findUpperBound()
		}
	*/
	return out, next
}

// insert inserts an item into the suAugBTree rooted at this node, making sure no
// nodes in the suAugBTree exceed MaxItems items. Returns true if an existing item
// was replaced and false if an item was inserted. Also returns whether the
// node's upper bound changes.
func (n *node[T, A, AP]) insert(item T) (replaced, newBound bool) {
	i, found := n.find(item)
	if found {
		n.items[i] = item
		return true, false
	}
	if n.leaf {
		n.insertAt(i, item, nil)
		return false, AP(&n.aug).UpdateOnInsert(item, n, nil)
	}
	if n.children[i].count >= MaxItems {
		splitLa, splitNode := mut(&n.children[i]).split(MaxItems / 2)
		n.insertAt(i, splitLa, splitNode)
		if item.Less(n.items[i]) {
			// no change, we want first split node
		} else if n.items[i].Less(item) {
			i++ // we want second split node
		} else {
			// TODO(ajwerner): add something to the augmentation api to
			// deal with replacement.
			n.items[i] = item
			return true, false
		}
	}
	replaced, newBound = mut(&n.children[i]).insert(item)
	if newBound {
		newBound = AP(&n.aug).UpdateOnInsert(item, n, nil)
	}
	return replaced, newBound
}

// removeMax removes and returns the maximum item from the suAugBTree rooted at
// this node.
func (n *node[T, A, AP]) removeMax() T {
	if n.leaf {
		n.count--
		out := n.items[n.count]
		var r T
		n.items[n.count] = r
		AP(&n.aug).UpdateOnRemoval(out, n, nil)
		return out
	}
	// Recurse into max child.
	i := int(n.count)
	if n.children[i].count <= MinItems {
		// Child not large enough to remove from.
		n.rebalanceOrMerge(i)
		return n.removeMax() // redo
	}
	child := mut(&n.children[i])
	out := child.removeMax()
	AP(&n.aug).UpdateOnRemoval(out, n, nil)
	return out
}

// rebalanceOrMerge grows child 'i' to ensure it has sufficient room to remove
// an item from it while keeping it at or above MinItems.
func (n *node[T, A, AP]) rebalanceOrMerge(i int) {
	switch {
	case i > 0 && n.children[i-1].count > MinItems:
		// Rebalance from left sibling.
		//
		//          +-----------+
		//          |     y     |
		//          +----/-\----+
		//              /   \
		//             v     v
		// +-----------+     +-----------+
		// |         x |     |           |
		// +----------\+     +-----------+
		//             \
		//              v
		//              a
		//
		// After:
		//
		//          +-----------+
		//          |     x     |
		//          +----/-\----+
		//              /   \
		//             v     v
		// +-----------+     +-----------+
		// |           |     | y         |
		// +-----------+     +/----------+
		//                   /
		//                  v
		//                  a
		//
		left := mut(&n.children[i-1])
		child := mut(&n.children[i])
		xLa, grandChild := left.popBack()
		yLa := n.items[i-1]
		child.pushFront(yLa, grandChild)
		n.items[i-1] = xLa

		AP(&left.aug).UpdateOnRemoval(xLa, left, grandChild)
		AP(&child.aug).UpdateOnInsert(yLa, child, grandChild)

	case i < int(n.count) && n.children[i+1].count > MinItems:
		// Rebalance from right sibling.
		//
		//          +-----------+
		//          |     y     |
		//          +----/-\----+
		//              /   \
		//             v     v
		// +-----------+     +-----------+
		// |           |     | x         |
		// +-----------+     +/----------+
		//                   /
		//                  v
		//                  a
		//
		// After:
		//
		//          +-----------+
		//          |     x     |
		//          +----/-\----+
		//              /   \
		//             v     v
		// +-----------+     +-----------+
		// |         y |     |           |
		// +----------\+     +-----------+
		//             \
		//              v
		//              a
		//
		right := mut(&n.children[i+1])
		child := mut(&n.children[i])
		xLa, grandChild := right.popFront()
		yLa := n.items[i]
		child.pushBack(yLa, grandChild)
		n.items[i] = xLa

		AP(&right.aug).UpdateOnRemoval(xLa, right, grandChild)
		AP(&child.aug).UpdateOnInsert(yLa, child, grandChild)

	default:
		// Merge with either the left or right sibling.
		//
		//          +-----------+
		//          |   u y v   |
		//          +----/-\----+
		//              /   \
		//             v     v
		// +-----------+     +-----------+
		// |         x |     | z         |
		// +-----------+     +-----------+
		//
		// After:
		//
		//          +-----------+
		//          |    u v    |
		//          +-----|-----+
		//                |
		//                v
		//          +-----------+
		//          |   x y z   |
		//          +-----------+
		//
		if i >= int(n.count) {
			i = int(n.count - 1)
		}
		child := mut(&n.children[i])
		// Make mergeChild mutable, bumping the refcounts on its children if necessary.
		_ = mut(&n.children[i+1])
		mergeLa, mergeChild := n.removeAt(i)
		child.items[child.count] = mergeLa
		copy(child.items[child.count+1:], mergeChild.items[:mergeChild.count])
		if !child.leaf {
			copy(child.children[child.count+1:], mergeChild.children[:mergeChild.count+1])
		}
		child.count += mergeChild.count + 1

		AP(&child.aug).UpdateOnInsert(mergeLa, child, mergeChild)
		mergeChild.decRef(false /* recursive */)
	}
}

// remove removes an item from the suAugBTree rooted at this node. Returns the item
// that was removed or nil if no matching item was found. Also returns whether
// the node's upper bound changes.
func (n *node[T, A, AP]) remove(item T) (out T, found, newBound bool) {
	i, found := n.find(item)
	if n.leaf {
		if found {
			out, _ = n.removeAt(i)
			return out, true, AP(&n.aug).UpdateOnRemoval(out, n, nil)
		}
		var r T
		return r, false, false
	}
	if n.children[i].count <= MinItems {
		// Child not large enough to remove from.
		n.rebalanceOrMerge(i)
		return n.remove(item) // redo
	}
	child := mut(&n.children[i])
	if found {
		// Replace the item being removed with the max item in our left child.
		out = n.items[i]
		n.items[i] = child.removeMax()
		return out, true, AP(&n.aug).UpdateOnRemoval(out, n, nil)
	}
	// Latch is not in this node and child is large enough to remove from.
	out, found, newBound = child.remove(item)
	if newBound {
		newBound = AP(&n.aug).UpdateOnRemoval(out, n, nil)
	}
	return out, found, newBound
}

func (n *node[T, A, AP]) writeString(b *strings.Builder) {
	if n.leaf {
		for i := int16(0); i < n.count; i++ {
			if i != 0 {
				b.WriteString(",")
			}
			fmt.Fprintf(b, "%v", n.items[i])
		}
		return
	}
	for i := int16(0); i <= n.count; i++ {
		b.WriteString("(")
		n.children[i].writeString(b)
		b.WriteString(")")
		if i < n.count {
			fmt.Fprintf(b, "%v", n.items[i])
		}
	}
}
