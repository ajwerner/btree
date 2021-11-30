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

import "github.com/ajwerner/btree/internal/abstract"

type aug[K any] struct {
	keyBound[K]
}

type updater[I, K any] struct {
	key, end func(I) K
	cmp      Cmp[K]
}

func (u *updater[I, K]) Update(
	n abstract.Node[I, aug[K]],
	md abstract.UpdateMeta[I, aug[K]],
) (updated bool) {
	a := n.GetA()
	switch md.Action {
	case abstract.Insertion:
		up := u.upperBound(md.RelevantKey)
		if child := md.ModifiedOther; child != nil {
			if up.compare(u.cmp, child.keyBound) < 0 {
				up = child.keyBound
			}
		}
		if a.compare(u.cmp, up) < 0 {
			a.keyBound = up
			return true
		}
		return false
	case abstract.Removal:
		up := u.upperBound(md.RelevantKey)
		if child := md.ModifiedOther; child != nil {
			if up.compare(u.cmp, child.keyBound) < 0 {
				up = child.keyBound
			}
		}
		if a.compare(u.cmp, up) == 0 {
			a.keyBound = u.findUpperBound(n)
			return a.compare(u.cmp, up) != 0
		}
		return false
	case abstract.Split:
		if a.compare(u.cmp, md.ModifiedOther.keyBound) != 0 &&
			a.compare(u.cmp, u.upperBound(md.RelevantKey)) != 0 {
			return false
		}
		fallthrough
	case abstract.Default:
		prev := a.keyBound
		a.keyBound = u.findUpperBound(n)
		return a.compare(u.cmp, prev) != 0
	default:
		panic("")
	}
}

type keyBound[K any] struct {
	k         K
	inclusive bool
}

func (up *updater[I, K]) upperBound(interval I) keyBound[K] {
	k, end := up.key(interval), up.end(interval)
	// if the key is equal to the end, or somehow, greater, then we'll say that
	// the interval is represented only by the point. There should be an
	// invariant to disallow the end being greater than the start.
	//
	// TODO(ajwerner): Panic on insert if the interval invariant is not upheld.
	// TODO(ajwerner): Consider a different API for single-point intervals like
	// a boolean method to indicate that there is no end key.
	if isZero(up.cmp, end) {
		return keyBound[K]{k: k, inclusive: true}
	}
	return keyBound[K]{k: end}
}

func isZero[K any](cmp Cmp[K], k K) bool {
	var z K
	return cmp(k, z) == 0
}

func (up *updater[I, K]) findUpperBound(n abstract.Node[I, aug[K]]) keyBound[K] {
	var max keyBound[K]
	var setMax bool
	for i, cnt := int16(0), n.Count(); i < cnt; i++ {
		ub := up.upperBound(n.GetKey(i))
		if !setMax || max.compare(up.cmp, ub) < 0 {
			setMax = true
			max = ub
		}
	}
	if !n.IsLeaf() {
		for i, cnt := int16(0), n.Count(); i <= cnt; i++ {
			ub := n.GetChild(i).keyBound
			if max.compare(up.cmp, ub) < 0 {
				max = ub
			}
		}
	}
	return max
}

func (b keyBound[K]) compare(cmp Cmp[K], o keyBound[K]) int {
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

func (b keyBound[K]) contains(cmp Cmp[K], o K) bool {
	c := cmp(o, b.k)
	if c == 0 {
		return b.inclusive
	}
	return c < 0
}
