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

package btree

import (
	"cmp"
	"fmt"
	"maps"
	"math/rand/v2"
	"slices"
	"testing"
)

func TestBTree(t *testing.T) {
	tree := NewSet(cmp.Compare[int])
	tree.Upsert(2)
	tree.Upsert(12)
	tree.Upsert(1)

	it := tree.Iterator()
	it.First()
	expected := []int{1, 2, 12}
	for _, exp := range expected {
		if got := it.Key(); got != exp {
			t.Fatalf("expected %d, got %d", exp, got)
		}
		it.Next()
	}
	if it.Valid() {
		t.Fatal("expected iterator to be exhausted")
	}
}

// model is a reference implementation used by the differential tests.
type model struct {
	m map[int]int
}

func (r *model) sortedKeys() []int {
	keys := make([]int, 0, len(r.m))
	for k := range r.m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func (r *model) clone() *model {
	c := &model{m: make(map[int]int, len(r.m))}
	maps.Copy(c.m, r.m)
	return c
}

// checkMap verifies that m matches the model exactly: length, forward and
// reverse iteration, Get, SeekGE and SeekLT, and structural invariants.
func checkMap(t *testing.T, tag string, m *Map[int, int], r *model, rng *rand.Rand) {
	t.Helper()
	if err := m.Verify(); err != nil {
		t.Fatalf("%s: %v", tag, err)
	}
	keys := r.sortedKeys()
	if m.Len() != len(keys) {
		t.Fatalf("%s: len %d, want %d", tag, m.Len(), len(keys))
	}
	it := m.Iterator()
	i := 0
	for it.First(); it.Valid(); it.Next() {
		if i >= len(keys) || it.Key() != keys[i] || it.Value() != r.m[keys[i]] {
			t.Fatalf("%s: forward position %d: got (%d, %d)", tag, i, it.Key(), it.Value())
		}
		i++
	}
	if i != len(keys) {
		t.Fatalf("%s: forward walk visited %d, want %d", tag, i, len(keys))
	}
	i = len(keys) - 1
	for it.Last(); it.Valid(); it.Prev() {
		if i < 0 || it.Key() != keys[i] {
			t.Fatalf("%s: reverse position %d: got %d", tag, i, it.Key())
		}
		i--
	}
	if i != -1 {
		t.Fatalf("%s: reverse walk stopped at %d", tag, i)
	}
	for range 64 {
		k := rng.IntN(1200) - 100
		v, ok := m.Get(k)
		rv, rok := r.m[k]
		if ok != rok || v != rv {
			t.Fatalf("%s: Get(%d) = (%d, %v), want (%d, %v)", tag, k, v, ok, rv, rok)
		}
		idx, exists := slices.BinarySearch(keys, k)
		if found := it.SeekGE(k); found != exists {
			t.Fatalf("%s: SeekGE(%d) found=%v, want %v", tag, k, found, exists)
		}
		if idx < len(keys) {
			if !it.Valid() || it.Key() != keys[idx] {
				t.Fatalf("%s: SeekGE(%d) valid=%v cur=%d, want %d", tag, k, it.Valid(), it.Key(), keys[idx])
			}
		} else if it.Valid() {
			t.Fatalf("%s: SeekGE(%d) should be invalid", tag, k)
		}
		it.SeekLT(k)
		if idx > 0 {
			if !it.Valid() || it.Key() != keys[idx-1] {
				t.Fatalf("%s: SeekLT(%d) valid=%v cur=%d, want %d", tag, k, it.Valid(), it.Key(), keys[idx-1])
			}
		} else if it.Valid() {
			t.Fatalf("%s: SeekLT(%d) should be invalid", tag, k)
		}
	}
}

// TestDifferential drives a Map and a set of clones against a reference
// model with interleaved Upsert, Delete, Clone and Reset operations.
func TestDifferential(t *testing.T) {
	for _, degree := range []int{2, 3, 5, 16, 64} {
		for _, ordered := range []bool{false, true} {
			t.Run(fmt.Sprintf("degree=%d/ordered=%v", degree, ordered), func(t *testing.T) {
				testDifferential(t, degree, ordered)
			})
		}
	}
}

func testDifferential(t *testing.T, degree int, ordered bool) {
	seed := rand.Uint64()
	t.Logf("seed %d", seed)
	rng := rand.New(rand.NewPCG(seed, 0))

	type tree struct {
		m *Map[int, int]
		r *model
	}
	base := New[int, int](cmp.Compare[int], WithDegree(degree))
	if ordered {
		base = NewOrdered[int, int](WithDegree(degree))
	}
	trees := []tree{{base, &model{m: map[int]int{}}}}
	const steps = 20000
	for step := range steps {
		tr := &trees[rng.IntN(len(trees))]
		k := rng.IntN(1000)
		switch op := rng.IntN(100); {
		case op < 55:
			_, ov, replaced := tr.m.Upsert(k, step)
			pv, ok := tr.r.m[k]
			if replaced != ok || (ok && ov != pv) {
				t.Fatalf("step %d: Upsert(%d) replaced=%v old=%d, want %v %d", step, k, replaced, ov, ok, pv)
			}
			tr.r.m[k] = step
		case op < 90:
			_, ov, found := tr.m.Delete(k)
			pv, ok := tr.r.m[k]
			if found != ok || (ok && ov != pv) {
				t.Fatalf("step %d: Delete(%d) found=%v old=%d, want %v %d", step, k, found, ov, ok, pv)
			}
			delete(tr.r.m, k)
		case op < 97:
			if len(trees) < 8 {
				trees = append(trees, tree{tr.m.Clone(), tr.r.clone()})
			}
		default:
			if len(trees) > 1 {
				idx := rng.IntN(len(trees))
				trees[idx].m.Clear()
				trees = slices.Delete(trees, idx, idx+1)
			}
		}
		if step%500 == 0 || step == steps-1 {
			for i := range trees {
				checkMap(t, "tree", trees[i].m, trees[i].r, rng)
			}
		}
	}
}

func TestIteratorEdges(t *testing.T) {
	for _, degree := range []int{2, 16} {
		for _, n := range []int{0, 1, 10, 1000, 40000} {
			testIteratorEdges(t, degree, n)
		}
	}
}

func testIteratorEdges(t *testing.T, degree, n int) {
	{
		s := NewSet(cmp.Compare[int], WithDegree(degree))
		for i := range n {
			s.Upsert(i)
		}
		it := s.Iterator()
		// Prev before the beginning stays invalid; Next then yields the
		// first key.
		it.Prev()
		if it.Valid() {
			t.Fatalf("n=%d: Prev on reset iterator is valid", n)
		}
		it.Next()
		if n == 0 {
			if it.Valid() {
				t.Fatalf("n=%d: Next on empty is valid", n)
			}
			return
		}
		if !it.Valid() || it.Key() != 0 {
			t.Fatalf("n=%d: Next after reset gives %v %d", n, it.Valid(), it.Key())
		}
		// Next past the end stays invalid; Prev then yields the last key.
		it.Last()
		it.Next()
		it.Next()
		if it.Valid() {
			t.Fatalf("n=%d: Next past end is valid", n)
		}
		it.Prev()
		if !it.Valid() || it.Key() != n-1 {
			t.Fatalf("n=%d: Prev after end gives %v %d", n, it.Valid(), it.Key())
		}
		// Prev past the beginning stays invalid; Next then yields the first.
		it.First()
		it.Prev()
		it.Prev()
		if it.Valid() {
			t.Fatalf("n=%d: Prev past beginning is valid", n)
		}
		it.Next()
		if !it.Valid() || it.Key() != 0 {
			t.Fatalf("n=%d: Next after beginning gives %v %d", n, it.Valid(), it.Key())
		}
		// SeekGE past the end then Prev yields the last key.
		it.SeekGE(n + 5)
		if it.Valid() {
			t.Fatalf("n=%d: SeekGE past end is valid", n)
		}
		it.Prev()
		if !it.Valid() || it.Key() != n-1 {
			t.Fatalf("n=%d: Prev after SeekGE past end gives %v %d", n, it.Valid(), it.Key())
		}
		// SeekLT before the beginning then Next yields the first key.
		it.SeekLT(0)
		if it.Valid() {
			t.Fatalf("n=%d: SeekLT before beginning is valid", n)
		}
		it.Next()
		if !it.Valid() || it.Key() != 0 {
			t.Fatalf("n=%d: Next after SeekLT before beginning gives %v %d", n, it.Valid(), it.Key())
		}
	}
}

func TestCloneIsolation(t *testing.T) {
	m := New[int, string](cmp.Compare[int])
	for i := range 5000 {
		m.Upsert(i, "orig")
	}
	c := m.Clone()
	for i := 0; i < 5000; i += 7 {
		c.Upsert(i, "clone")
	}
	c.Delete(3)
	for i := range 5000 {
		v, ok := m.Get(i)
		if !ok || v != "orig" {
			t.Fatalf("original key %d: (%q, %v)", i, v, ok)
		}
		v, ok = c.Get(i)
		switch {
		case i == 3:
			if ok {
				t.Fatalf("clone key %d should be deleted", i)
			}
		case i%7 == 0:
			if !ok || v != "clone" {
				t.Fatalf("clone key %d: (%q, %v)", i, v, ok)
			}
		default:
			if !ok || v != "orig" {
				t.Fatalf("clone key %d: (%q, %v)", i, v, ok)
			}
		}
	}
	m.Clear()
	if err := c.Verify(); err != nil {
		t.Fatal(err)
	}
	if c.Len() != 4999 {
		t.Fatalf("clone len %d", c.Len())
	}
	c.Clear()
	if c.Len() != 0 || m.Len() != 0 {
		t.Fatal("reset did not empty")
	}
}

// FuzzMap interprets the input as a script of operations against a Map and
// a model, with clones taken and released along the way.
func FuzzMap(f *testing.F) {
	f.Add([]byte{0, 1, 0, 2, 1, 1, 2, 0, 0, 3, 3, 0})
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, script []byte) {
		type tree struct {
			m *Map[int, int]
			r *model
		}
		if len(script) == 0 {
			return
		}
		degree := 2 + int(script[0]%8)
		script = script[1:]
		base := New[int, int](cmp.Compare[int], WithDegree(degree))
		trees := []tree{{base, &model{m: map[int]int{}}}}
		rng := rand.New(rand.NewPCG(uint64(len(script)), 1))
		for i := 0; i+1 < len(script); i += 2 {
			op, k := script[i], int(script[i+1])
			tr := &trees[int(op>>2)%len(trees)]
			switch op & 3 {
			case 0:
				tr.m.Upsert(k, i)
				tr.r.m[k] = i
			case 1:
				tr.m.Delete(k)
				delete(tr.r.m, k)
			case 2:
				if len(trees) < 4 {
					trees = append(trees, tree{tr.m.Clone(), tr.r.clone()})
				}
			case 3:
				if len(trees) > 1 {
					idx := k % len(trees)
					trees[idx].m.Clear()
					trees = slices.Delete(trees, idx, idx+1)
				}
			}
		}
		for i := range trees {
			checkMap(t, "fuzz", trees[i].m, trees[i].r, rng)
		}
	})
}
