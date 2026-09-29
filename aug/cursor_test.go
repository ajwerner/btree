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
	"fmt"
	"maps"
	"math/rand/v2"
	"slices"
	"sync"
	"testing"

	"github.com/ajwerner/btree/aug"
)

type sumStats = aug.Pair[int, int] // count, sum of values

func newSumMap(degree int) *aug.MonoidMap[int, int, sumStats] {
	return aug.NewMonoid[int, int, sumStats](cmp.Compare[int],
		aug.PairOf[int, int, int, int](aug.Count[int, int]{}, aug.Sum(func(_ int, v int) int { return v })),
		aug.WithDegree(degree))
}

func TestCursor(t *testing.T) {
	for _, degree := range []int{2, 3, 16} {
		t.Run(fmt.Sprintf("degree=%d", degree), func(t *testing.T) {
			testCursor(t, degree)
		})
	}
}

func testCursor(t *testing.T, degree int) {
	seed := rand.Uint64()
	t.Logf("seed %d", seed)
	rng := rand.New(rand.NewPCG(seed, 0))
	m := newSumMap(degree)
	ref := map[int]int{}
	sortedKeys := func(ref map[int]int) []int {
		keys := make([]int, 0, len(ref))
		for k := range ref {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		return keys
	}
	check := func(m *aug.MonoidMap[int, int, sumStats], ref map[int]int) {
		t.Helper()
		if err := m.Verify(); err != nil {
			t.Fatal(err)
		}
		keys := sortedKeys(ref)
		if m.Len() != len(keys) {
			t.Fatalf("len %d, want %d", m.Len(), len(keys))
		}
		it := m.Iterator()
		i := 0
		for it.First(); it.Valid(); it.Next() {
			if it.Key() != keys[i] || it.Value() != ref[keys[i]] {
				t.Fatalf("position %d: (%d, %d), want (%d, %d)", i, it.Key(), it.Value(), keys[i], ref[keys[i]])
			}
			i++
		}
		var want sumStats
		for k, v := range ref {
			_ = k
			want.A++
			want.B += v
		}
		if got := m.Total(); got != want {
			t.Fatalf("Total %v, want %v", got, want)
		}
	}
	type snapshot struct {
		m   *aug.MonoidMap[int, int, sumStats]
		ref map[int]int
	}
	var snapshots []snapshot
	for i := range 500 {
		m.Upsert(i*2, i)
		ref[i*2] = i
	}
	c := m.Cursor()
	c.First()
	// expected cursor key, or -1 when the cursor should be invalid.
	expectKey := func(want int, what string) {
		t.Helper()
		if want == -1 {
			if c.Valid() {
				t.Fatalf("%s: cursor valid at %d, want invalid", what, c.Key())
			}
			return
		}
		if !c.Valid() || c.Key() != want {
			t.Fatalf("%s: cursor at valid=%v %d, want %d", what, c.Valid(), c.Key(), want)
		}
	}
	successor := func(keys []int, k int) int {
		i, _ := slices.BinarySearch(keys, k)
		if i < len(keys) {
			return keys[i]
		}
		return -1
	}
	for step := range 40000 {
		keys := sortedKeys(ref)
		switch op := rng.IntN(100); {
		case op < 10:
			if len(snapshots) < 3 {
				cr := make(map[int]int, len(ref))
				maps.Copy(cr, ref)
				snapshots = append(snapshots, snapshot{m.Clone(), cr})
			}
			// Cloning is a write to m; the cursor must be re-positioned.
			c.SeekGE(rng.IntN(1200))
		case op < 25:
			k := rng.IntN(1200)
			c.SeekGE(k)
			expectKey(successor(keys, k), "SeekGE")
		case op < 40:
			if !c.Valid() {
				continue
			}
			v := rng.IntN(1000)
			c.SetValue(v)
			ref[c.Key()] = v
		case op < 55:
			if !c.Valid() {
				continue
			}
			k := c.Key()
			gotK, gotV := c.Delete()
			if gotK != k || gotV != ref[k] {
				t.Fatalf("step %d: Delete returned (%d, %d), want (%d, %d)", step, gotK, gotV, k, ref[k])
			}
			delete(ref, k)
			expectKey(successor(keys, k+1), "Delete")
		case op < 75:
			if !c.Valid() {
				continue
			}
			old := c.Key()
			v := ref[old]
			var k int
			if rng.IntN(2) == 0 {
				k = max(0, old+rng.IntN(5)-2) // near: often in place
			} else {
				k = rng.IntN(1200) // far
			}
			c.Rekey(k)
			delete(ref, old)
			ref[k] = v
			expectKey(k, "Rekey")
		case op < 90:
			k := rng.IntN(1200)
			v := rng.IntN(1000)
			pv, had := ref[k]
			gotV, replaced := c.Upsert(k, v)
			if replaced != had || (had && gotV != pv) {
				t.Fatalf("step %d: Upsert(%d) = (%d, %v), want (%d, %v)", step, k, gotV, replaced, pv, had)
			}
			ref[k] = v
			expectKey(k, "Upsert")
		case op < 95:
			if c.Valid() {
				k := c.Key()
				c.Next()
				expectKey(successor(keys, k+1), "Next")
			} else {
				c.First()
				if len(keys) > 0 {
					expectKey(keys[0], "First")
				}
			}
		default:
			if c.Valid() {
				k := c.Key()
				c.Prev()
				i, _ := slices.BinarySearch(keys, k)
				want := -1
				if i > 0 {
					want = keys[i-1]
				}
				expectKey(want, "Prev")
			}
		}
		if step%1000 == 999 {
			check(m, ref)
			for _, s := range snapshots {
				check(s.m, s.ref)
			}
			c.SeekGE(rng.IntN(1200))
		}
	}
	check(m, ref)
	for _, s := range snapshots {
		check(s.m, s.ref)
		s.m.Clear()
	}
	check(m, ref)
}

func TestCursorEmptiesAndRefills(t *testing.T) {
	m := newSumMap(16)
	for i := range 100 {
		m.Upsert(i, i)
	}
	c := m.Cursor()
	c.First()
	for c.Valid() {
		c.Delete()
	}
	if m.Len() != 0 || m.Height() != 0 {
		t.Fatalf("len %d height %d", m.Len(), m.Height())
	}
	if err := m.Verify(); err != nil {
		t.Fatal(err)
	}
	for i := range 100 {
		c.Upsert(i, i)
		if !c.Valid() || c.Key() != i {
			t.Fatalf("after Upsert(%d) cursor at valid=%v %d", i, c.Valid(), c.Key())
		}
	}
	if err := m.Verify(); err != nil {
		t.Fatal(err)
	}
	if m.Len() != 100 {
		t.Fatalf("len %d", m.Len())
	}
}

type fitKey struct{ pool, fullness, node uint64 }

func cmpFit(a, b fitKey) int {
	if c := cmp.Compare(a.pool, b.pool); c != 0 {
		return c
	}
	if c := cmp.Compare(a.fullness, b.fullness); c != 0 {
		return c
	}
	return cmp.Compare(a.node, b.node)
}

// BenchmarkRekey compares moving an entry to a nearby key through a Cursor
// with the seek, delete and upsert sequence it replaces, on a snapshot that
// is re-cloned every 200 moves as a scheduler round would.
func BenchmarkRekey(b *testing.B) {
	const fleet = 20000
	const perRound = 200
	rng := rand.New(rand.NewPCG(3, 5))
	keys := make([]fitKey, fleet)
	fl := aug.NewFreeList[fitKey, int, int](2048)
	base := aug.NewMonoid[fitKey, int, int](cmpFit, aug.Count[fitKey, int]{}, aug.WithFreeList(fl))
	for i := range keys {
		keys[i] = fitKey{pool: uint64(i % 100), fullness: uint64(rng.IntN(1000)), node: uint64(i)}
		base.Upsert(keys[i], i)
	}
	picks := rng.Perm(fleet)
	for _, name := range []string{"seek+delete+upsert", "cursor.Rekey"} {
		b.Run(name, func(b *testing.B) {
			var m *aug.MonoidMap[fitKey, int, int]
			i := 0
			b.ReportAllocs()
			for b.Loop() {
				if i%perRound == 0 {
					if m != nil {
						m.Clear()
					}
					m = base.Clone()
				}
				h := picks[i%fleet]
				k := keys[h]
				nk := k
				nk.fullness += 3
				switch name {
				case "seek+delete+upsert":
					it := m.Iterator()
					it.SeekGE(k)
					v := it.Value()
					m.Delete(k)
					m.Upsert(nk, v)
				case "cursor.Rekey":
					c := m.Cursor()
					c.SeekGE(k)
					c.Rekey(nk)
				}
				i++
			}
		})
	}
}

// FuzzCursor interprets the input as a script of cursor operations against
// a model, with snapshots taken along the way.
func FuzzCursor(f *testing.F) {
	f.Add([]byte{1, 0, 5, 1, 7, 4, 9, 2, 3, 8, 1, 2, 6, 0})
	f.Add([]byte{0})
	f.Fuzz(func(t *testing.T, script []byte) {
		if len(script) == 0 {
			return
		}
		degree := 2 + int(script[0]%8)
		script = script[1:]
		m := newSumMap(degree)
		ref := map[int]int{}
		type snapshot struct {
			m   *aug.MonoidMap[int, int, sumStats]
			ref map[int]int
		}
		var snaps []snapshot
		c := m.Cursor()
		for i := 0; i+1 < len(script); i += 2 {
			op, k := script[i]&7, int(script[i+1])
			switch op {
			case 0:
				c.SeekGE(k)
			case 1:
				if c.Valid() {
					c.SetValue(k)
					ref[c.Key()] = k
				}
			case 2:
				if c.Valid() {
					key, _ := c.Delete()
					delete(ref, key)
				}
			case 3:
				if c.Valid() {
					old := c.Key()
					v := ref[old]
					c.Rekey(k)
					delete(ref, old)
					ref[k] = v
					if !c.Valid() || c.Key() != k {
						t.Fatalf("after Rekey(%d) cursor at valid=%v %d", k, c.Valid(), c.Key())
					}
				}
			case 4:
				c.Upsert(k, i)
				ref[k] = i
				if !c.Valid() || c.Key() != k {
					t.Fatalf("after Upsert(%d) cursor at valid=%v %d", k, c.Valid(), c.Key())
				}
			case 5:
				c.Next()
			case 6:
				c.Prev()
			case 7:
				if len(snaps) < 3 {
					cr := make(map[int]int, len(ref))
					maps.Copy(cr, ref)
					snaps = append(snaps, snapshot{m.Clone(), cr})
					c.SeekGE(k)
				}
			}
		}
		verify := func(m *aug.MonoidMap[int, int, sumStats], ref map[int]int) {
			t.Helper()
			if err := m.Verify(); err != nil {
				t.Fatal(err)
			}
			if m.Len() != len(ref) {
				t.Fatalf("len %d, want %d", m.Len(), len(ref))
			}
			var want sumStats
			for k, v := range ref {
				got, ok := m.Get(k)
				if !ok || got != v {
					t.Fatalf("Get(%d) = (%d, %v), want %d", k, got, ok, v)
				}
				want.A++
				want.B += v
			}
			if m.Total() != want {
				t.Fatalf("Total %v, want %v", m.Total(), want)
			}
		}
		verify(m, ref)
		for _, s := range snaps {
			verify(s.m, s.ref)
			s.m.Clear()
		}
		verify(m, ref)
	})
}

// TestCursorWriterWithSnapshotReaders runs a cursor-driven writer while
// readers walk snapshots it hands out, under the race detector.
func TestCursorWriterWithSnapshotReaders(t *testing.T) {
	m := newSumMap(4)
	for i := range 5000 {
		m.Upsert(i, i)
	}
	snapshots := make(chan *aug.MonoidMap[int, int, sumStats], 8)
	done := make(chan struct{})
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for s := range snapshots {
				prev := -1
				n := 0
				var sum int
				for k, v := range s.All() {
					if k <= prev {
						t.Errorf("snapshot out of order at %d after %d", k, prev)
						return
					}
					prev = k
					n++
					sum += v
				}
				if n != s.Len() || s.Total() != (sumStats{A: n, B: sum}) {
					t.Errorf("snapshot inconsistent: %d items, total %v", n, s.Total())
					return
				}
				s.Clear()
			}
		})
	}
	go func() {
		defer close(done)
		rng := rand.New(rand.NewPCG(9, 9))
		c := m.Cursor()
		for step := range 20000 {
			k := rng.IntN(6000)
			switch rng.IntN(5) {
			case 0:
				c.SeekGE(k)
			case 1:
				if c.Valid() {
					c.SetValue(step)
				}
			case 2:
				if c.Valid() {
					c.Delete()
				}
			case 3:
				if c.Valid() {
					c.Rekey(k)
				}
			case 4:
				c.Upsert(k, step)
			}
			if step%250 == 0 {
				snapshots <- m.Clone()
				c.SeekGE(k)
			}
		}
		close(snapshots)
	}()
	<-done
	wg.Wait()
	if err := m.Verify(); err != nil {
		t.Fatal(err)
	}
}
