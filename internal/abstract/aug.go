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

// Node represents an abstraction of a node exposed to the
// augmentation and low-level iteration primitives.
type Node[K, A any] interface {

	// IsLeaf returns whether this node is a leaf.
	IsLeaf() bool

	// Count returns the number of keys and entries in this node.
	Count() int16

	// GetKey returns the key at the given position. It may be
	// called with values in [0, Count()) if Count() is greater than 0.
	GetKey(i int16) K

	// GetChild returns the augmentation of the child at the given position. It
	// may be called on non-leaf nodes with values in [0, Count()] if Count()
	// is greater than 0.
	GetChild(i int16) A
}

// Aug is a data structure which augments a node of the tree. It is updated
// when the structure or contents of the subtree rooted at the current node
// changes.
type Aug[K, Aux, A any] interface {
	*A

	// Update is used to update the state of the node augmentation in response
	// to a mutation to the tree. See Action and UpdateMeta for the semantics.
	// The method must return true if the augmentation's value changed.
	Update(*Config[K, Aux], Node[K, *A], UpdateMeta[K, A]) (changed bool)
}

// Action is used to classify the type of Update in order to permit various
// optimizations when updated the augmented state.
type Action int

const (

	// Default implies that no assumptions may be made with regards to the
	// change in state of the node and thus the augmented state should be
	// recalculated in full.
	Default Action = iota

	// Split indicates that this node is the left-hand side of a split.
	// The ModifiedOther will correspond to the updated state of the
	// augmentation for the right-hand side and the RelevantKey is the split
	// key to be moved into the parent.
	Split

	// Removal indicates that this is a removal event. If the RelevantNode is
	// populated, it indicates a rebalance which caused the node rooted
	// at that subtree to also be removed.
	Removal

	// Insertion indicates that this is a insertion event. If the RelevantNode is
	// populated, it indicates a rebalance which caused the node rooted
	// at that subtree to also be added.
	Insertion
)

// UpdateMeta is used to describe the update operation.
type UpdateMeta[K, A any] struct {

	// Action indicates the semantics of the below fields. If Default, no
	// fields will be populated.
	Action Action

	// ModifiedOther is the augmentation of a node which was either a previous
	// child (Removal), new child (Insertion), or represents the new
	// right-hand-side after a split.
	ModifiedOther *A

	// RelevantKey will be populated in all non-Default events.
	RelevantKey K
}
