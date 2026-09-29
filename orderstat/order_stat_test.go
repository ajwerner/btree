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

package orderstat

import (
	"cmp"
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/ajwerner/btree/aug"
)

func TestOrderStatTree(t *testing.T) {
	tree := New[int, int](cmp.Compare[int])
	tree.Upsert(2, 1)
	tree.Upsert(3, 2)
	tree.Upsert(5, 4)
	tree.Upsert(4, 3)
	iter := tree.Iterator()
	iter.First()
	for i, exp := range []int{2, 3, 4, 5} {
		requireEqual(t, exp, iter.Cur())
		requireEqual(t, i, iter.Rank())
		iter.Next()
	}
	iter.SeekNth(2)
	requireEqual(t, 4, iter.Cur())
}

func TestOrderStatNth(t *testing.T) {
	t.Parallel()
	tree := NewSet(cmp.Compare[int])
	const maxN = 1000
	N := rand.IntN(maxN)
	items := make([]int, 0, N)
	for i := range N {
		items = append(items, i)
	}
	perm := rand.Perm(N)
	for _, idx := range perm {
		tree.Upsert(items[idx])
	}
	removePerm := rand.Perm(N)
	retainAll := rand.Float64() < .25
	var removed []int
	for _, idx := range removePerm {
		if !retainAll && rand.Float64() < .05 {
			continue
		}
		tree.Delete(items[idx])
		removed = append(removed, items[idx])
	}
	t.Logf("removed %d/%d", len(removed), N)
	for _, i := range removed {
		tree.Upsert(i)
	}
	perm = rand.Perm(N)

	iter := tree.Iterator()
	for _, idx := range perm {
		iter.SeekNth(idx)
		requireEqual(t, items[idx], iter.Cur())
		for i := idx + 1; i < N; i++ {
			iter.Next()
			requireEqual(t, items[i], iter.Cur())
		}
		requireEqual(t, true, iter.Valid())
		iter.Next()
		requireEqual(t, false, iter.Valid())
	}

	clone := tree.Clone()
	clone.Clear()
	requireEqual(t, len(perm), tree.Len())

}

func Example_blog() {
	s := NewSet(cmp.Compare[int])
	for _, i := range rand.Perm(100) {
		s.Upsert(i)
	}
	fmt.Println(s.Len())
	it := s.Iterator()
	it.SeekNth(90)
	fmt.Println(it.Cur())

	// Output:
	// 100
	// 90
}

func requireEqual[T comparable](t *testing.T, exp, got T) {
	t.Helper()
	if exp != got {
		t.Fatalf("expected %v, got %v", exp, got)
	}
}

// TestRankAndSeekNthAtEveryHeight checks Rank and SeekNth against a sorted
// slice for trees of height 1, 2 and 3. At the current degree the tree is
// height 2 up to roughly 16k items and height 3 beyond that.
func TestRankAndSeekNthAtEveryHeight(t *testing.T) {
	for _, degree := range []int{2, 4, 16} {
		t.Run(fmt.Sprintf("degree=%d", degree), func(t *testing.T) {
			testRankAndSeekNth(t, degree)
		})
	}
}

func testRankAndSeekNth(t *testing.T, degree int) {
	rng := rand.New(rand.NewPCG(11, 13))
	for _, n := range []int{1, 50, 200, 5000, 20000, 300000} {
		if degree == 2 && n > 20000 {
			break
		}
		s := NewSet(cmp.Compare[int], aug.WithDegree(degree))
		keys := make([]int, n)
		for i := range keys {
			keys[i] = i * 3
		}
		for _, i := range rng.Perm(n) {
			s.Upsert(keys[i])
		}
		if err := s.Verify(); err != nil {
			t.Fatalf("n=%d: %v", n, err)
		}
		height := s.Height()
		it := s.Iterator()
		// Every position, walking forward, for the smaller sizes; a sample
		// for the largest.
		positions := n
		if n > 20000 {
			positions = 2000
		}
		for j := range positions {
			idx := j
			if positions != n {
				idx = rng.IntN(n)
			}
			it.SeekNth(idx)
			if !it.Valid() || it.Cur() != keys[idx] {
				t.Fatalf("n=%d h=%d: SeekNth(%d) = %v %d, want %d", n, height, idx, it.Valid(), it.Cur(), keys[idx])
			}
			if r := it.Rank(); r != idx {
				t.Fatalf("n=%d h=%d: Rank after SeekNth(%d) = %d", n, height, idx, r)
			}
			it.SeekGE(keys[idx])
			if r := it.Rank(); r != idx {
				t.Fatalf("n=%d h=%d: Rank after SeekGE(%d) = %d, want %d", n, height, keys[idx], r, idx)
			}
			it.Next()
			if idx+1 < n {
				if r := it.Rank(); r != idx+1 {
					t.Fatalf("n=%d h=%d: Rank after Next from %d = %d", n, height, idx, r)
				}
			} else if it.Rank() != n {
				t.Fatalf("n=%d h=%d: Rank past end = %d, want %d", n, height, it.Rank(), n)
			}
			if r, found := s.Rank(keys[idx]); r != idx || !found {
				t.Fatalf("n=%d h=%d: Map.Rank(%d) = %d %v", n, height, keys[idx], r, found)
			}
			if r, found := s.Rank(keys[idx] + 1); r != idx+1 || found {
				t.Fatalf("n=%d h=%d: Map.Rank(%d) = %d %v, want %d", n, height, keys[idx]+1, r, found, idx+1)
			}
			if r, found := s.Rank(keys[idx] - 1); r != idx || found {
				t.Fatalf("n=%d h=%d: Map.Rank(%d) = %d %v, want %d", n, height, keys[idx]-1, r, found, idx)
			}
			if got, ok := s.Nth(idx); !ok || got != keys[idx] {
				t.Fatalf("n=%d h=%d: Nth(%d) = %d %v", n, height, idx, got, ok)
			}
			if j := rng.IntN(n + 1); j >= idx {
				if c := s.Count(keys[idx], keys[idx]+3*(j-idx)); c != j-idx {
					t.Fatalf("n=%d h=%d: Count(%d, %d) = %d, want %d", n, height, idx, j, c, j-idx)
				}
			}
		}
		if n >= 20000 && height < 3 {
			t.Fatalf("n=%d: expected height >= 3, got %d", n, height)
		}
		it.SeekNth(n)
		if it.Valid() {
			t.Fatalf("n=%d: SeekNth(n) is valid", n)
		}
		it.SeekNth(-1)
		if it.Valid() {
			t.Fatalf("n=%d: SeekNth(-1) is valid", n)
		}
	}
}

// TestOrderStatDifferential interleaves mutation, cloning and rank queries
// against a reference model.
func TestOrderStatDifferential(t *testing.T) {
	seed := rand.Uint64()
	t.Logf("seed %d", seed)
	rng := rand.New(rand.NewPCG(seed, 0))
	m := New[int, int](cmp.Compare[int])
	ref := map[int]int{}
	var clones []*Map[int, int]
	var cloneRefs []map[int]int
	check := func(m *Map[int, int], ref map[int]int) {
		t.Helper()
		if err := m.Verify(); err != nil {
			t.Fatal(err)
		}
		keys := make([]int, 0, len(ref))
		for k := range ref {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		if m.Len() != len(keys) {
			t.Fatalf("len %d, want %d", m.Len(), len(keys))
		}
		it := m.Iterator()
		i := 0
		for it.First(); it.Valid(); it.Next() {
			if it.Cur() != keys[i] || it.Value() != ref[keys[i]] || it.Rank() != i {
				t.Fatalf("position %d: (%d, %d) rank %d, want (%d, %d)", i, it.Cur(), it.Value(), it.Rank(), keys[i], ref[keys[i]])
			}
			i++
		}
		for j := 0; j < 32 && len(keys) > 0; j++ {
			idx := rng.IntN(len(keys))
			it.SeekNth(idx)
			if it.Cur() != keys[idx] {
				t.Fatalf("SeekNth(%d) = %d, want %d", idx, it.Cur(), keys[idx])
			}
		}
	}
	for step := range 40000 {
		k := rng.IntN(30000)
		switch op := rng.IntN(100); {
		case op < 60:
			m.Upsert(k, step)
			ref[k] = step
		case op < 95:
			m.Delete(k)
			delete(ref, k)
		default:
			if len(clones) < 4 {
				clones = append(clones, m.Clone())
				cr := make(map[int]int, len(ref))
				for k, v := range ref {
					cr[k] = v
				}
				cloneRefs = append(cloneRefs, cr)
			}
		}
		if step%2000 == 1999 {
			check(m, ref)
			for i := range clones {
				for j := 0; j < 50; j++ {
					k := rng.IntN(30000)
					clones[i].Upsert(k, -j)
					cloneRefs[i][k] = -j
				}
				check(clones[i], cloneRefs[i])
			}
		}
	}
	for i := range clones {
		clones[i].Clear()
	}
	check(m, ref)
}
