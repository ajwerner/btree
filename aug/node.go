// Copyright 2018 The Cockroach Authors.
// Copyright 2021 Andrew Werner.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package aug

import (
	"fmt"
	"strings"
	"sync/atomic"
)

// entry holds one key and its value. The value comes first so that a
// zero-size V adds no trailing padding.
type entry[K, V any] struct {
	v V
	k K
}

// Node represents a node in the tree. Its methods expose read-only access
// to augmentation code; the tree itself is manipulated through Map.
type Node[K, V, A any] struct {
	ref      int32
	aug      A
	entries  []entry[K, V]
	children []*Node[K, V, A] // empty for leaves
}

// Aug returns a pointer to the node's augmentation.
func (n *Node[K, V, A]) Aug() *A {
	return &n.aug
}

// IsLeaf returns true if the node is a leaf.
func (n *Node[K, V, A]) IsLeaf() bool {
	return len(n.children) == 0
}

// Count returns the number of entries in the node.
func (n *Node[K, V, A]) Count() int16 {
	return int16(len(n.entries))
}

// Key returns the key at position i, which must be in [0, Count()).
func (n *Node[K, V, A]) Key(i int16) K {
	return n.entries[i].k
}

// Value returns the value at position i, which must be in [0, Count()).
func (n *Node[K, V, A]) Value(i int16) V {
	return n.entries[i].v
}

// ChildAug returns the augmentation of the child at position i, which must
// be in [0, Count()] for a non-leaf node. It returns nil for a leaf.
func (n *Node[K, V, A]) ChildAug(i int16) *A {
	if int(i) < len(n.children) {
		return &n.children[i].aug
	}
	return nil
}

func (c *config[K, V, A]) getNode() *Node[K, V, A] {
	n := c.fl.Get()
	if n == nil {
		n = &Node[K, V, A]{}
	}
	if cap(n.entries) < c.maxEntries {
		n.entries = make([]entry[K, V], 0, c.maxEntries)
	}
	n.ref = 1
	return n
}

func (c *config[K, V, A]) getLeaf() *Node[K, V, A] {
	return c.getNode()
}

func (c *config[K, V, A]) getInterior() *Node[K, V, A] {
	n := c.getNode()
	if cap(n.children) < c.maxEntries+1 {
		n.children = make([]*Node[K, V, A], 0, c.maxEntries+1)
	}
	return n
}

// putNode clears a node and offers it to the free list. The node must not
// hold references to any children.
func (c *config[K, V, A]) putNode(n *Node[K, V, A]) {
	clear(n.entries)
	n.entries = n.entries[:0]
	clear(n.children)
	n.children = n.children[:0]
	var zero A
	n.aug = zero
	n.ref = 0
	c.fl.Put(n)
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
func mut[K, V, A any](
	c *config[K, V, A],
	n **Node[K, V, A],
) *Node[K, V, A] {
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
	cl := (*n).clone(c)
	(*n).decRef(c, true /* recursive */)
	*n = cl
	return *n
}

// incRef acquires a reference to the node.
func (n *Node[K, V, A]) incRef() {
	atomic.AddInt32(&n.ref, 1)
}

// decRef releases a reference to the node. If requested, the method
// will recurse into child nodes and decrease their refcounts as well.
func (n *Node[K, V, A]) decRef(
	c *config[K, V, A], recursive bool,
) {
	if atomic.AddInt32(&n.ref, -1) > 0 {
		// Other references remain. Can't free.
		return
	}
	if recursive {
		for _, child := range n.children {
			child.decRef(c, true /* recursive */)
		}
	}
	c.putNode(n)
}

// clone creates a clone of the receiver with a single reference count.
func (n *Node[K, V, A]) clone(c *config[K, V, A]) *Node[K, V, A] {
	var out *Node[K, V, A]
	if n.IsLeaf() {
		out = c.getLeaf()
	} else {
		out = c.getInterior()
	}
	// NB: copy field-by-field without touching n.ref to avoid
	// triggering the race detector and looking like a data race.
	out.aug = n.aug
	out.entries = append(out.entries[:0], n.entries...)
	if !n.IsLeaf() {
		out.children = append(out.children[:0], n.children...)
		for _, child := range out.children {
			child.incRef()
		}
	}
	return out
}

// insertAt inserts the entry at index and, for a non-leaf node, the child
// at index+1.
func (n *Node[K, V, A]) insertAt(index int, k K, v V, child *Node[K, V, A]) {
	n.entries = append(n.entries, entry[K, V]{})
	copy(n.entries[index+1:], n.entries[index:])
	n.entries[index] = entry[K, V]{v: v, k: k}
	if !n.IsLeaf() {
		n.children = append(n.children, nil)
		copy(n.children[index+2:], n.children[index+1:])
		n.children[index+1] = child
	}
}

func (n *Node[K, V, A]) pushBack(k K, v V, child *Node[K, V, A]) {
	n.entries = append(n.entries, entry[K, V]{v: v, k: k})
	if !n.IsLeaf() {
		n.children = append(n.children, child)
	}
}

func (n *Node[K, V, A]) pushFront(k K, v V, child *Node[K, V, A]) {
	n.entries = append(n.entries, entry[K, V]{})
	copy(n.entries[1:], n.entries)
	n.entries[0] = entry[K, V]{v: v, k: k}
	if !n.IsLeaf() {
		n.children = append(n.children, nil)
		copy(n.children[1:], n.children)
		n.children[0] = child
	}
}

// removeAt removes the entry at index and, for a non-leaf node, the child
// at index+1, returning both.
func (n *Node[K, V, A]) removeAt(index int) (K, V, *Node[K, V, A]) {
	var child *Node[K, V, A]
	if !n.IsLeaf() {
		child = n.children[index+1]
		last := len(n.children) - 1
		copy(n.children[index+1:], n.children[index+2:])
		n.children[last] = nil
		n.children = n.children[:last]
	}
	out := n.entries[index]
	last := len(n.entries) - 1
	copy(n.entries[index:], n.entries[index+1:])
	n.entries[last] = entry[K, V]{}
	n.entries = n.entries[:last]
	return out.k, out.v, child
}

// popBack removes and returns the last entry and, for a non-leaf node, the
// last child.
func (n *Node[K, V, A]) popBack() (K, V, *Node[K, V, A]) {
	last := len(n.entries) - 1
	out := n.entries[last]
	n.entries[last] = entry[K, V]{}
	n.entries = n.entries[:last]
	if n.IsLeaf() {
		return out.k, out.v, nil
	}
	lastChild := len(n.children) - 1
	child := n.children[lastChild]
	n.children[lastChild] = nil
	n.children = n.children[:lastChild]
	return out.k, out.v, child
}

// popFront removes and returns the first entry and, for a non-leaf node,
// the first child.
func (n *Node[K, V, A]) popFront() (K, V, *Node[K, V, A]) {
	var child *Node[K, V, A]
	if !n.IsLeaf() {
		child = n.children[0]
		last := len(n.children) - 1
		copy(n.children, n.children[1:])
		n.children[last] = nil
		n.children = n.children[:last]
	}
	out := n.entries[0]
	last := len(n.entries) - 1
	copy(n.entries, n.entries[1:])
	n.entries[last] = entry[K, V]{}
	n.entries = n.entries[:last]
	return out.k, out.v, child
}

// find returns the index where the given item should be inserted into this
// list. 'found' is true if the item already exists in the list at the given
// index.
func (n *Node[K, V, A]) find(cmp func(K, K) int, item K) (index int, found bool) {
	// Logic copied from sort.Search. Inlining this gave
	// an 11% speedup on BenchmarkBTreeDeleteInsert.
	i, j := 0, len(n.entries)
	for i < j {
		h := int(uint(i+j) >> 1) // avoid overflow when computing h
		// i ≤ h < j
		c := cmp(item, n.entries[h].k)
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
//	+-----------+
//	|   x y z   |
//	+--/-/-\-\--+
//
// After:
//
//	         +-----------+
//	         |     y     |
//	         +----/-\----+
//	             /   \
//	            v     v
//	+-----------+     +-----------+
//	|         x |     | z         |
//	+-----------+     +-----------+
func (n *Node[K, V, A]) split(c *config[K, V, A], i int) (K, V, *Node[K, V, A]) {
	out := n.entries[i]
	var next *Node[K, V, A]
	if n.IsLeaf() {
		next = c.getLeaf()
	} else {
		next = c.getInterior()
	}
	next.entries = append(next.entries[:0], n.entries[i+1:]...)
	clear(n.entries[i:])
	n.entries = n.entries[:i]
	if !n.IsLeaf() {
		next.children = append(next.children[:0], n.children[i+1:]...)
		clear(n.children[i+1:])
		n.children = n.children[:i+1]
	}
	next.update(&c.Config)
	n.updateOn(&c.Config, Split, out.k, out.v, next)
	return out.k, out.v, next
}

func (n *Node[K, V, A]) update(cfg *Config[K, V, A]) bool {
	if cfg.Updater == nil {
		return false
	}
	return cfg.Updater.Update(n, UpdateInfo[K, V, A]{})
}

func (n *Node[K, V, A]) updateOn(cfg *Config[K, V, A], action Action, k K, v V, affected *Node[K, V, A]) bool {
	if cfg.Updater == nil {
		return false
	}
	var a *A
	if affected != nil {
		a = &affected.aug
	}
	return cfg.Updater.Update(n, UpdateInfo[K, V, A]{
		Action:        action,
		RelevantKey:   k,
		RelevantValue: v,
		ModifiedOther: a,
	})
}

func (n *Node[K, V, A]) updateOnReplace(cfg *Config[K, V, A], k K, v V, prevK K, prevV V) bool {
	if cfg.Updater == nil {
		return false
	}
	return cfg.Updater.Update(n, UpdateInfo[K, V, A]{
		Action:        Replacement,
		RelevantKey:   k,
		RelevantValue: v,
		PrevKey:       prevK,
		PrevValue:     prevV,
	})
}

// insert inserts an item into the subtree rooted at this node, making sure no
// nodes in the subtree exceed maxEntries keys. Returns true if an existing item
// was replaced and false if an item was inserted. Also returns whether the
// node's augmentation changed.
func (n *Node[K, V, A]) insert(c *config[K, V, A], item K, value V) (replacedK K, replacedV V, replaced, changed bool) {
	i, found := n.find(c.cmp, item)
	if found {
		return n.replaceAt(c, i, item, value)
	}
	if n.IsLeaf() {
		n.insertAt(i, item, value, nil)
		return replacedK, replacedV, false, n.updateOn(&c.Config, Insertion, item, value, nil)
	}
	if len(n.children[i].entries) >= c.maxEntries {
		splitK, splitV, splitNode := mut(c, &n.children[i]).split(c, c.maxEntries/2)
		n.insertAt(i, splitK, splitV, splitNode)
		if cmp := c.cmp(item, n.entries[i].k); cmp < 0 {
			// no change, we want first split node
		} else if cmp > 0 {
			i++ // we want second split node
		} else {
			return n.replaceAt(c, i, item, value)
		}
	}
	replacedK, replacedV, replaced, changed =
		mut(c, &n.children[i]).insert(c, item, value)
	if changed {
		if replaced {
			changed = n.updateOnReplace(&c.Config, item, value, replacedK, replacedV)
		} else {
			changed = n.updateOn(&c.Config, Insertion, item, value, nil)
		}
	}
	return replacedK, replacedV, replaced, changed
}

// replaceAt replaces the entry at index i, which has a key equal to item.
func (n *Node[K, V, A]) replaceAt(c *config[K, V, A], i int, item K, value V) (replacedK K, replacedV V, replaced, changed bool) {
	e := &n.entries[i]
	replacedK, replacedV = e.k, e.v
	e.k, e.v = item, value
	return replacedK, replacedV, true, n.updateOnReplace(&c.Config, item, value, replacedK, replacedV)
}

// removeMax removes and returns the maximum item from the subtree rooted at
// this node.
func (n *Node[K, V, A]) removeMax(c *config[K, V, A]) (K, V) {
	if n.IsLeaf() {
		outK, outV, _ := n.popBack()
		n.updateOn(&c.Config, Removal, outK, outV, nil)
		return outK, outV
	}
	// Recurse into max child.
	i := len(n.entries)
	if len(n.children[i].entries) <= c.minEntries {
		// Child not large enough to remove from.
		n.rebalanceOrMerge(c, i)
		return n.removeMax(c) // redo
	}
	child := mut(c, &n.children[i])
	outK, outV := child.removeMax(c)
	n.updateOn(&c.Config, Removal, outK, outV, nil)
	return outK, outV
}

// rebalanceOrMerge grows child 'i' to ensure it has sufficient room to remove
// an item from it while keeping it at or above minEntries.
func (n *Node[K, V, A]) rebalanceOrMerge(c *config[K, V, A], i int) {
	switch {
	case i > 0 && len(n.children[i-1].entries) > c.minEntries:
		// Rebalance from left sibling.
		//
		//           +-----------+
		//           |     y     |
		//           +----/-\----+
		//               /   \
		//              v     v
		//  +-----------+     +-----------+
		//  |         x |     |           |
		//  +----------\+     +-----------+
		//              \
		//               v
		//               a
		//
		// After:
		//
		//           +-----------+
		//           |     x     |
		//           +----/-\----+
		//               /   \
		//              v     v
		//  +-----------+     +-----------+
		//  |           |     | y         |
		//  +-----------+     +/----------+
		//                    /
		//                   v
		//                   a
		//
		left := mut(c, &n.children[i-1])
		child := mut(c, &n.children[i])
		xK, xV, grandChild := left.popBack()
		y := &n.entries[i-1]
		yK, yV := y.k, y.v
		child.pushFront(yK, yV, grandChild)
		y.k, y.v = xK, xV
		left.updateOn(&c.Config, Removal, xK, xV, grandChild)
		child.updateOn(&c.Config, Insertion, yK, yV, grandChild)

	case i < len(n.entries) && len(n.children[i+1].entries) > c.minEntries:
		// Rebalance from right sibling.
		//
		//           +-----------+
		//           |     y     |
		//           +----/-\----+
		//               /   \
		//              v     v
		//  +-----------+     +-----------+
		//  |           |     | x         |
		//  +-----------+     +/----------+
		//                    /
		//                   v
		//                   a
		//
		// After:
		//
		//           +-----------+
		//           |     x     |
		//           +----/-\----+
		//               /   \
		//              v     v
		//  +-----------+     +-----------+
		//  |         y |     |           |
		//  +----------\+     +-----------+
		//              \
		//               v
		//               a
		//
		right := mut(c, &n.children[i+1])
		child := mut(c, &n.children[i])
		xK, xV, grandChild := right.popFront()
		y := &n.entries[i]
		yK, yV := y.k, y.v
		child.pushBack(yK, yV, grandChild)
		y.k, y.v = xK, xV
		right.updateOn(&c.Config, Removal, xK, xV, grandChild)
		child.updateOn(&c.Config, Insertion, yK, yV, grandChild)

	default:
		// Merge with either the left or right sibling.
		//
		//           +-----------+
		//           |   u y v   |
		//           +----/-\----+
		//               /   \
		//              v     v
		//  +-----------+     +-----------+
		//  |         x |     | z         |
		//  +-----------+     +-----------+
		//
		// After:
		//
		//           +-----------+
		//           |    u v    |
		//           +-----|-----+
		//                 |
		//                 v
		//           +-----------+
		//           |   x y z   |
		//           +-----------+
		//
		if i >= len(n.entries) {
			i = len(n.entries) - 1
		}
		child := mut(c, &n.children[i])
		mergeK, mergeV, mergeChild := n.removeAt(i)
		child.entries = append(child.entries, entry[K, V]{v: mergeV, k: mergeK})
		child.entries = append(child.entries, mergeChild.entries...)
		if !child.IsLeaf() {
			child.children = append(child.children, mergeChild.children...)
		}
		child.updateOn(&c.Config, Insertion, mergeK, mergeV, mergeChild)
		if atomic.LoadInt32(&mergeChild.ref) == 1 {
			// We own mergeChild exclusively, so its references to its
			// children transfer to child. Drop them from mergeChild so
			// that freeing it does not release them.
			clear(mergeChild.children)
			mergeChild.children = mergeChild.children[:0]
			mergeChild.decRef(c, false /* recursive */)
		} else {
			// mergeChild is shared: child needs its own references to the
			// grandchildren, and mergeChild keeps its own. We may be
			// racing with another decRef, so release recursively.
			for _, grandChild := range mergeChild.children {
				grandChild.incRef()
			}
			mergeChild.decRef(c, true /* recursive */)
		}
	}
}

// remove removes an item from the subtree rooted at this node. Returns the item
// that was removed or nil if no matching item was found. Also returns whether
// the node's augmentation changed.
func (n *Node[K, V, A]) remove(
	c *config[K, V, A], item K,
) (outK K, outV V, found, changed bool) {
	i, found := n.find(c.cmp, item)
	if n.IsLeaf() {
		if found {
			outK, outV, _ = n.removeAt(i)
			return outK, outV, true, n.updateOn(&c.Config, Removal, outK, outV, nil)
		}
		return outK, outV, false, false
	}
	if len(n.children[i].entries) <= c.minEntries {
		// Child not large enough to remove from.
		n.rebalanceOrMerge(c, i)
		return n.remove(c, item) // redo
	}
	child := mut(c, &n.children[i])
	if found {
		// Replace the item being removed with the max item in our left child.
		e := &n.entries[i]
		outK, outV = e.k, e.v
		e.k, e.v = child.removeMax(c)
		return outK, outV, true, n.updateOn(&c.Config, Removal, outK, outV, nil)
	}
	// Item is not in this node and child is large enough to remove from.
	outK, outV, found, changed = child.remove(c, item)
	if changed {
		changed = n.updateOn(&c.Config, Removal, outK, outV, nil)
	}
	return outK, outV, found, changed
}

func (n *Node[K, V, A]) writeString(b *strings.Builder) {
	if n.IsLeaf() {
		for i, e := range n.entries {
			if i != 0 {
				b.WriteString(",")
			}
			fmt.Fprintf(b, "%v:%v", e.k, e.v)
		}
		return
	}
	for i := range n.children {
		b.WriteString("(")
		n.children[i].writeString(b)
		b.WriteString(")")
		if i < len(n.entries) {
			fmt.Fprintf(b, "%v:%v", n.entries[i].k, n.entries[i].v)
		}
	}
}
