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

import "reflect"

// Monoid describes an augmentation that is the combination of per-entry
// contributions: the augmentation of a subtree is Combine folded over Of of
// every entry in it. The zero value of A must be the identity of Combine,
// and Combine must be associative and commutative: the Updater folds
// contributions in whatever order the tree operations produce them (a new
// entry is combined onto the right of a node's aggregate wherever it
// sits). Counts, sums, minima, maxima and bounds all qualify; an
// order-dependent aggregate such as concatenation needs a hand-written
// Updater that recomputes nodes.
//
// A Map whose Updater comes from MonoidUpdater supports Prefix, Aggregate,
// Total, Iterator.Prefix and Iterator.SeekWhere.
type Monoid[K, V, A any] interface {

	// Of returns the contribution of a single entry.
	Of(k K, v V) A

	// Combine combines the aggregates of two spans.
	Combine(a, b A) A
}

// Group is a Monoid whose contributions can be removed again. It lets the
// Updater maintain augmentations in O(1) on removal, split and replacement
// instead of recomputing the node.
type Group[K, V, A any] interface {
	Monoid[K, V, A]

	// Uncombine returns the aggregate a without the contribution b, i.e.
	// Uncombine(Combine(x, b), b) == x for every x.
	Uncombine(a, b A) A
}

// Equaler may optionally be implemented by a Monoid to let the Updater
// stop propagating an update up the tree when a node's augmentation did not
// change. Without it every update propagates to the root.
type Equaler[A any] interface {
	Equal(a, b A) bool
}

// Folder may optionally be implemented by a Monoid to fold spans of a node
// in one call instead of one Of and Combine call per entry or child. Prefix,
// Aggregate and Iterator.Prefix use it when present.
type Folder[K, V, A any] interface {

	// FoldEntries returns the combination of Of over the node's entries at
	// indexes [lo, hi).
	FoldEntries(n *Node[K, V, A], lo, hi int) A

	// FoldChildren returns the combination of the augmentations of the
	// node's children at indexes [lo, hi). It is only called on non-leaf
	// nodes.
	FoldChildren(n *Node[K, V, A], lo, hi int) A
}

// MonoidUpdater returns an Updater maintaining the augmentation described
// by m. If m is also a Group, removals, splits and replacements are applied
// incrementally; otherwise they recompute the node from its entries and
// children in O(degree).
func MonoidUpdater[K, V, A any](m Monoid[K, V, A]) Updater[K, V, A] {
	if c, ok := any(m).(Count[K, V]); ok {
		// Count is common enough to deserve an Updater with no interface
		// calls; the conversion below is a no-op since A is int.
		return any(&countUpdater[K, V]{m: c}).(Updater[K, V, A])
	}
	u := &monoidUpdater[K, V, A]{m: m}
	u.g, _ = m.(Group[K, V, A])
	u.eq, _ = m.(Equaler[A])
	return u
}

// countUpdater is MonoidUpdater specialised for Count. It must agree with
// monoidUpdater given Count; the aggregate tests exercise the generic path
// with Count wrapped in PairOf and the orderstat tests exercise this one.
type countUpdater[K, V any] struct {
	m Count[K, V]
}

func (u *countUpdater[K, V]) monoid() Monoid[K, V, int] { return u.m }

// Equal compares counts.
func (u *countUpdater[K, V]) Equal(a, b int) bool { return a == b }

func (u *countUpdater[K, V]) Update(n *Node[K, V, int], md UpdateInfo[K, V, int]) bool {
	a := n.Aug()
	switch md.Action {
	case Insertion:
		*a++
		if md.ModifiedOther != nil {
			*a += *md.ModifiedOther
		}
		return true
	case Removal:
		*a--
		if md.ModifiedOther != nil {
			*a -= *md.ModifiedOther
		}
		return true
	case Split:
		*a -= 1 + *md.ModifiedOther
		return true
	case Replacement:
		return false
	default:
		prev := *a
		count := len(n.keys)
		for _, c := range n.children {
			count += c.aug
		}
		*a = count
		return prev != count
	}
}

type monoidUpdater[K, V, A any] struct {
	m  Monoid[K, V, A]
	g  Group[K, V, A]
	eq Equaler[A]
}

func (u *monoidUpdater[K, V, A]) monoid() Monoid[K, V, A] { return u.m }

// Equal compares aggregates with the Monoid's Equaler, or reflect.DeepEqual
// without one. Verify uses it.
func (u *monoidUpdater[K, V, A]) Equal(a, b A) bool {
	if u.eq != nil {
		return u.eq.Equal(a, b)
	}
	return reflect.DeepEqual(a, b)
}

func (u *monoidUpdater[K, V, A]) Update(n *Node[K, V, A], md UpdateInfo[K, V, A]) bool {
	a := n.Aug()
	prev := *a
	switch md.Action {
	case Insertion:
		*a = u.m.Combine(*a, u.m.Of(md.RelevantKey, md.RelevantValue))
		if md.ModifiedOther != nil {
			*a = u.m.Combine(*a, *md.ModifiedOther)
		}
	case Removal:
		if u.g == nil {
			*a = u.recompute(n)
			break
		}
		*a = u.g.Uncombine(*a, u.m.Of(md.RelevantKey, md.RelevantValue))
		if md.ModifiedOther != nil {
			*a = u.g.Uncombine(*a, *md.ModifiedOther)
		}
	case Split:
		if u.g == nil {
			*a = u.recompute(n)
			break
		}
		*a = u.g.Uncombine(*a, u.m.Of(md.RelevantKey, md.RelevantValue))
		*a = u.g.Uncombine(*a, *md.ModifiedOther)
	case Replacement:
		if u.g == nil {
			*a = u.recompute(n)
			break
		}
		*a = u.g.Uncombine(*a, u.m.Of(md.PrevKey, md.PrevValue))
		*a = u.m.Combine(*a, u.m.Of(md.RelevantKey, md.RelevantValue))
	default:
		*a = u.recompute(n)
	}
	if u.eq != nil {
		return !u.eq.Equal(prev, *a)
	}
	return true
}

func (u *monoidUpdater[K, V, A]) recompute(n *Node[K, V, A]) A {
	var acc A
	leaf := n.IsLeaf()
	for i, k := range n.keys {
		if !leaf {
			acc = u.m.Combine(acc, n.children[i].aug)
		}
		acc = u.m.Combine(acc, u.m.Of(k, n.values[i]))
	}
	if !leaf {
		acc = u.m.Combine(acc, n.children[len(n.keys)].aug)
	}
	return acc
}

// Count is a Group counting entries; its aggregate over a span is the
// number of entries in it. It is the augmentation of the orderstat package.
type Count[K, V any] struct{}

func (Count[K, V]) Of(K, V) int            { return 1 }
func (Count[K, V]) Combine(a, b int) int   { return a + b }
func (Count[K, V]) Uncombine(a, b int) int { return a - b }
func (Count[K, V]) Equal(a, b int) bool    { return a == b }

func (Count[K, V]) FoldEntries(_ *Node[K, V, int], lo, hi int) int { return hi - lo }

func (Count[K, V]) FoldChildren(n *Node[K, V, int], lo, hi int) int {
	var acc int
	for _, c := range n.children[lo:hi] {
		acc += c.aug
	}
	return acc
}

// Integer is the set of types Sum can add. Floating-point sums are not
// associative, let alone invertible, so they do not qualify as a Monoid;
// sum floats as fixed-point integers, or implement Updater directly and
// recompute nodes.
type Integer interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

// Sum returns a Group summing f over entries.
func Sum[K, V any, N Integer](f func(K, V) N) Group[K, V, N] {
	return sum[K, V, N]{f: f}
}

type sum[K, V any, N Integer] struct {
	f func(K, V) N
}

func (s sum[K, V, N]) Of(k K, v V) N      { return s.f(k, v) }
func (s sum[K, V, N]) Combine(a, b N) N   { return a + b }
func (s sum[K, V, N]) Uncombine(a, b N) N { return a - b }
func (s sum[K, V, N]) Equal(a, b N) bool  { return a == b }

func (s sum[K, V, N]) FoldEntries(n *Node[K, V, N], lo, hi int) N {
	var acc N
	for i := lo; i < hi; i++ {
		acc += s.f(n.keys[i], n.values[i])
	}
	return acc
}

func (s sum[K, V, N]) FoldChildren(n *Node[K, V, N], lo, hi int) N {
	var acc N
	for _, c := range n.children[lo:hi] {
		acc += c.aug
	}
	return acc
}

// Pair is the aggregate of two augmentations maintained side by side.
type Pair[A, B any] struct {
	A A
	B B
}

// PairOf returns a Monoid maintaining a and b together. The result is a
// Group if both a and b are Groups, and an Equaler if both are Equalers.
func PairOf[K, V, A, B any](a Monoid[K, V, A], b Monoid[K, V, B]) Monoid[K, V, Pair[A, B]] {
	p := pair[K, V, A, B]{a: a, b: b}
	ga, aOK := a.(Group[K, V, A])
	gb, bOK := b.(Group[K, V, B])
	ea, aEq := a.(Equaler[A])
	eb, bEq := b.(Equaler[B])
	switch {
	case aOK && bOK && aEq && bEq:
		return pairGroupEq[K, V, A, B]{pairGroup[K, V, A, B]{p, ga, gb}, ea, eb}
	case aOK && bOK:
		return pairGroup[K, V, A, B]{p, ga, gb}
	case aEq && bEq:
		return pairEq[K, V, A, B]{p, ea, eb}
	default:
		return p
	}
}

type pair[K, V, A, B any] struct {
	a Monoid[K, V, A]
	b Monoid[K, V, B]
}

func (p pair[K, V, A, B]) Of(k K, v V) Pair[A, B] {
	return Pair[A, B]{A: p.a.Of(k, v), B: p.b.Of(k, v)}
}

func (p pair[K, V, A, B]) Combine(x, y Pair[A, B]) Pair[A, B] {
	return Pair[A, B]{A: p.a.Combine(x.A, y.A), B: p.b.Combine(x.B, y.B)}
}

type pairGroup[K, V, A, B any] struct {
	pair[K, V, A, B]
	ga Group[K, V, A]
	gb Group[K, V, B]
}

func (p pairGroup[K, V, A, B]) Uncombine(x, y Pair[A, B]) Pair[A, B] {
	return Pair[A, B]{A: p.ga.Uncombine(x.A, y.A), B: p.gb.Uncombine(x.B, y.B)}
}

type pairEq[K, V, A, B any] struct {
	pair[K, V, A, B]
	ea Equaler[A]
	eb Equaler[B]
}

func (p pairEq[K, V, A, B]) Equal(x, y Pair[A, B]) bool {
	return p.ea.Equal(x.A, y.A) && p.eb.Equal(x.B, y.B)
}

type pairGroupEq[K, V, A, B any] struct {
	pairGroup[K, V, A, B]
	ea Equaler[A]
	eb Equaler[B]
}

func (p pairGroupEq[K, V, A, B]) Equal(x, y Pair[A, B]) bool {
	return p.ea.Equal(x.A, y.A) && p.eb.Equal(x.B, y.B)
}
