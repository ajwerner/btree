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

// TODO(ajwerner): Tighten up this interface and make this more generally
// efficient for updates where possible.
type Node[K, A any] interface {
	Count() int16
	IsLeaf() bool
	GetKey(i int16) K
	GetChild(i int16) A
}

// Aug is a data structure which augments a node of the tree. It is updated
// when the structure or contents of the substract rooted at the current node
// changes.
type Aug[K, Aux, A any] interface {
	*A
	Update(*Config[K, Aux], Node[K, *A], UpdateMeta[K, A]) (changed bool)
}

type Config[K, Aux any] struct {
	Config  Aux
	Compare func(K, K) int
}

type Action int

const (
	Default Action = iota
	Split
	Removal
	Insertion
)

type UpdateMeta[K, A any] struct {

	// Action indicates the semantics of the below fields. If Default,
	// no fields will be populated.
	Action        Action
	ModifiedOther *A
	RelevantKey   K
}
