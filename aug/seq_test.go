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
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/ajwerner/btree/aug"
)

func TestSeq(t *testing.T) {
	for _, degree := range []int{2, 3, 16} {
		for _, n := range []int{0, 1, 7, 500, 20000} {
			t.Run(fmt.Sprintf("degree=%d/n=%d", degree, n), func(t *testing.T) {
				testSeq(t, degree, n)
			})
		}
	}
}

func testSeq(t *testing.T, degree, n int) {
	rng := rand.New(rand.NewPCG(uint64(degree), uint64(n)))
	m := aug.New[int, int, struct{}](cmp.Compare[int], nil, aug.WithDegree(degree))
	keys := make([]int, n)
	for i := range keys {
		keys[i] = 3 * i
	}
	for _, i := range rng.Perm(n) {
		m.Upsert(keys[i], -keys[i])
	}
	var got []int
	for k, v := range m.All() {
		if v != -k {
			t.Fatalf("All: (%d, %d)", k, v)
		}
		got = append(got, k)
	}
	if !slices.Equal(got, keys) {
		t.Fatalf("All: got %d keys, want %d", len(got), len(keys))
	}
	got = got[:0]
	for k := range m.Backward() {
		got = append(got, k)
	}
	slices.Reverse(got)
	if !slices.Equal(got, keys) {
		t.Fatalf("Backward: got %d keys, want %d", len(got), len(keys))
	}
	// Early exit.
	count := 0
	for range m.All() {
		count++
		if count == 5 {
			break
		}
	}
	if want := min(n, 5); count != want {
		t.Fatalf("early exit visited %d, want %d", count, want)
	}
	for range 50 {
		lo, hi := rng.IntN(3*n+10)-5, rng.IntN(3*n+10)-5
		got = got[:0]
		for k := range m.Range(lo, hi) {
			got = append(got, k)
		}
		i, _ := slices.BinarySearch(keys, lo)
		j, _ := slices.BinarySearch(keys, hi)
		want := []int{}
		if i < j {
			want = keys[i:j]
		}
		if !slices.Equal(got, want) {
			t.Fatalf("Range(%d, %d): got %v, want %v", lo, hi, got, want)
		}
		got = got[:0]
		for k := range m.From(lo) {
			got = append(got, k)
		}
		if !slices.Equal(got, keys[i:]) {
			t.Fatalf("From(%d): got %d keys, want %d", lo, len(got), len(keys[i:]))
		}
	}
	if k, _, ok := m.Min(); ok != (n > 0) || (ok && k != keys[0]) {
		t.Fatalf("Min = %d %v", k, ok)
	}
	if k, _, ok := m.Max(); ok != (n > 0) || (ok && k != keys[n-1]) {
		t.Fatalf("Max = %d %v", k, ok)
	}
}

func BenchmarkIterateAll(b *testing.B) {
	const n = 1_000_000
	m := aug.New[int, int, struct{}](cmp.Compare[int], nil)
	rng := rand.New(rand.NewPCG(1, 2))
	for _, k := range rng.Perm(n) {
		m.Upsert(k, k)
	}
	b.Run("Iterator", func(b *testing.B) {
		it := m.Iterator()
		for b.Loop() {
			for it.First(); it.Valid(); it.Next() {
			}
		}
		b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/n, "ns/item")
	})
	b.Run("All", func(b *testing.B) {
		for b.Loop() {
			for range m.All() {
			}
		}
		b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/n, "ns/item")
	})
	b.Run("Range/10", func(b *testing.B) {
		i := 0
		for b.Loop() {
			c := 0
			for range m.Range(i%n, i%n+10) {
				c++
			}
			i++
		}
	})
	b.Run("Iterator/SeekGE+10", func(b *testing.B) {
		it := m.Iterator()
		i := 0
		for b.Loop() {
			c := 0
			for it.SeekGE(i % n); it.Valid() && c < 10; it.Next() {
				c++
			}
			i++
		}
	})
}
