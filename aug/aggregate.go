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

func (t *Map[K, V, A]) monoid() CommutativeMonoid[K, V, A] {
	if t.cfg.monoid == nil {
		panic("aug: operation requires an Updater created by MonoidUpdater")
	}
	return t.cfg.monoid
}

// foldEntries combines Of over the node's entries at indexes [lo, hi).
func (c *config[K, V, A]) foldEntries(acc A, n *Node[K, V, A], lo, hi int) A {
	if lo >= hi {
		return acc
	}
	if c.folder != nil {
		return c.monoid.Combine(acc, c.folder.FoldEntries(n, lo, hi))
	}
	for i := lo; i < hi; i++ {
		acc = c.monoid.Combine(acc, c.monoid.Of(n.keys[i], n.values[i]))
	}
	return acc
}

// foldChildren combines the augmentations of the node's children at
// indexes [lo, hi).
func (c *config[K, V, A]) foldChildren(acc A, n *Node[K, V, A], lo, hi int) A {
	if lo >= hi {
		return acc
	}
	if c.folder != nil {
		return c.monoid.Combine(acc, c.folder.FoldChildren(n, lo, hi))
	}
	for _, child := range n.children[lo:hi] {
		acc = c.monoid.Combine(acc, child.aug)
	}
	return acc
}

// foldBefore combines into acc everything in n to the left of pos: the
// children and entries at indexes below pos and, when withChild is set and
// n is not a leaf, the child at pos.
func (c *config[K, V, A]) foldBefore(acc A, n *Node[K, V, A], pos int, withChild bool) A {
	if pos > len(n.keys) {
		pos = len(n.keys)
	}
	if pos < 0 {
		return acc
	}
	if !n.IsLeaf() {
		end := pos
		if withChild {
			end++
		}
		acc = c.foldChildren(acc, n, 0, end)
	}
	return c.foldEntries(acc, n, 0, pos)
}

func (t *Map[K, V, A]) total() A {
	m := t.monoid()
	if t.root == nil {
		return m.Identity()
	}
	return t.root.aug
}

func (t *Map[K, V, A]) prefix(k K) (prefix A, found bool) {
	prefix = t.monoid().Identity()
	n := t.root
	for n != nil {
		i, found := n.find(&t.cfg, k)
		if n.IsLeaf() {
			return t.cfg.foldBefore(prefix, n, i, false), found
		}
		prefix = t.cfg.foldBefore(prefix, n, i, found)
		if found {
			return prefix, true
		}
		n = n.children[i]
	}
	return prefix, false
}

func (t *Map[K, V, A]) aggregateRange(lo, hi K) A {
	m := t.monoid()
	if t.root == nil || t.cfg.cmp(lo, hi) >= 0 {
		return m.Identity()
	}
	return t.aggregate(m, t.root, &lo, &hi)
}

// aggregate folds the entries of the subtree at n with keys in [lo, hi),
// where a nil bound is unconstrained.
func (t *Map[K, V, A]) aggregate(m CommutativeMonoid[K, V, A], n *Node[K, V, A], lo, hi *K) A {
	if lo == nil && hi == nil {
		return n.aug
	}
	i, j := 0, len(n.keys)
	var loFound bool
	if lo != nil {
		i, loFound = n.find(&t.cfg, *lo) // first entry >= lo
	}
	if hi != nil {
		j, _ = n.find(&t.cfg, *hi) // first entry >= hi
	}
	acc := m.Identity()
	if n.IsLeaf() {
		return t.cfg.foldEntries(acc, n, i, j)
	}
	if i == j {
		// Both bounds fall within the same child.
		return t.aggregate(m, n.children[i], lo, hi)
	}
	if !loFound {
		// Otherwise every key in children[i] is below lo.
		acc = t.aggregate(m, n.children[i], lo, nil)
	}
	acc = t.cfg.foldEntries(acc, n, i, j)
	acc = t.cfg.foldChildren(acc, n, i+1, j)
	return m.Combine(acc, t.aggregate(m, n.children[j], nil, hi))
}

func (i *Iterator[K, V, A]) prefix() A {
	c := &i.r.cfg
	p := i.r.monoid().Identity()
	// Ancestor frames contribute what lies left of the child through which
	// the iterator descended; the current node contributes what lies left
	// of its position, including the child at the position, whose entries
	// precede the entry there.
	for d, depth := 0, i.s.len(); d < depth; d++ {
		f := i.s.at(d)
		p = c.foldBefore(p, f.node, int(f.pos), false)
	}
	if i.node != nil {
		p = c.foldBefore(p, i.node, int(i.pos), true)
	}
	return p
}

func (i *Iterator[K, V, A]) seekPrefix(pred func(inclusivePrefix A) bool) (A, bool) {
	m := i.r.monoid()
	i.Reset()
	p := m.Identity()
	if i.node == nil {
		return p, false
	}
	ll := i.lowLevel()
	for {
		n := i.node
		leaf := n.IsLeaf()
		count := len(n.keys)
		pos := 0
		for ; pos <= count; pos++ {
			if !leaf {
				ca := n.children[pos].aug
				if pred(m.Combine(p, ca)) {
					break
				}
				p = m.Combine(p, ca)
			}
			if pos < count {
				oc := m.Of(n.keys[pos], n.values[pos])
				if pred(m.Combine(p, oc)) {
					i.pos = int16(pos)
					return p, true
				}
				p = m.Combine(p, oc)
			}
		}
		if pos > count {
			// The predicate never became true; leave the iterator past the
			// end with the total.
			for i.s.len() > 0 {
				ll.Ascend()
			}
			i.pos = i.node.Count()
			return i.r.root.aug, false
		}
		i.pos = int16(pos)
		ll.Descend()
	}
}
