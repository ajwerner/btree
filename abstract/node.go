package abstract

import (
	"fmt"
	"strings"
	"sync/atomic"
)

type node[K, V, A any, AP Aug[K, A]] struct {
	ref      int32
	count    int16
	leaf     bool
	aug      A
	keys     [MaxEntries]K
	values   [MaxEntries]V
	children [MaxEntries + 1]*node[K, V, A, AP]
}

func (n *node[K, V, A, AP]) GetA() *A {
	return &n.aug
}

func (n *node[K, V, A, AP]) IsLeaf() bool {
	return n.leaf
}

func (n *node[K, V, A, AP]) Count() int16 {
	return n.count
}

func (n *node[K, V, A, AP]) IterateItems(f func(K, V)) {
	for i := int16(0); i < n.count; i++ {
		f(n.keys[i], n.values[i])
	}
}

func (n *node[K, V, A, AP]) GetChild(i int16) Node[*A] {
	if !n.leaf && n.children[i] != nil {
		return n.children[i]
	}
	return nil
}

func (n *node[K, V, A, AP]) IterateChildren(f func(*A)) {
	if n.leaf {
		return
	}
	for i := int16(0); i <= n.count; i++ {
		f(&n.children[i].aug)
	}
}

/*

//go:nocheckptr casts a ptr to a smaller struct to a ptr to a larger struct.
func leafToNode[K, V, A any, AP Aug[K, A]](ln *leafNode[K, V, A, AP]) *node[K, V, A, AP] {
	return (*node[K, V, A, AP])(unsafe.Pointer(ln))
}

func nodeToLeaf[K, V, A any, AP Aug[K, A]](n *node[K, V, A, AP]) *leafNode[K, V, A, AP] {
	return (*leafNode[K, V, A, AP])(unsafe.Pointer(n))
}
*/

func newLeafNode[K, V, A any, AP Aug[K, A]]() *node[K, V, A, AP] {
	/*
		n := leafToNode(new(leafNode[K, V, A, AP]))
		n.leaf = true
		n.ref = 1
	*/
	n := newNode[K, V, A, AP]()
	n.leaf = true
	return n

}

func newNode[K, V, A any, AP Aug[K, A]]() *node[K, V, A, AP] {
	n := new(node[K, V, A, AP])
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
func mut[K, V, A any, AP Aug[K, A]](n **node[K, V, A, AP]) *node[K, V, A, AP] {
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
func (n *node[K, V, A, AP]) incRef() {
	atomic.AddInt32(&n.ref, 1)
}

// decRef releases a reference to the node. If requested, the method
// will recurse into child nodes and decrease their refcounts as well.
func (n *node[K, V, A, AP]) decRef(recursive bool) {
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
func (n *node[K, V, A, AP]) clone() *node[K, V, A, AP] {
	var c *node[K, V, A, AP]
	if n.leaf {
		c = newLeafNode[K, V, A, AP]()
	} else {
		c = newNode[K, V, A, AP]()
	}
	// NB: copy field-by-field without touching N.N.ref to avoid
	// triggering the race detector and looking like a data race.
	c.count = n.count
	n.aug = c.aug
	c.keys = n.keys
	if !c.leaf {
		// Copy children and increase each refcount.
		c.children = n.children
		for i := int16(0); i <= c.count; i++ {
			c.children[i].incRef()
		}
	}
	return c
}

func (n *node[K, V, A, AP]) insertAt(index int, item K, value V, nd *node[K, V, A, AP]) {
	if index < int(n.count) {
		copy(n.keys[index+1:n.count+1], n.keys[index:n.count])
		if !n.leaf {
			copy(n.children[index+2:n.count+2], n.children[index+1:n.count+1])
		}
	}
	n.keys[index] = item
	n.values[index] = value
	if !n.leaf {
		n.children[index+1] = nd
	}
	n.count++
}

func (n *node[K, V, A, AP]) pushBack(item K, value V, nd *node[K, V, A, AP]) {
	n.keys[n.count] = item
	n.values[n.count] = value
	if !n.leaf {
		n.children[n.count+1] = nd
	}
	n.count++
}

func (n *node[K, V, A, AP]) pushFront(item K, value V, nd *node[K, V, A, AP]) {
	if !n.leaf {
		copy(n.children[1:n.count+2], n.children[:n.count+1])
		n.children[0] = nd
	}
	copy(n.keys[1:n.count+1], n.keys[:n.count])
	copy(n.values[1:n.count+1], n.values[:n.count])
	n.keys[0] = item
	n.values[0] = value
	n.count++
}

// removeAt removes a value at a given index, pulling all subsequent values
// back.
func (n *node[K, V, A, AP]) removeAt(index int) (K, V, *node[K, V, A, AP]) {
	var child *node[K, V, A, AP]
	if !n.leaf {
		child = n.children[index+1]
		copy(n.children[index+1:n.count], n.children[index+2:n.count+1])
		n.children[n.count] = nil
	}
	n.count--
	outK := n.keys[index]
	outV := n.values[index]
	copy(n.keys[index:n.count], n.keys[index+1:n.count+1])
	copy(n.values[index:n.count], n.values[index+1:n.count+1])
	var rk K
	var rv V
	n.keys[n.count] = rk
	n.values[n.count] = rv
	return outK, outV, child
}

// popBack removes and returns the last element in the list.
func (n *node[K, V, A, AP]) popBack() (K, V, *node[K, V, A, AP]) {
	n.count--
	outK := n.keys[n.count]
	outV := n.values[n.count]
	var rK K
	var rV V
	n.keys[n.count] = rK
	n.values[n.count] = rV
	if n.leaf {
		return outK, outV, nil
	}
	child := n.children[n.count+1]
	n.children[n.count+1] = nil
	return outK, outV, child
}

// popFront removes and returns the first element in the list.
func (n *node[K, V, A, AP]) popFront() (K, V, *node[K, V, A, AP]) {
	n.count--
	var child *node[K, V, A, AP]
	if !n.leaf {
		child = n.children[0]
		copy(n.children[:n.count+1], n.children[1:n.count+2])
		n.children[n.count+1] = nil
	}
	outK := n.keys[0]
	outV := n.values[0]
	copy(n.keys[:n.count], n.keys[1:n.count+1])
	copy(n.values[:n.count], n.values[1:n.count+1])
	var rK K
	var rV V
	n.keys[n.count] = rK
	n.values[n.count] = rV
	return outK, outV, child
}

// find returns the index where the given item should be inserted into this
// list. 'found' is true if the item already exists in the list at the given
// index.
func (n *node[K, V, A, AP]) find(cmp func(K, K) int, item K) (index int, found bool) {
	// Logic copied from sort.Search. Inlining this gave
	// an 11% speedup on BenchmarkBTreeDeleteInsert.
	i, j := 0, int(n.count)
	for i < j {
		h := int(uint(i+j) >> 1) // avoid overflow when computing h
		// i ≤ h < j
		c := cmp(item, n.keys[h])
		if c < 0 {
			j = h
		} else if c > 0 {
			i = h + 1
		} else {
			return h, true
		}
	}
	return i, false
}

// split splits the given node at the given index. The current node shrinks,
// and this function returns the item that existed at that index and a new
// node containing all keys/children after it.
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
func (n *node[K, V, A, AP]) split(i int) (K, V, *node[K, V, A, AP]) {
	outK := n.keys[i]
	outV := n.values[i]
	var next *node[K, V, A, AP]
	if n.leaf {
		next = newLeafNode[K, V, A, AP]()
	} else {
		next = newNode[K, V, A, AP]()
	}
	next.count = n.count - int16(i+1)
	copy(next.keys[:], n.keys[i+1:n.count])
	copy(next.values[:], n.values[i+1:n.count])
	var rK K
	var rV V
	for j := int16(i); j < n.count; j++ {
		n.keys[j] = rK
		n.values[j] = rV
	}
	if !n.leaf {
		copy(next.children[:], n.children[i+1:n.count+1])
		for j := int16(i + 1); j <= n.count; j++ {
			n.children[j] = nil
		}
	}
	n.count = int16(i)
	n.update()
	next.update()
	//AP(&N.N.aug).UpdateOnSplit(next)
	/*
		if N.max.compare(next.max) != 0 && N.max.compare(upperBound(out)) != 0 {
			// If upper bound wasn't from new node or item
			// at index i, it must still be from old node.
		} else {
			N.max = N.findUpperBound()
		}
	*/
	return outK, outV, next
}

func (n *node[K, V, A, AP]) update() {
	AP(&n.aug).Update(n)
}

// insert inserts an item into the suAugBTree rooted at this node, making sure no
// nodes in the suAugBTree exceed MaxEntries keys. Returns true if an existing item
// was replaced and false if an item was inserted. Also returns whether the
// node's upper bound changes.
func (n *node[K, V, A, AP]) insert(cmp func(K, K) int, item K, value V) (replacedK K, replacedV V, replaced, newBound bool) {
	i, found := n.find(cmp, item)
	if found {
		replacedV = n.values[i]
		replacedK = n.keys[i]
		n.keys[i] = item
		n.values[i] = value
		return replacedK, replacedV, true, false
	}
	if n.leaf {
		n.insertAt(i, item, value, nil)
		return replacedK, replacedV, false, AP(&n.aug).UpdateOnInsert(item, n, nil)
	}
	if n.children[i].count >= MaxEntries {
		splitLK, splitLV, splitNode := mut(&n.children[i]).split(MaxEntries / 2)
		n.insertAt(i, splitLK, splitLV, splitNode)
		if c := cmp(item, n.keys[i]); c < 0 {
			// no change, we want first split node
		} else if c > 0 {
			i++ // we want second split node
		} else {
			// TODO(ajwerner): add something to the augmentation api to
			// deal with replacement.
			replacedV = n.values[i]
			replacedK = n.keys[i]
			n.keys[i] = item
			n.values[i] = value
			return replacedK, replacedV, true, false
		}
	}
	replacedK, replacedV, replaced, newBound = mut(&n.children[i]).insert(cmp, item, value)
	if newBound {
		newBound = AP(&n.aug).UpdateOnInsert(item, n, nil)
	}
	return replacedK, replacedV, replaced, newBound
}

// removeMax removes and returns the maximum item from the suAugBTree rooted at
// this node.
func (n *node[K, V, A, AP]) removeMax() (K, V) {
	if n.leaf {
		n.count--
		outK := n.keys[n.count]
		outV := n.values[n.count]
		var rK K
		var rV V
		n.keys[n.count] = rK
		n.values[n.count] = rV
		AP(&n.aug).UpdateOnRemoval(outK, n, nil)
		return outK, outV
	}
	// Recurse into max child.
	i := int(n.count)
	if n.children[i].count <= MinEntries {
		// Child not large enough to remove from.
		n.rebalanceOrMerge(i)
		return n.removeMax() // redo
	}
	child := mut(&n.children[i])
	outK, outV := child.removeMax()
	AP(&n.aug).UpdateOnRemoval(outK, n, nil)
	return outK, outV
}

// rebalanceOrMerge grows child 'i' to ensure it has sufficient room to remove
// an item from it while keeping it at or above MinItems.
func (n *node[K, V, A, AP]) rebalanceOrMerge(i int) {
	switch {
	case i > 0 && n.children[i-1].count > MinEntries:
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
		xLaK, xLaV, grandChild := left.popBack()
		yLaK, yLaV := n.keys[i-1], n.values[i-1]
		child.pushFront(yLaK, yLaV, grandChild)
		n.keys[i-1], n.values[i-1] = xLaK, xLaV

		AP(&left.aug).UpdateOnRemoval(xLaK, left, grandChild)
		AP(&child.aug).UpdateOnInsert(yLaK, child, grandChild)

	case i < int(n.count) && n.children[i+1].count > MinEntries:
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
		xLaK, xLaV, grandChild := right.popFront()
		yLaK, yLaV := n.keys[i], n.values[i]
		child.pushBack(yLaK, yLaV, grandChild)
		n.keys[i], n.values[i] = xLaK, xLaV

		AP(&right.aug).UpdateOnRemoval(xLaK, right, grandChild)
		AP(&child.aug).UpdateOnInsert(yLaK, child, grandChild)

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
		mergeLaK, mergeLaV, mergeChild := n.removeAt(i)
		child.keys[child.count] = mergeLaK
		child.values[child.count] = mergeLaV
		copy(child.keys[child.count+1:], mergeChild.keys[:mergeChild.count])
		copy(child.values[child.count+1:], mergeChild.values[:mergeChild.count])
		if !child.leaf {
			copy(child.children[child.count+1:], mergeChild.children[:mergeChild.count+1])
		}
		child.count += mergeChild.count + 1

		AP(&child.aug).UpdateOnInsert(mergeLaK, child, mergeChild)
		mergeChild.decRef(false /* recursive */)
	}
}

// remove removes an item from the suAugBTree rooted at this node. Returns the item
// that was removed or nil if no matching item was found. Also returns whether
// the node's upper bound changes.
func (n *node[K, V, A, AP]) remove(cmp func(K, K) int, item K) (outK K, outV V, found, newBound bool) {
	i, found := n.find(cmp, item)
	if n.leaf {
		if found {
			outK, outV, _ = n.removeAt(i)
			return outK, outV, true, AP(&n.aug).UpdateOnRemoval(outK, n, nil)
		}
		var rK K
		var rV V
		return rK, rV, false, false
	}
	if n.children[i].count <= MinEntries {
		// Child not large enough to remove from.
		n.rebalanceOrMerge(i)
		return n.remove(cmp, item) // redo
	}
	child := mut(&n.children[i])
	if found {
		// Replace the item being removed with the max item in our left child.
		outK = n.keys[i]
		outV = n.values[i]
		n.keys[i], n.values[i] = child.removeMax()
		return outK, outV, true, AP(&n.aug).UpdateOnRemoval(outK, n, nil)
	}
	// Latch is not in this node and child is large enough to remove from.
	outK, outV, found, newBound = child.remove(cmp, item)
	if newBound {
		newBound = AP(&n.aug).UpdateOnRemoval(outK, n, nil)
	}
	return outK, outV, found, newBound
}

func (n *node[K, V, A, AP]) writeString(b *strings.Builder) {
	if n.leaf {
		for i := int16(0); i < n.count; i++ {
			if i != 0 {
				b.WriteString(",")
			}
			fmt.Fprintf(b, "%v:%v", n.keys[i], n.values[i])
		}
		return
	}
	for i := int16(0); i <= n.count; i++ {
		b.WriteString("(")
		n.children[i].writeString(b)
		b.WriteString(")")
		if i < n.count {
			fmt.Fprintf(b, "%v:%v", n.keys[i], n.values[i])
		}
	}
}
