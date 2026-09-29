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

import "sync"

// FreeList recycles nodes between trees. Implementations must be safe for
// concurrent use if the trees sharing them may be written concurrently.
//
// Nodes handed to Put have been cleared; nodes returned from Get may have
// backing storage of any capacity, which the tree grows as needed.
type FreeList[K, V, A any] interface {

	// Get returns a node for reuse, or nil if none is available.
	Get() *Node[K, V, A]

	// Put offers a node for reuse. It returns false if the free list did
	// not retain the node.
	Put(*Node[K, V, A]) bool
}

// NewFreeList returns a FreeList that retains up to size nodes, guarded by
// a mutex. It is the free list New uses by default, with
// DefaultFreeListSize.
func NewFreeList[K, V, A any](size int) FreeList[K, V, A] {
	return &boundedFreeList[K, V, A]{size: size}
}

type boundedFreeList[K, V, A any] struct {
	mu    sync.Mutex
	size  int
	nodes []*Node[K, V, A] // allocated on the first Put
}

func (f *boundedFreeList[K, V, A]) Get() *Node[K, V, A] {
	f.mu.Lock()
	i := len(f.nodes) - 1
	if i < 0 {
		f.mu.Unlock()
		return nil
	}
	n := f.nodes[i]
	f.nodes[i] = nil
	f.nodes = f.nodes[:i]
	f.mu.Unlock()
	return n
}

func (f *boundedFreeList[K, V, A]) Put(n *Node[K, V, A]) bool {
	f.mu.Lock()
	ok := len(f.nodes) < f.size
	if ok {
		if f.nodes == nil {
			f.nodes = make([]*Node[K, V, A], 0, f.size)
		}
		f.nodes = append(f.nodes, n)
	}
	f.mu.Unlock()
	return ok
}

var syncPools sync.Map

// SyncPoolFreeList returns a FreeList backed by a sync.Pool shared by every
// tree with the same type parameters in the process. It is unbounded and
// drained by the garbage collector, which suits workloads that build and
// drop many trees at once.
func SyncPoolFreeList[K, V, A any]() FreeList[K, V, A] {
	var key *Node[K, V, A]
	if v, ok := syncPools.Load(key); ok {
		return v.(*syncPoolFreeList[K, V, A])
	}
	v, _ := syncPools.LoadOrStore(key, &syncPoolFreeList[K, V, A]{})
	return v.(*syncPoolFreeList[K, V, A])
}

type syncPoolFreeList[K, V, A any] struct {
	pool sync.Pool
}

func (f *syncPoolFreeList[K, V, A]) Get() *Node[K, V, A] {
	n, _ := f.pool.Get().(*Node[K, V, A])
	return n
}

func (f *syncPoolFreeList[K, V, A]) Put(n *Node[K, V, A]) bool {
	f.pool.Put(n)
	return true
}
