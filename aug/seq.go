// Copyright 2026 Andrew Werner.
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

import "iter"

// All returns an iterator over every entry in key order. It walks the tree
// directly and is the fastest way to visit every entry; the stateful
// Iterator is for seeking and stepping. The Map must not be written during
// the iteration.
func (t *Map[K, V, A]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		if t.root != nil {
			t.root.ascend(yield)
		}
	}
}

// Backward returns an iterator over every entry in reverse key order.
func (t *Map[K, V, A]) Backward() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		if t.root != nil {
			t.root.descend(yield)
		}
	}
}

// Range returns an iterator over the entries with keys in [lo, hi) in key
// order.
func (t *Map[K, V, A]) Range(lo, hi K) iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		if t.root != nil && t.cfg.cmp(lo, hi) < 0 {
			t.root.ascendRange(&t.cfg, &lo, &hi, yield)
		}
	}
}

// From returns an iterator over the entries with keys greater than or
// equal to lo in key order.
func (t *Map[K, V, A]) From(lo K) iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		if t.root != nil {
			t.root.ascendRange(&t.cfg, &lo, nil, yield)
		}
	}
}

func (n *Node[K, V, A]) ascend(yield func(K, V) bool) bool {
	if n.IsLeaf() {
		for i, k := range n.keys {
			if !yield(k, n.values[i]) {
				return false
			}
		}
		return true
	}
	for i, k := range n.keys {
		if !n.children[i].ascend(yield) {
			return false
		}
		if !yield(k, n.values[i]) {
			return false
		}
	}
	return n.children[len(n.keys)].ascend(yield)
}

func (n *Node[K, V, A]) descend(yield func(K, V) bool) bool {
	if n.IsLeaf() {
		for i := len(n.keys) - 1; i >= 0; i-- {
			if !yield(n.keys[i], n.values[i]) {
				return false
			}
		}
		return true
	}
	if !n.children[len(n.keys)].descend(yield) {
		return false
	}
	for i := len(n.keys) - 1; i >= 0; i-- {
		if !yield(n.keys[i], n.values[i]) {
			return false
		}
		if !n.children[i].descend(yield) {
			return false
		}
	}
	return true
}

// ascendRange yields the entries of the subtree with keys in [lo, hi),
// where a nil bound is unconstrained.
func (n *Node[K, V, A]) ascendRange(c *config[K, V, A], lo, hi *K, yield func(K, V) bool) bool {
	if lo == nil && hi == nil {
		return n.ascend(yield)
	}
	i, j := 0, len(n.keys)
	if lo != nil {
		i, _ = n.find(c, *lo) // first entry >= lo
	}
	if hi != nil {
		j, _ = n.find(c, *hi) // first entry >= hi
	}
	leaf := n.IsLeaf()
	if !leaf && i == j {
		// Both bounds fall within the same child.
		return n.children[i].ascendRange(c, lo, hi, yield)
	}
	if !leaf && !n.children[i].ascendRange(c, lo, nil, yield) {
		return false
	}
	for k := i; k < j; k++ {
		if !yield(n.keys[k], n.values[k]) {
			return false
		}
		if !leaf && k+1 < j && !n.children[k+1].ascend(yield) {
			return false
		}
	}
	if !leaf {
		return n.children[j].ascendRange(c, nil, hi, yield)
	}
	return true
}

// Min returns the entry with the smallest key.
func (t *Map[K, V, A]) Min() (k K, v V, ok bool) {
	n := t.root
	if n == nil {
		return k, v, false
	}
	for !n.IsLeaf() {
		n = n.children[0]
	}
	return n.keys[0], n.values[0], true
}

// Max returns the entry with the largest key.
func (t *Map[K, V, A]) Max() (k K, v V, ok bool) {
	n := t.root
	if n == nil {
		return k, v, false
	}
	for !n.IsLeaf() {
		n = n.children[len(n.keys)]
	}
	last := len(n.keys) - 1
	return n.keys[last], n.values[last], true
}
