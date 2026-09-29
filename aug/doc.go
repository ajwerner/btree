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

// Package aug implements a copy-on-write augmented B-tree. It is the core
// shared by the btree, orderstat and interval packages and is exported so
// that users can define their own augmentations.
//
// # Ownership
//
// The same contract holds for every collection in this module:
//
//   - New constructs a collection; its zero value is not usable, and it
//     must not be copied by value.
//   - Clone returns another collection that may be mutated independently
//     of the original, in constant time; the two share nodes until one
//     writes them.
//   - Iterators and cursors borrow the collection they came from and
//     observe it as it is when they are positioned; sequence views (All,
//     Range, ...) observe it each time they are ranged over.
//   - A mutation invalidates every borrowed position on that collection
//     except the cursor performing it; positioning again is always safe.
//   - Clear empties a collection and releases what it shared; the
//     collection stays usable.
//
// # Concurrency
//
// A Map owns a reference to its root node; nodes are reference counted and
// shared between a Map and its clones. A mutation copies each node on the
// path from the root to the affected leaf the first time that node is
// touched after a Clone, and mutates in place thereafter.
//
// Writes to a Map must be serialized by the caller. Any number of goroutines
// may read a Map concurrently as long as none writes it; the usual way to
// get a snapshot for readers is Clone. Clone itself is a write to the
// receiver (it bumps a reference count) and must be serialized with other
// writes to the receiver, but the returned clone is independent and may be
// used from another goroutine immediately.
//
// A Map must not be copied by value: a copy shares the root without holding
// a reference to it, so clearing or writing either corrupts the other. Use
// the pointer New returns, and Clone for a second handle.
//
// Keys, values and augmentations are copied shallowly when a node is
// copied on write, so a map, its clones and their free-listed nodes may
// share any references they contain. Treat a key or augmentation that
// holds a reference as immutable once stored, and have Monoid operations
// return fresh values rather than modify their arguments.
//
// Clear releases the Map's reference to its nodes. Nodes that no other Map
// references are returned to the free list. A Map that is dropped without
// Clear leaks nothing to the garbage collector's eyes, but nodes it shared
// stay pinned at a reference count above one, so every later write to a
// surviving clone copies them once more before mutating in place.
package aug
