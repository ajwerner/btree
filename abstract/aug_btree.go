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

package abstract

import (
	"strings"
)

// TODO(ajwerner): It'd be amazing to find a way to make this not a single
// compile-time constant.

const (
	Degree     = 2
	MaxEntries = 2*Degree - 1
	MinEntries = Degree - 1
)

// TODO(ajwerner): Probably we want comparison to occur on pointers to the
// objects rather than the objects themselves, at least in some cases. For very
// large objects, probably it's better to just store the objects as pointers
// in the btree itself and to use a sync.Pool to pool allocations. For very
// small objects, directly calling less on the object is probably ideal. The
// question is mid-sized objects.

// Map is an implementation of an augmented B-Tree.
//
// Write operations are not safe for concurrent mutation by multiple
// goroutines, but Read operations are.
type Map[K, V, Aux, A any, AP Aug[K, Aux, A]] struct {
	root   *node[K, V, Aux, A, AP]
	length int
	td     treeData[K, Aux]
}

type treeData[K, Aux any] struct {
	aux Aux
	cmp func(K, K) int
}

func MakeMap[K, V, Aux, A any, AP Aug[K, Aux, A]](aux Aux, cmp func(K, K) int) Map[K, V, Aux, A, AP] {
	return Map[K, V, Aux, A, AP]{
		td: treeData[K, Aux]{
			cmp: cmp,
			aux: aux,
		},
	}
}

// Reset removes all items from the AugBTree. In doing so, it allows memory
// held by the AugBTree to be recycled. Failure to call this method before
// letting a AugBTree be GCed is safe in that it won't cause a memory leak,
// but it will prevent AugBTree nodes from being efficiently re-used.
func (t *Map[K, V, Aux, A, AP]) Reset() {
	if t.root != nil {
		t.root.decRef(true /* recursive */)
		t.root = nil
	}
	t.length = 0
}

// Clone clones the AugBTree, lazily. It does so in constant time.
func (t *Map[K, V, Aux, A, AP]) Clone() *Map[K, V, Aux, A, AP] {
	c := *t
	if c.root != nil {
		// Incrementing the reference count on the root node is sufficient to
		// ensure that no node in the cloned tree can be mutated by an actor
		// holding a reference to the original tree and vice versa. This
		// property is upheld because the root node in the receiver AugBTree and
		// the returned AugBTree will both necessarily have a reference count of at
		// least 2 when this method returns. All tree mutations recursively
		// acquire mutable node references (see mut) as they traverse down the
		// tree. The act of acquiring a mutable node reference performs a clone
		// if a node's reference count is greater than one. Cloning a node (see
		// clone) increases the reference count on each of its children,
		// ensuring that they have a reference count of at least 2. This, in
		// turn, ensures that any of the child nodes that are modified will also
		// be copied-on-write, recursively ensuring the immutability property
		// over the entire tree.
		c.root.incRef()
	}
	return &c
}

// Delete removes an item equal to the passed in item from the tree.
func (t *Map[K, V, Aux, A, AP]) Delete(k K) (removedK K, v V, found bool) {
	if t.root == nil || t.root.count == 0 {
		return removedK, v, false
	}
	if removedK, v, found, _ = mut(&t.root).remove(&t.td, k); found {
		t.length--
	}
	if t.root.count == 0 {
		old := t.root
		if t.root.leaf {
			t.root = nil
		} else {
			t.root = t.root.children[0]
		}
		old.decRef(false /* recursive */)
	}
	return removedK, v, found
}

// Upsert adds the given item to the tree. If an item in the tree already equals
// the given one, it is replaced with the new item.
func (t *Map[K, V, Aux, A, AP]) Upsert(item K, value V) (replacedK K, replacedV V, replaced bool) {
	if t.root == nil {
		t.root = newLeafNode[K, V, Aux, A, AP]()
	} else if t.root.count >= MaxEntries {
		splitLaK, splitLaV, splitNode := mut(&t.root).split(&t.td, MaxEntries/2)
		newRoot := newNode[K, V, Aux, A, AP]()
		newRoot.count = 1
		newRoot.keys[0] = splitLaK
		newRoot.values[0] = splitLaV
		newRoot.children[0] = t.root
		t.root.update(t.td.aux)
		splitNode.update(t.td.aux)
		newRoot.children[1] = splitNode
		newRoot.update(t.td.aux)
		t.root = newRoot
	}
	replacedK, replacedV, replaced, _ = mut(&t.root).insert(&t.td, item, value)
	if !replaced {
		t.length++
	}
	return replacedK, replacedV, replaced
}

// MakeIter returns a new Iterator object. It is not safe to continue using an
// Iterator after modifications are made to the tree. If modifications are made,
// create a new Iterator.
func (t *Map[K, V, Aux, A, AP]) MakeIter() Iterator[K, V, Aux, A, AP] {
	it := Iterator[K, V, Aux, A, AP]{r: t}
	it.Reset()
	return it
}

// Height returns the height of the tree.
func (t *Map[K, V, Aux, A, AP]) Height() int {
	if t.root == nil {
		return 0
	}
	h := 1
	n := t.root
	for !n.leaf {
		n = n.children[0]
		h++
	}
	return h
}

// Len returns the number of items currently in the tree.
func (t *Map[K, V, Aux, A, AP]) Len() int {
	return t.length
}

// String returns a string description of the tree. The format is
// similar to the https://en.wikipedia.org/wiki/Newick_format.
func (t *Map[K, V, Aux, A, AP]) String() string {
	if t.length == 0 {
		return ";"
	}
	var b strings.Builder
	t.root.writeString(&b)
	return b.String()
}
