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

// Iterator is responsible for search and traversal within a Map. Every
// positioning method reports whether the Iterator is now at an entry, so
// that
//
//	for ok := it.SeekGE(lo); ok && it.Compare(it.Key(), hi) < 0; ok = it.Next() {
//
// visits a key range. It holds the path from the root to its position, so
// it must not be copied once positioned; take a fresh one from
// Map.Iterator instead. Reset, First, Last and the Seek methods position it
// from scratch and are safe after any mutation of the Map; the other
// methods are not.
type Iterator[K, V, A any] struct {
	r *Map[K, V, A]
	iterFrame[K, V, A]
	s iterStack[K, V, A]
}

func (i *Iterator[K, V, A]) lowLevel() *LowLevelIterator[K, V, A] {
	return (*LowLevelIterator[K, V, A])(i)
}

// Compare compares two keys using the same comparison function as the map.
func (i *Iterator[K, V, A]) Compare(a, b K) int {
	return i.r.cfg.cmp(a, b)
}

// Reset marks the iterator as invalid and clears any state.
func (i *Iterator[K, V, A]) Reset() {
	i.node = i.r.root
	i.pos = -1
	i.s.reset()
}

// SeekGE positions the Iterator at the first key greater than or equal to
// key and reports whether there is one. If there is none the Iterator is
// left past the end.
func (i *Iterator[K, V, A]) SeekGE(key K) bool {
	i.seekGE(key)
	return i.Valid()
}

// SeekExact positions the Iterator at key and reports whether it exists.
// If it does not, the Iterator is at the first greater key, or past the
// end.
func (i *Iterator[K, V, A]) SeekExact(key K) bool {
	return i.seekGE(key)
}

// seekGE positions the Iterator at the first key >= key and reports
// whether that key is equal to it.
func (i *Iterator[K, V, A]) seekGE(key K) (found bool) {
	i.Reset()
	if i.node == nil {
		return false
	}
	ll := i.lowLevel()
	for {
		pos, found := i.node.find(&i.r.cfg, key)
		i.pos = int16(pos)
		if found {
			return true
		}
		if i.node.IsLeaf() {
			if i.pos == i.node.Count() {
				i.Next()
			}
			return false
		}
		ll.Descend()
	}
}

// SeekGT positions the Iterator at the first key greater than key and
// reports whether there is one. If there is none the Iterator is left past
// the end.
func (i *Iterator[K, V, A]) SeekGT(key K) bool {
	if i.seekGE(key) {
		return i.Next()
	}
	return i.Valid()
}

// SeekLE positions the Iterator at the last key less than or equal to key
// and reports whether there is one. If there is none the Iterator is left
// before the beginning.
func (i *Iterator[K, V, A]) SeekLE(key K) bool {
	if !i.seekGE(key) {
		return i.Prev()
	}
	return true
}

// SeekLT positions the Iterator at the last key less than key and reports
// whether there is one. If there is none the Iterator is left before the
// beginning.
func (i *Iterator[K, V, A]) SeekLT(key K) bool {
	i.Reset()
	if i.node == nil {
		return false
	}
	ll := i.lowLevel()
	for {
		pos, found := i.node.find(&i.r.cfg, key)
		i.pos = int16(pos)
		if found || i.node.IsLeaf() {
			return i.Prev()
		}
		ll.Descend()
	}
}

// First positions the Iterator at the first key and reports whether there
// is one.
func (i *Iterator[K, V, A]) First() bool {
	i.Reset()
	if i.node == nil {
		return false
	}
	ll := i.lowLevel()
	i.pos = 0 // Reset leaves -1; Descend follows children[pos]
	for !i.node.IsLeaf() {
		ll.Descend()
	}
	i.pos = 0
	return i.Valid()
}

// Last positions the Iterator at the last key and reports whether there is
// one.
func (i *Iterator[K, V, A]) Last() bool {
	i.Reset()
	if i.node == nil {
		return false
	}
	ll := i.lowLevel()
	for !i.node.IsLeaf() {
		i.pos = i.node.Count()
		ll.Descend()
	}
	i.pos = i.node.Count() - 1
	return i.Valid()
}

// Next positions the Iterator at the key following its current position
// and reports whether there is one. If the Iterator is before the first
// key (as after Reset), Next positions it at the first key. If it is
// already past the last key, Next leaves it there.
func (i *Iterator[K, V, A]) Next() bool {
	if i.node == nil {
		return false
	}
	ll := i.lowLevel()
	if i.node.IsLeaf() {
		if i.pos < i.node.Count() {
			i.pos++
			if i.pos < i.node.Count() {
				return true
			}
		}
		i.settle()
		return i.pos < i.node.Count()
	}
	if i.pos >= i.node.Count() {
		// Past the end; stay there.
		return false
	}
	i.pos++
	ll.Descend()
	for !i.node.IsLeaf() {
		i.pos = 0
		ll.Descend()
	}
	i.pos = 0
	return true
}

// settle moves an iterator whose position ran off the end of a leaf up to
// the separator that follows the leaf, or leaves it past the end at the
// root when there is none.
func (i *Iterator[K, V, A]) settle() {
	ll := i.lowLevel()
	for i.pos >= i.node.Count() && i.s.len() > 0 {
		ll.Ascend()
	}
}

// Prev positions the Iterator at the key preceding its current position
// and reports whether there is one. If the Iterator is past the last key,
// Prev positions it at the last key. If it is already before the first key
// (as after Reset), Prev leaves it there.
func (i *Iterator[K, V, A]) Prev() bool {
	if i.node == nil {
		return false
	}
	ll := i.lowLevel()
	if i.node.IsLeaf() {
		if i.pos >= 0 {
			i.pos--
		}
		for i.pos < 0 && i.s.len() > 0 {
			ll.Ascend()
			i.pos--
		}
		return i.pos >= 0
	}
	if i.pos < 0 {
		// Before the beginning; stay there.
		return false
	}
	ll.Descend()
	for !i.node.IsLeaf() {
		i.pos = i.node.Count()
		ll.Descend()
	}
	i.pos = i.node.Count() - 1
	return true
}

// Valid returns whether the Iterator is positioned at a valid position.
func (i *Iterator[K, V, A]) Valid() bool {
	return i.node != nil && i.pos >= 0 && i.pos < i.node.Count()
}

// Key returns the key at the Iterator's current position. It is illegal
// to call Key if the Iterator is not valid.
func (i *Iterator[K, V, A]) Key() K {
	return i.node.keys[i.pos]
}

// Value returns the value at the Iterator's current position. It is illegal
// to call Value if the Iterator is not valid.
func (i *Iterator[K, V, A]) Value() V {
	return i.node.values[i.pos]
}
