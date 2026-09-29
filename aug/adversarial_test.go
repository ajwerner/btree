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

package aug_test

import (
	"cmp"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/ajwerner/btree/aug"
)

// TestSeekWhereNonMonotone checks that a predicate that breaks the
// monotonicity contract still leaves the iterator in a consistent state:
// valid at some entry, or past the end, never panicking or looping.
func TestSeekWhereNonMonotone(t *testing.T) {
	rng := rand.New(rand.NewPCG(5, 6))
	for _, degree := range []int{2, 16} {
		m := aug.NewMonoid[int, int, int](cmp.Compare[int], aug.Count[int, int]{}, aug.WithDegree(degree))
		for i := range 5000 {
			m.Upsert(i, i)
		}
		it := m.Iterator()
		for range 2000 {
			calls := 0
			prefix := it.SeekWhere(func(int, int) bool {
				calls++
				if calls > 100000 {
					t.Fatal("predicate called too many times")
				}
				return rng.IntN(3) == 0
			})
			if it.Valid() {
				if got := it.Prefix(); got != prefix {
					t.Fatalf("returned prefix %d, Prefix() %d", prefix, got)
				}
				if it.Key() != prefix {
					t.Fatalf("at key %d with prefix %d", it.Key(), prefix)
				}
			} else {
				// Past the end: the total is returned and the iterator
				// behaves like one that ran off.
				if prefix != 5000 {
					t.Fatalf("past the end with prefix %d, want the total", prefix)
				}
				it.Prev()
				if !it.Valid() || it.Key() != 4999 {
					t.Fatalf("Prev after a failed SeekWhere gave valid=%v %d", it.Valid(), it.Key())
				}
			}
			it.Next()
			it.Prev()
		}
		// A predicate that is always false ends past the end with the total.
		if p := it.SeekWhere(func(int, int) bool { return false }); p != 5000 || it.Valid() {
			t.Fatalf("always-false: prefix %d valid %v", p, it.Valid())
		}
		// Always true stops at the first entry.
		if p := it.SeekWhere(func(int, int) bool { return true }); p != 0 || !it.Valid() || it.Key() != 0 {
			t.Fatalf("always-true: prefix %d valid %v key %d", p, it.Valid(), it.Key())
		}
	}
}

// TestDeepTree builds a degree-2 tree taller than the iterator's inline
// frame stack and drives seeks, ranks, views and cursor writes through the
// heap-spilled stack.
func TestDeepTree(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a 2M-entry tree")
	}
	const n = 2_000_000
	m := aug.NewMonoid[int, int, int](cmp.Compare[int], aug.Count[int, int]{}, aug.WithDegree(2))
	for i := range n {
		m.Upsert(i, i)
	}
	if h := m.Height(); h <= 10 {
		t.Fatalf("height %d is not deep enough to spill the frame stack", h)
	}
	rng := rand.New(rand.NewPCG(7, 8))
	it := m.Iterator()
	for range 2000 {
		k := rng.IntN(n)
		it.SeekGE(k)
		if !it.Valid() || it.Key() != k || it.Prefix() != k {
			t.Fatalf("SeekGE(%d): valid=%v key=%d prefix=%d", k, it.Valid(), it.Key(), it.Prefix())
		}
		it.Next()
		if k+1 < n && (it.Key() != k+1 || it.Prefix() != k+1) {
			t.Fatalf("Next from %d: key=%d prefix=%d", k, it.Key(), it.Prefix())
		}
		it.Prev()
		it.Prev()
		if k > 0 && it.Key() != k-1 {
			t.Fatalf("Prev twice from %d: key=%d", k+1, it.Key())
		}
		if p, found := m.Prefix(k); p != k || !found {
			t.Fatalf("Prefix(%d) = %d %v", k, p, found)
		}
	}
	count := 0
	for k := range m.All() {
		if k != count {
			t.Fatalf("All out of order at %d", count)
		}
		count++
	}
	if count != n {
		t.Fatalf("All visited %d", count)
	}
	// Cursor writes through the deep pinned path, on a clone so that every
	// frame is copied on the first write.
	c := m.Clone().Cursor()
	for i := range 1000 {
		k := rng.IntN(n)
		c.SeekGE(k)
		c.SetValue(-i)
		c.Rekey(k) // in place: same key is between the neighbours
		if _, v := c.Delete(); v != -i {
			t.Fatalf("Delete returned value %d, want %d", v, -i)
		}
		c.Upsert(k, i)
		if !c.Valid() || c.Key() != k || c.Value() != i {
			t.Fatalf("after cursor round trip at %d: valid=%v key=%d value=%d", k, c.Valid(), c.Key(), c.Value())
		}
	}
	if v, ok := m.Get(0); !ok || v != 0 {
		t.Fatalf("original changed: %d %v", v, ok)
	}
}

func TestDegreeBounds(t *testing.T) {
	for _, d := range []int{1, aug.MaxDegree + 1} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("degree %d accepted", d)
				}
			}()
			aug.New[int, int, struct{}](cmp.Compare[int], nil, aug.WithDegree(d))
		}()
	}
	m := aug.New[int, int, struct{}](cmp.Compare[int], nil, aug.WithDegree(aug.MaxDegree))
	for i := range 3 * aug.MaxDegree {
		m.Upsert(i, i)
	}
	it := m.Iterator()
	n := 0
	for it.First(); it.Valid(); it.Next() {
		n++
	}
	if n != 3*aug.MaxDegree || m.Height() != 2 {
		t.Fatalf("iterated %d of %d entries, height %d", n, m.Len(), m.Height())
	}
	if err := m.Verify(); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyInterfaceTypes(t *testing.T) {
	m := aug.New[any, any, struct{}](func(a, b any) int { return cmp.Compare(a.(int), b.(int)) }, nil, aug.WithDegree(2))
	for i := range 100 {
		m.Upsert(i, i)
	}
	for i := 0; i < 100; i += 3 {
		m.Delete(i)
	}
	if err := m.Verify(); err != nil {
		t.Fatal(err)
	}
}

// nodeCounter is a shape-dependent augmentation: the number of nodes in
// the subtree. It recomputes from its children on every event, as the
// Updater documentation prescribes for such augmentations.
type nodeCounter[K, V any] struct{}

func (nodeCounter[K, V]) Update(n *aug.Node[K, V, int], _ aug.UpdateInfo[K, V, int]) bool {
	count := 1
	if !n.IsLeaf() {
		for i := int16(0); i <= n.Count(); i++ {
			count += *n.ChildAug(i)
		}
	}
	changed := *n.Aug() != count
	*n.Aug() = count
	return changed
}

// TestStructuralAugmentation checks that splits, merges and rebalances
// reach ancestors' augmentations even when no entry below them changed,
// including the merge caused by deleting a key that is absent.
func TestStructuralAugmentation(t *testing.T) {
	// The reviewer's example: degree 2, keys 1..6 leave the root at 3
	// nodes instead of 4 when the split is not reported.
	m := aug.New[int, int, int](cmp.Compare[int], nodeCounter[int, int]{}, aug.WithDegree(2))
	for i := 1; i <= 6; i++ {
		m.Upsert(i, i)
	}
	if err := m.Verify(); err != nil {
		t.Fatal(err)
	}
	for _, degree := range []int{2, 3} {
		m := aug.New[int, int, int](cmp.Compare[int], nodeCounter[int, int]{}, aug.WithDegree(degree))
		rng := rand.New(rand.NewPCG(31, 32))
		for step := range 20000 {
			k := rng.IntN(500)
			switch rng.IntN(3) {
			case 0, 1:
				m.Upsert(k, step)
			case 2:
				m.Delete(rng.IntN(600)) // often absent, which can still merge
			}
			if step%500 == 499 {
				if err := m.Verify(); err != nil {
					t.Fatalf("degree %d step %d: %v", degree, step, err)
				}
			}
		}
	}
}

// maxFloat is a NaN-propagating maximum over non-negative floats whose
// Equaler treats NaN as equal to itself, as cmp.Compare does, where
// reflect.DeepEqual would not.
type maxFloat struct{}

func (maxFloat) Of(_ int, v float64) float64 { return v }
func (maxFloat) Combine(a, b float64) float64 {
	if math.IsNaN(a) || math.IsNaN(b) {
		return math.NaN()
	}
	return max(a, b)
}
func (maxFloat) Equal(a, b float64) bool { return cmp.Compare(a, b) == 0 }

func TestVerifyUsesEqualer(t *testing.T) {
	m := aug.NewMonoid[int, float64, float64](cmp.Compare[int], maxFloat{}, aug.WithDegree(2))
	nan := math.NaN()
	for i := range 50 {
		m.Upsert(i, nan)
	}
	if err := m.Verify(); err != nil {
		t.Fatal(err)
	}
}
