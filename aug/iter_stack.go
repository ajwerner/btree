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

// iterStack represents a stack of (node, pos) tuples, which captures
// iteration state as an Iterator descends a Map.
type iterStack[K, V, A any] struct {
	a    iterStackArr[K, V, A]
	aLen int16 // -1 when using s
	s    []iterFrame[K, V, A]
}

const iterStackDepth = 10

// Used to avoid allocations for stacks below a certain size.
type iterStackArr[K, V, A any] [iterStackDepth]iterFrame[K, V, A]

type iterFrame[K, V, A any] struct {
	node *Node[K, V, A]
	pos  int16
}

func (is *iterStack[K, V, A]) push(f iterFrame[K, V, A]) {
	if is.aLen == -1 {
		is.s = append(is.s, f)
	} else if int(is.aLen) == len(is.a) {
		// Spill to the heap, reusing a slice kept from an earlier spill.
		n := int(is.aLen) + 1
		if cap(is.s) >= n {
			is.s = is.s[:n]
		} else {
			is.s = make([]iterFrame[K, V, A], n, 2*n)
		}
		copy(is.s, is.a[:])
		is.s[n-1] = f
		clear(is.a[:])
		is.aLen = -1
	} else {
		is.a[is.aLen] = f
		is.aLen++
	}
}

// pop removes and returns the top frame, clearing its slot so that the
// stack holds no stale node pointers.
func (is *iterStack[K, V, A]) pop() iterFrame[K, V, A] {
	if is.aLen == -1 {
		last := len(is.s) - 1
		f := is.s[last]
		is.s[last] = iterFrame[K, V, A]{}
		is.s = is.s[:last]
		return f
	}
	is.aLen--
	f := is.a[is.aLen]
	is.a[is.aLen] = iterFrame[K, V, A]{}
	return f
}

// at returns the frame at depth d, where 0 is the frame closest to the root.
func (is *iterStack[K, V, A]) at(d int) iterFrame[K, V, A] {
	if is.aLen == -1 {
		return is.s[d]
	}
	return is.a[d]
}

// setNode replaces the node of the frame at depth d.
func (is *iterStack[K, V, A]) setNode(d int, n *Node[K, V, A]) {
	if is.aLen == -1 {
		is.s[d].node = n
		return
	}
	is.a[d].node = n
}

func (is *iterStack[K, V, A]) len() int {
	if is.aLen == -1 {
		return len(is.s)
	}
	return int(is.aLen)
}

// reset empties the stack and returns to the inline array, keeping a
// spilled slice for reuse but holding no node pointers.
func (is *iterStack[K, V, A]) reset() {
	if is.aLen == -1 {
		clear(is.s)
		is.s = is.s[:0]
	} else {
		clear(is.a[:is.aLen])
	}
	is.aLen = 0
}
