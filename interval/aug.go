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

import "github.com/ajwerner/btree/new/abstract"

type aug[K any, I Interval[K]] struct {
	keyBound[K]
}

func (a *aug[K, I]) Update(
	n abstract.Node[I, *aug[K, I]],
	md abstract.UpdateMeta[I, CompareFn[K], aug[K, I]],
) (updated bool) {
	cmp := md.Aux
	switch md.Action {
	case abstract.Insertion:
		up := upperBound(md.RelevantKey, cmp)
		if child := md.ModifiedOther; child != nil {
			if up.compare(cmp, child.keyBound) < 0 {
				up = child.keyBound
			}
		}
		if a.compare(cmp, up) < 0 {
			a.keyBound = up
			return true
		}
		return false
	case abstract.Removal:
		up := upperBound(md.RelevantKey, cmp)
		if child := md.ModifiedOther; child != nil {
			if up.compare(cmp, child.keyBound) < 0 {
				up = child.keyBound
			}
		}
		if a.keyBound.compare(cmp, up) != 0 {
			a.keyBound = findUpperBound(n, cmp)
			return a.compare(cmp, up) != 0
		}
		return false
	case abstract.Split:
		if a.compare(cmp, md.ModifiedOther.keyBound) != 0 &&
			a.compare(cmp, upperBound(md.RelevantKey, cmp)) != 0 {
			return false
		}
		fallthrough
	case abstract.Default:
		// Fin
		prev := a.keyBound
		a.keyBound = findUpperBound(n, cmp)
		return a.compare(cmp, prev) != 0
	default:
		panic("")
	}
}

type keyBound[K any] struct {
	k         K
	inclusive bool
}

func upperBound[I Interval[K], K any](interval I, f CompareFn[K]) keyBound[K] {
	k, end := interval.Key(), interval.End()
	// if the key is equal to the end, or somehow, greater, then we'll say that
	// the interval is represented only by the point. There should be an
	// invariant to disallow the end being greater than the start.
	//
	// TODO(ajwerner): Panic on insert if the interval invariant is not upheld.
	// TODO(ajwerner): Consider a different API for single-point intervals like
	// a boolean method to indicate that there is no end key.
	if f(k, end) >= 0 {
		return keyBound[K]{k: k, inclusive: true}
	}
	return keyBound[K]{k: end}
}

func findUpperBound[I Interval[K], K any](n abstract.Node[I, *aug[K, I]], cmp CompareFn[K]) keyBound[K] {
	var max keyBound[K]
	var setMax bool
	for i, cnt := int16(0), n.Count(); i < cnt; i++ {
		up := upperBound(n.GetKey(i), cmp)
		if !setMax || max.compare(cmp, up) < 0 {
			setMax = true
			max = up
		}
	}
	if !n.IsLeaf() {
		for i, cnt := int16(0), n.Count(); i <= cnt; i++ {
			up := n.GetChild(i).keyBound
			if max.compare(cmp, up) < 0 {
				max = up
			}
		}
	}
	return max
}

func (b keyBound[K]) compare(cmp CompareFn[K], o keyBound[K]) int {
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

func (b keyBound[K]) contains(cmp CompareFn[K], o K) bool {
	c := cmp(o, b.k)
	if c == 0 {
		return b.inclusive
	}
	return c < 0
}
