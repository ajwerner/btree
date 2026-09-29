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

package interval

import "github.com/ajwerner/btree/aug"

type subtreeBound[K any] struct {
	keyBound[K]
	set bool // false until the first entry; the zero K is not a bound
}

type updater[I, K, V any] struct {
	key, end func(I) K
	cmp      func(K, K) int
	hasEnd   func(I) bool
}

func (u *updater[I, K, V]) Update(
	n *aug.Node[I, V, subtreeBound[K]],
	md aug.UpdateInfo[I, V, subtreeBound[K]],
) (updated bool) {
	a := n.Aug()
	switch md.Action {
	case aug.Insertion:
		up := u.upperBound(md.RelevantKey)
		if child := md.ModifiedOther; child != nil {
			if up.compare(u.cmp, child.keyBound) < 0 {
				up = child.keyBound
			}
		}
		if !a.set || a.compare(u.cmp, up) < 0 {
			a.keyBound, a.set = up, true
			return true
		}
		return false
	case aug.Removal:
		up := u.upperBound(md.RelevantKey)
		if child := md.ModifiedOther; child != nil {
			if up.compare(u.cmp, child.keyBound) < 0 {
				up = child.keyBound
			}
		}
		if a.set && a.compare(u.cmp, up) == 0 {
			a.keyBound, a.set = u.findUpperBound(n)
			return !a.set || a.compare(u.cmp, up) != 0
		}
		return false
	case aug.Split:
		if a.set && a.compare(u.cmp, md.ModifiedOther.keyBound) != 0 &&
			a.compare(u.cmp, u.upperBound(md.RelevantKey)) != 0 {
			return false
		}
		fallthrough
	case aug.Default, aug.Replacement:
		prev, prevSet := a.keyBound, a.set
		a.keyBound, a.set = u.findUpperBound(n)
		return prevSet != a.set || (a.set && a.compare(u.cmp, prev) != 0)
	default:
		panic("interval: unknown action")
	}
}

type keyBound[K any] struct {
	k         K
	inclusive bool
}

func (up *updater[I, K, V]) upperBound(interval I) keyBound[K] {
	if !up.hasEnd(interval) {
		return keyBound[K]{k: up.key(interval), inclusive: true}
	}
	return keyBound[K]{k: up.end(interval)}
}

// findUpperBound recomputes the bound of n's subtree; ok is false for an
// empty node.
func (up *updater[I, K, V]) findUpperBound(n *aug.Node[I, V, subtreeBound[K]]) (max keyBound[K], ok bool) {
	var setMax bool
	for i, cnt := int16(0), n.Count(); i < cnt; i++ {
		ub := up.upperBound(n.Key(i))
		if !setMax || max.compare(up.cmp, ub) < 0 {
			setMax = true
			max = ub
		}
	}
	if !n.IsLeaf() {
		for i, cnt := int16(0), n.Count(); i <= cnt; i++ {
			child := n.ChildAug(i)
			if !child.set {
				continue
			}
			if !setMax || max.compare(up.cmp, child.keyBound) < 0 {
				setMax = true
				max = child.keyBound
			}
		}
	}
	return max, setMax
}

func (b keyBound[K]) compare(cmp func(K, K) int, o keyBound[K]) int {
	c := cmp(b.k, o.k)
	if c != 0 {
		return c
	}
	if b.inclusive == o.inclusive {
		return 0
	}
	if b.inclusive {
		return 1
	}
	return -1
}

func (b keyBound[K]) contains(cmp func(K, K) int, o K) bool {
	c := cmp(o, b.k)
	if c == 0 {
		return b.inclusive
	}
	return c < 0
}
