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

import "sync"

type nodePool[K, V, Aux, A any, AP Aug[K, Aux, A]] struct {
	interiorNodePool, leafNodePool sync.Pool
}

var syncPoolMap sync.Map

func getNodePool[K, V, Aux, A any, AP Aug[K, Aux, A]]() *nodePool[K, V, Aux, A, AP] {
	var nilNode *node[K, V, Aux, A, AP]
	v, ok := syncPoolMap.Load(nilNode)
	if !ok {
		v, _ = syncPoolMap.LoadOrStore(nilNode, newNodePool[K, V, Aux, A, AP]())
	}
	return v.(*nodePool[K, V, Aux, A, AP])

}

func newNodePool[K, V, Aux, A any, AP Aug[K, Aux, A]]() *nodePool[K, V, Aux, A, AP] {
	np := nodePool[K, V, Aux, A, AP]{}
	np.leafNodePool = sync.Pool{
		New: func() interface{} {
			return new(node[K, V, Aux, A, AP])
		},
	}
	np.interiorNodePool = sync.Pool{
		New: func() interface{} {
			n := new(interiorNode[K, V, Aux, A, AP])
			n.node.children = &n.children
			return &n.node
		},
	}
	return &np
}

func (np *nodePool[K, V, Aux, A, AP]) getInteriorNode() *node[K, V, Aux, A, AP] {
	n := np.interiorNodePool.Get().(*node[K, V, Aux, A, AP])
	n.ref = 1
	return n
}

func (np *nodePool[K, V, Aux, A, AP]) getLeafNode() *node[K, V, Aux, A, AP] {
	n := np.leafNodePool.Get().(*node[K, V, Aux, A, AP])
	n.ref = 1
	return n
}

func (np *nodePool[K, V, Aux, A, AP]) putInteriorNode(n *node[K, V, Aux, A, AP]) {
	children := n.children
	*children = [MaxEntries + 1]*node[K, V, Aux, A, AP]{}
	*n = node[K, V, Aux, A, AP]{}
	n.children = children
	np.interiorNodePool.Put(n)
}

func (np *nodePool[K, V, Aux, A, AP]) putLeafNode(n *node[K, V, Aux, A, AP]) {
	*n = node[K, V, Aux, A, AP]{}
	np.leafNodePool.Put(n)
}
