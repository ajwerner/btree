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

import (
	"fmt"
	"math"
)

// DefaultDegree is the degree used by New when WithDegree is not given. A
// tree of degree d holds between d-1 and 2d-1 entries in every node except
// the root.
const DefaultDegree = 16

// MaxDegree is the largest degree New accepts: positions within a node
// are 16-bit, and traversals use one past the last entry as a sentinel.
const MaxDegree = math.MaxInt16 / 2

// DefaultFreeListSize is the capacity of the free list created by New when
// WithFreeList is not given. A clone that is written to and then cleared
// frees about one node per touched root-to-leaf path, so a workload that
// clones, applies a few hundred writes and clears fits within the default;
// heavier churn should size its own free list with WithFreeList.
const DefaultFreeListSize = 256

// Config holds a Map's comparison function and Updater, fixed at
// construction. Augmentation code reaches a copy through
// LowLevelIterator.Config.
type Config[K, V, A any] struct {

	// Updater is used to update the augmentations to the tree.
	Updater Updater[K, V, A]

	cmp func(K, K) int
}

// Updater is used to update the augmentation of the node when the subtree
// changes.
//
// The tree tells an Updater about every entry that enters, leaves or is
// replaced below a node, and recomputes a node (Default) when it is
// restructured: after a split, merge or rebalance the node whose children
// changed shape receives Default, and if its augmentation changed the
// ancestors hear of it through the event that caused the restructuring
// or through Default. An augmentation that depends only on the entries
// of the subtree (counts, sums, bounds) therefore needs the incremental
// cases only; one that depends on the shape of the subtree (node counts,
// heights) should recompute from its children on every event.
//
// An Updater may implement Equaler to give Verify its notion of equality.
type Updater[K, V, A any] interface {

	// Update should update the augmentation of the passed node, optionally
	// using the data in the UpdateInfo to optimize the update. If the
	// augmentation changed, and thus, changes should occur in the ancestors
	// of the subtree rooted at this node, return true.
	Update(*Node[K, V, A], UpdateInfo[K, V, A]) (changed bool)
}

// UpdateInfo is used to describe the update operation.
type UpdateInfo[K, V, A any] struct {

	// Action indicates the semantics of the below fields. If Default, no
	// fields will be populated.
	Action Action

	// ModifiedOther is the augmentation of a node which was either a previous
	// child (Removal), new child (Insertion), or represents the new
	// right-hand-side after a split.
	ModifiedOther *A

	// RelevantKey and RelevantValue are the entry that was inserted,
	// removed or moved into the parent (Split), or the new entry for a
	// Replacement. They are populated in all non-Default events.
	RelevantKey   K
	RelevantValue V

	// PrevKey and PrevValue are the entry that was replaced. They are
	// populated only for Replacement.
	PrevKey   K
	PrevValue V
}

// Action is used to classify the type of Update in order to permit various
// optimizations when updating the augmented state.
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

	// Removal indicates that this is a removal event. If ModifiedOther is
	// populated, it indicates a rebalance which caused the subtree with
	// that augmentation to also be removed.
	Removal

	// Insertion indicates that this is an insertion event. If ModifiedOther
	// is populated, it indicates a rebalance which caused the subtree with
	// that augmentation to also be added.
	Insertion

	// Replacement indicates that an entry in the subtree rooted at this
	// node was replaced by another at the same position. RelevantKey and
	// RelevantValue are the new entry; PrevKey and PrevValue the old. The
	// keys are equal, except for Cursor.Rekey, where the new key still
	// sorts between the same neighbours as the old one.
	Replacement
)

// Compare compares two values using the same comparison function as the Map.
func (c *Config[K, V, A]) Compare(a, b K) int { return c.cmp(a, b) }

type config[K, V, A any] struct {
	Config[K, V, A]
	find       func(*Node[K, V, A], K) (int, bool) // nil: binary search with cmp
	monoid     CommutativeMonoid[K, V, A]          // set when Updater came from MonoidUpdater
	folder     Folder[K, V, A]                     // set when the monoid implements Folder
	fl         FreeList[K, V, A]
	maxEntries int
	minEntries int
}

// Option configures a Map at construction.
type Option interface {
	apply(*options)
}

type options struct {
	degree   int
	freeList any
	find     any
}

type optionFunc func(*options)

func (f optionFunc) apply(o *options) { f(o) }

// WithDegree sets the degree of the tree. A tree of degree d holds between
// d-1 and 2d-1 entries in every node except the root. The degree must be at
// least 2. Smaller degrees copy less per write after a Clone; larger degrees
// are shallower.
func WithDegree(degree int) Option {
	return optionFunc(func(o *options) { o.degree = degree })
}

// WithFreeList sets the free list from which the tree allocates nodes and to
// which it returns them. The free list's type parameters must match those
// of the Map being constructed, or New panics. A free list may be shared by
// any number of trees, including trees of different degrees.
func WithFreeList[K, V, A any](fl FreeList[K, V, A]) Option {
	return optionFunc(func(o *options) { o.freeList = fl })
}

// withFind replaces the node search. It is used by NewOrdered.
func withFind[K, V, A any](find func(*Node[K, V, A], K) (int, bool)) Option {
	return optionFunc(func(o *options) { o.find = find })
}

func makeConfig[K, V, A any](
	cmp func(K, K) int, up Updater[K, V, A], opts []Option,
) config[K, V, A] {
	o := options{degree: DefaultDegree}
	for _, opt := range opts {
		opt.apply(&o)
	}
	if o.degree < 2 || o.degree > MaxDegree {
		panic(fmt.Sprintf("btree: degree must be in [2, %d], got %d", MaxDegree, o.degree))
	}
	var fl FreeList[K, V, A]
	if o.freeList != nil {
		var ok bool
		if fl, ok = o.freeList.(FreeList[K, V, A]); !ok {
			var want FreeList[K, V, A]
			panic(fmt.Sprintf("btree: WithFreeList: got %T, want %T", o.freeList, want))
		}
	} else {
		fl = NewFreeList[K, V, A](DefaultFreeListSize)
	}
	if cmp == nil {
		panic("btree: a comparison function is required")
	}
	var m CommutativeMonoid[K, V, A]
	var f Folder[K, V, A]
	if mu, ok := up.(interface {
		monoid() CommutativeMonoid[K, V, A]
	}); ok {
		m = mu.monoid()
		f, _ = m.(Folder[K, V, A])
	}
	var find func(*Node[K, V, A], K) (int, bool)
	if o.find != nil {
		find, _ = o.find.(func(*Node[K, V, A], K) (int, bool))
	}
	return config[K, V, A]{
		Config:     Config[K, V, A]{Updater: up, cmp: cmp},
		find:       find,
		monoid:     m,
		folder:     f,
		fl:         fl,
		maxEntries: 2*o.degree - 1,
		minEntries: o.degree - 1,
	}
}
