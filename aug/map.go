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
	"cmp"
	"strings"
)

// Map is an augmented copy-on-write B-tree map from K to V. Each node
// carries an augmentation of type A maintained by the Updater given to New.
// See the package documentation for the ownership and concurrency rules.
type Map[K, V, A any] struct {
	root   *Node[K, V, A]
	length int
	cfg    config[K, V, A]
}

// New constructs a Map with the provided comparison function and Updater.
// The Updater may be nil, in which case A is never touched. See WithDegree
// and WithFreeList for the options.
func New[K, V, A any](cmp func(K, K) int, up Updater[K, V, A], opts ...Option) *Map[K, V, A] {
	return &Map[K, V, A]{cfg: makeConfig(cmp, up, opts)}
}

// NewOrdered constructs a Map keyed by a type that supports <, ordered
// that way. It searches nodes with < directly rather than through a
// comparison function, which is faster. Floating-point NaN keys are not
// supported: they compare as equal to everything, so use New with
// cmp.Compare for keys that may be NaN.
func NewOrdered[K cmp.Ordered, V, A any](up Updater[K, V, A], opts ...Option) *Map[K, V, A] {
	opts = append(opts, withFind(findOrdered[K, V, A]))
	return &Map[K, V, A]{cfg: makeConfig(cmp.Compare[K], up, opts)}
}

// Degree returns the degree of the tree.
func (t *Map[K, V, A]) Degree() int {
	return t.cfg.minEntries + 1
}

// Clear removes all items from the Map, releasing its references to its
// nodes. Nodes no longer referenced by any other Map are returned to the
// free list. Failure to call Clear before dropping a Map is safe but
// prevents nodes from being recycled, and keeps nodes shared with other
// Maps in the copy-on-write state.
func (t *Map[K, V, A]) Clear() {
	if t.root != nil {
		t.root.decRef(&t.cfg, true /* recursive */)
		t.root = nil
	}
	t.length = 0
}

// Clone clones the Map, lazily. It does so in constant time. The clone
// shares the receiver's free list.
func (t *Map[K, V, A]) Clone() *Map[K, V, A] {
	if t.root != nil {
		// Incrementing the reference count on the root node is sufficient to
		// ensure that no node in the cloned tree can be mutated by an actor
		// holding a reference to the original tree and vice versa. This
		// property is upheld because the root node in the receiver and
		// the returned Map will both necessarily have a reference count of at
		// least 2 when this method returns. All tree mutations recursively
		// acquire mutable node references (see mut) as they traverse down the
		// tree. The act of acquiring a mutable node reference performs a clone
		// if a node's reference count is greater than one. Cloning a node (see
		// clone) increases the reference count on each of its children,
		// ensuring that they have a reference count of at least 2. This, in
		// turn, ensures that any of the child nodes that are modified will also
		// be copied-on-write, recursively ensuring the immutability property
		// over the entire tree.
		t.root.incRef()
	}
	c := *t
	return &c
}

// Delete removes the item with the given key from the tree, returning it.
func (t *Map[K, V, A]) Delete(k K) (removedK K, v V, found bool) {
	if t.root == nil || len(t.root.entries) == 0 {
		return removedK, v, false
	}
	if removedK, v, found, _ = mut(&t.cfg, &t.root).remove(&t.cfg, k); found {
		t.length--
	}
	if len(t.root.entries) == 0 {
		old := t.root
		if t.root.IsLeaf() {
			t.root = nil
		} else {
			t.root = t.root.children[0]
			// The reference to children[0] transfers to t.root.
			old.children[0] = nil
			old.children = old.children[:0]
		}
		old.decRef(&t.cfg, false /* recursive */)
	}
	return removedK, v, found
}

// Upsert adds the given item to the tree. If an item in the tree already equals
// the given one, it is replaced with the new item.
func (t *Map[K, V, A]) Upsert(item K, value V) (replacedK K, replacedV V, replaced bool) {
	if t.root == nil {
		t.root = t.cfg.getLeaf()
	} else if len(t.root.entries) >= t.cfg.maxEntries {
		splitK, splitV, splitNode := mut(&t.cfg, &t.root).split(&t.cfg, t.cfg.maxEntries/2)
		newRoot := t.cfg.getInterior()
		newRoot.entries = append(newRoot.entries, entry[K, V]{v: splitV, k: splitK})
		newRoot.children = append(newRoot.children, t.root, splitNode)
		newRoot.update(&t.cfg.Config)
		t.root = newRoot
	}
	replacedK, replacedV, replaced, _ = mut(&t.cfg, &t.root).insert(&t.cfg, item, value)
	if !replaced {
		t.length++
	}
	return replacedK, replacedV, replaced
}

// Iterator returns a new Iterator object. It is not safe to continue using an
// Iterator after modifications are made to the tree. If modifications are made,
// create a new Iterator.
func (t *Map[K, V, A]) Iterator() Iterator[K, V, A] {
	it := Iterator[K, V, A]{r: t}
	it.Reset()
	return it
}

// Height returns the height of the tree.
func (t *Map[K, V, A]) Height() int {
	if t.root == nil {
		return 0
	}
	h := 1
	n := t.root
	for !n.IsLeaf() {
		n = n.children[0]
		h++
	}
	return h
}

// Len returns the number of items currently in the tree.
func (t *Map[K, V, A]) Len() int {
	return t.length
}

// Get returns the value associated with the requested key, if it exists.
func (t *Map[K, V, A]) Get(k K) (v V, ok bool) {
	n := t.root
	for n != nil {
		i, found := n.find(&t.cfg, k)
		if found {
			return n.entries[i].v, true
		}
		if n.IsLeaf() {
			break
		}
		n = n.children[i]
	}
	return v, false
}

// Compare compares two keys using the Map's comparison function.
func (t *Map[K, V, A]) Compare(a, b K) int {
	return t.cfg.cmp(a, b)
}

// String returns a string description of the tree. The format is
// similar to the https://en.wikipedia.org/wiki/Newick_format.
func (t *Map[K, V, A]) String() string {
	if t.length == 0 {
		return ";"
	}
	var b strings.Builder
	t.root.writeString(&b)
	return b.String()
}
