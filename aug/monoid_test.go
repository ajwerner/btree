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
	"testing"

	"github.com/ajwerner/btree/aug"
)

// maxOf is a Monoid (not a Group) tracking the largest value.
type maxOf struct{}

func (maxOf) Of(_ int, v int) int  { return v }
func (maxOf) Combine(a, b int) int { return max(a, b) }
func (maxOf) Equal(a, b int) bool  { return a == b }

type stats = aug.Pair[int, aug.Pair[int, int]] // count, (sum, max)

func newStatsMap(degree int) *aug.Map[int, int, stats] {
	m := aug.PairOf[int, int, int, aug.Pair[int, int]](
		aug.Count[int, int]{},
		aug.PairOf[int, int, int, int](aug.Sum(func(_ int, v int) int { return v }), maxOf{}),
	)
	return aug.New[int, int, stats](cmp.Compare[int], aug.MonoidUpdater(m), aug.WithDegree(degree))
}

func statsOf(keys []int, ref map[int]int, lo, hi int) (s stats) {
	for _, k := range keys {
		if k < lo || k >= hi {
			continue
		}
		v := ref[k]
		s.A++
		s.B.A += v
		s.B.B = max(s.B.B, v)
	}
	return s
}

func TestMonoidAggregates(t *testing.T) {
	for _, degree := range []int{2, 3, 16} {
		t.Run(fmt.Sprintf("degree=%d", degree), func(t *testing.T) {
			testMonoidAggregates(t, degree)
		})
	}
}

func testMonoidAggregates(t *testing.T, degree int) {
	seed := rand.Uint64()
	t.Logf("seed %d", seed)
	rng := rand.New(rand.NewPCG(seed, 0))
	m := newStatsMap(degree)
	ref := map[int]int{}
	var clones []*aug.Map[int, int, stats]
	var cloneRefs []map[int]int
	check := func(m *aug.Map[int, int, stats], ref map[int]int) {
		t.Helper()
		if err := m.Verify(); err != nil {
			t.Fatal(err)
		}
		keys := make([]int, 0, len(ref))
		for k := range ref {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		if got, want := m.Total(), statsOf(keys, ref, -1<<31, 1<<31); got != want {
			t.Fatalf("Total() = %v, want %v", got, want)
		}
		for range 20 {
			lo, hi := rng.IntN(1100)-50, rng.IntN(1100)-50
			if got, want := m.Aggregate(lo, hi), statsOf(keys, ref, lo, hi); got != want {
				t.Fatalf("Aggregate(%d, %d) = %v, want %v", lo, hi, got, want)
			}
			got, found := m.Prefix(lo)
			_, refFound := ref[lo]
			if want := statsOf(keys, ref, -1<<31, lo); got != want || found != refFound {
				t.Fatalf("Prefix(%d) = %v %v, want %v %v", lo, got, found, want, refFound)
			}
		}
		// Iterator.Prefix at every position, including past the end.
		it := m.Iterator()
		i := 0
		for it.First(); it.Valid(); it.Next() {
			if got, want := it.Prefix(), statsOf(keys[:i], ref, -1<<31, 1<<31); got != want {
				t.Fatalf("Prefix at %d = %v, want %v", i, got, want)
			}
			i++
		}
		if got, want := it.Prefix(), statsOf(keys, ref, -1<<31, 1<<31); got != want {
			t.Fatalf("Prefix past end = %v, want %v", got, want)
		}
		it.Reset()
		if got := it.Prefix(); got != (stats{}) {
			t.Fatalf("Prefix after Reset = %v", got)
		}
		// SeekWhere by cumulative sum: first entry at which the running sum
		// of values reaches a threshold.
		total := statsOf(keys, ref, -1<<31, 1<<31).B.A
		for range 10 {
			threshold := rng.IntN(total + 2)
			prefix := it.SeekWhere(func(p, c stats) bool { return p.B.A+c.B.A >= threshold })
			running := 0
			wantIdx := -1
			for j, k := range keys {
				running += ref[k]
				if running >= threshold {
					wantIdx = j
					break
				}
			}
			if wantIdx == -1 {
				if it.Valid() {
					t.Fatalf("SeekWhere(sum >= %d) valid at %d, want past end", threshold, it.Key())
				}
				if prefix != statsOf(keys, ref, -1<<31, 1<<31) {
					t.Fatalf("SeekWhere past end returned prefix %v", prefix)
				}
				continue
			}
			if !it.Valid() || it.Key() != keys[wantIdx] {
				t.Fatalf("SeekWhere(sum >= %d) = %v %d, want %d", threshold, it.Valid(), it.Key(), keys[wantIdx])
			}
			if want := statsOf(keys[:wantIdx], ref, -1<<31, 1<<31); prefix != want {
				t.Fatalf("SeekWhere(sum >= %d) prefix %v, want %v", threshold, prefix, want)
			}
		}
	}
	for step := range 30000 {
		k := rng.IntN(1000)
		switch op := rng.IntN(100); {
		case op < 50:
			v := rng.IntN(100)
			m.Upsert(k, v)
			ref[k] = v
		case op < 90:
			m.Delete(k)
			delete(ref, k)
		default:
			if len(clones) < 3 {
				clones = append(clones, m.Clone())
				cr := make(map[int]int, len(ref))
				maps.Copy(cr, ref)
				cloneRefs = append(cloneRefs, cr)
			}
		}
		if step%1500 == 1499 {
			check(m, ref)
			for i := range clones {
				for range 30 {
					k, v := rng.IntN(1000), rng.IntN(100)
					clones[i].Upsert(k, v)
					cloneRefs[i][k] = v
				}
				check(clones[i], cloneRefs[i])
			}
		}
	}
	for _, c := range clones {
		c.Clear()
	}
	check(m, ref)
}

func TestMonoidRequired(t *testing.T) {
	m := aug.New[int, int, struct{}](cmp.Compare[int], nil)
	m.Upsert(1, 1)
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected a panic")
		}
	}()
	m.Prefix(1)
}

// Example_customAugmentation maintains, per subtree, the number of hosts
// and their total load, and answers "how many hosts in pool 2 and how loaded
// are they" and "which host takes the cumulative load past a threshold"
// with one descent each.
func Example_customAugmentation() {
	type key struct{ pool, host int }
	type load = aug.Pair[int, int] // hosts, total load
	compare := func(a, b key) int {
		if c := cmp.Compare(a.pool, b.pool); c != 0 {
			return c
		}
		return cmp.Compare(a.host, b.host)
	}
	m := aug.New[key, int, load](compare, aug.MonoidUpdater(
		aug.PairOf[key, int, int, int](aug.Count[key, int]{}, aug.Sum(func(_ key, v int) int { return v })),
	))
	for host := range 10 {
		m.Upsert(key{pool: host % 3, host: host}, 10*host)
	}
	pool2 := m.Aggregate(key{pool: 2}, key{pool: 3})
	fmt.Println("pool 2:", pool2.A, "hosts,", pool2.B, "load")

	it := m.Iterator()
	it.SeekWhere(func(prefix, c load) bool { return prefix.B+c.B >= 200 })
	fmt.Println("cumulative load reaches 200 at", it.Key())
	// Output:
	// pool 2: 3 hosts, 150 load
	// cumulative load reaches 200 at {1 4}
}
