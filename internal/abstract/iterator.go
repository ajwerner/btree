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

// Iterator is responsible for search and traversal within a AugBTree.
type Iterator[K, V, Aux, A any, AP Aug[K, Aux, A]] struct {
	r *Map[K, V, Aux, A, AP]
	iterFrame[K, V, Aux, A, AP]
	s iterStack[K, V, Aux, A, AP]
}

func (i *Iterator[K, V, Aux, A, AP]) lowLevel() *LowLevelIterator[K, V, Aux, A, AP] {
	return (*LowLevelIterator[K, V, Aux, A, AP])(i)
}

func (i *Iterator[K, V, Aux, A, AP]) Reset() {
	i.node = i.r.root
	i.pos = -1
	i.s.reset()
}

// SeekGE seeks to the first key greater-than or equal to the provided
// key.
func (i *Iterator[K, V, Aux, A, AP]) SeekGE(key K) {
	i.Reset()
	if i.node == nil {
		return
	}
	ll := i.lowLevel()
	for {
		pos, found := i.find(i.r.cfg.cmp, key)
		i.pos = int16(pos)
		if found {
			return
		}
		if i.leaf {
			if i.pos == i.count {
				i.Next()
			}
			return
		}
		ll.Descend()
	}
}

// SeekLT seeks to the first key less-than the provided key.
func (i *Iterator[K, V, Aux, A, AP]) SeekLT(key K) {
	i.Reset()
	if i.node == nil {
		return
	}
	ll := i.lowLevel()
	for {
		pos, found := i.find(i.r.cfg.cmp, key)
		i.pos = int16(pos)
		if found || i.leaf {
			i.Prev()
			return
		}
		ll.Descend()
	}
}

// First seeks to the first key in the AugBTree.
func (i *Iterator[K, V, Aux, A, AP]) First() {
	i.Reset()
	i.pos = 0
	if i.node == nil {
		return
	}
	ll := i.lowLevel()
	for !i.leaf {
		ll.Descend()
	}
	i.pos = 0
}

// Last seeks to the last key in the AugBTree.
func (i *Iterator[K, V, Aux, A, AP]) Last() {
	i.Reset()
	if i.node == nil {
		return
	}
	ll := i.lowLevel()
	for !i.leaf {
		i.pos = i.count
		ll.Descend()
	}
	i.pos = i.count - 1
}

// Next positions the Iterator to the key immediately following
// its current position.
func (i *Iterator[K, V, Aux, A, AP]) Next() {
	if i.node == nil {
		return
	}
	ll := i.lowLevel()
	if i.leaf {
		i.pos++
		if i.pos < i.count {
			return
		}
		for i.s.len() > 0 && i.pos >= i.count {
			ll.Ascend()
		}
		return
	}
	i.pos++
	ll.Descend()
	for !i.leaf {
		i.pos = 0
		ll.Descend()
	}
	i.pos = 0
}

// Prev positions the Iterator to the key immediately preceding
// its current position.
func (i *Iterator[K, V, Aux, A, AP]) Prev() {
	if i.node == nil {
		return
	}
	ll := i.lowLevel()
	if i.leaf {
		i.pos--
		if i.pos >= 0 {
			return
		}
		for i.s.len() > 0 && i.pos < 0 {
			ll.Ascend()
			i.pos--
		}
		return
	}

	ll.Descend()
	for !i.leaf {
		i.pos = i.count
		ll.Descend()
	}
	i.pos = i.count - 1
}

// Valid returns whether the Iterator is positioned at a valid position.
func (i *Iterator[K, V, Aux, A, AP]) Valid() bool {
	return i.pos >= 0 && i.pos < i.count
}

// Key returns the key at the Iterator's current position. It is illegal
// to call Key if the Iterator is not valid.
func (i *Iterator[K, V, Aux, A, AP]) Key() K {
	return i.keys[i.pos]
}

// Value returns the value at the Iterator's current position. It is illegal
// to call Value if the Iterator is not valid.
func (i *Iterator[K, V, Aux, A, AP]) Value() V {
	return i.values[i.pos]
}
