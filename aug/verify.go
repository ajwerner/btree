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

import (
	"fmt"
	"reflect"
	"sync/atomic"
)

// Verify checks the structural invariants of the tree and returns an error
// describing the first violation found. It is intended for tests. Like any
// read, it must not run concurrently with a writer of this tree.
func (t *Map[K, V, A]) Verify() error {
	if t.root == nil {
		if t.length != 0 {
			return fmt.Errorf("nil root with length %d", t.length)
		}
		return nil
	}
	v := verifier[K, V, A]{cfg: &t.cfg, leafDepth: -1}
	if err := v.node(t.root, 0, true, nil, nil); err != nil {
		return err
	}
	if v.items != t.length {
		return fmt.Errorf("length %d but counted %d items", t.length, v.items)
	}
	return nil
}

// isZero reports whether x is the zero value of its type, including a nil
// interface, for which reflect.ValueOf returns the invalid Value.
func isZero[T any](x T) bool {
	v := reflect.ValueOf(x)
	return !v.IsValid() || v.IsZero()
}

type verifier[K, V, A any] struct {
	cfg       *config[K, V, A]
	items     int
	leafDepth int
}

func (v *verifier[K, V, A]) node(n *Node[K, V, A], depth int, isRoot bool, lo, hi *K) error {
	if r := atomic.LoadInt32(&n.ref); r < 1 {
		return fmt.Errorf("node at depth %d has ref %d", depth, r)
	}
	count := len(n.keys)
	if count > v.cfg.maxEntries {
		return fmt.Errorf("node at depth %d has count %d > %d", depth, count, v.cfg.maxEntries)
	}
	if !isRoot && count < v.cfg.minEntries {
		return fmt.Errorf("non-root node at depth %d has count %d < %d", depth, count, v.cfg.minEntries)
	}
	if isRoot && count == 0 {
		return fmt.Errorf("root has count 0")
	}
	cmp := v.cfg.cmp
	for i := range count {
		k := n.keys[i]
		if i > 0 && cmp(n.keys[i-1], k) >= 0 {
			return fmt.Errorf("node at depth %d keys out of order at %d", depth, i)
		}
		if lo != nil && cmp(*lo, k) >= 0 {
			return fmt.Errorf("node at depth %d key %d not above lower bound", depth, i)
		}
		if hi != nil && cmp(k, *hi) >= 0 {
			return fmt.Errorf("node at depth %d key %d not below upper bound", depth, i)
		}
	}
	v.items += count
	if n.IsLeaf() {
		if v.leafDepth == -1 {
			v.leafDepth = depth
		} else if v.leafDepth != depth {
			return fmt.Errorf("leaf at depth %d, expected %d", depth, v.leafDepth)
		}
	} else {
		if len(n.children) != count+1 {
			return fmt.Errorf("node at depth %d has %d children for %d entries", depth, len(n.children), count)
		}
		for i, c := range n.children {
			if c == nil {
				return fmt.Errorf("node at depth %d has nil child %d", depth, i)
			}
			clo, chi := lo, hi
			if i > 0 {
				clo = &n.keys[i-1]
			}
			if i < count {
				chi = &n.keys[i]
			}
			if err := v.node(c, depth+1, false, clo, chi); err != nil {
				return err
			}
		}
		for i, c := range n.children[len(n.children):cap(n.children)] {
			if c != nil {
				return fmt.Errorf("node at depth %d has stale child pointer at %d", depth, len(n.children)+i)
			}
		}
	}
	for i, k := range n.keys[len(n.keys):cap(n.keys)] {
		if !isZero(k) {
			return fmt.Errorf("node at depth %d has stale key at %d", depth, len(n.keys)+i)
		}
	}
	for i, v := range n.values[len(n.values):cap(n.values)] {
		if !isZero(v) {
			return fmt.Errorf("node at depth %d has stale value at %d", depth, len(n.values)+i)
		}
	}
	if v.cfg.Updater != nil {
		// Recompute on a shallow copy so that nodes shared with snapshots
		// being read elsewhere are never written.
		tmp := *n
		v.cfg.Updater.Update(&tmp, UpdateInfo[K, V, A]{})
		if !reflect.DeepEqual(n.aug, tmp.aug) {
			return fmt.Errorf("node at depth %d has stale augmentation %v, recomputed %v", depth, n.aug, tmp.aug)
		}
	}
	return nil
}
