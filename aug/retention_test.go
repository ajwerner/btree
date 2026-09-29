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
	"runtime"
	"strings"
	"testing"
	"weak"

	"github.com/ajwerner/btree/aug"
)

type payload struct{ buf [64]byte }

// TestNoStaleReferences checks that values removed through every path
// (top-down Delete, Cursor.Delete, replacement, Clear of a clone and of
// the original) become unreachable, i.e. that no stale slot in a live node
// or in a free-listed node keeps them alive.
func TestNoStaleReferences(t *testing.T) {
	for _, degree := range []int{2, 16} {
		t.Run(fmt.Sprintf("degree=%d", degree), func(t *testing.T) {
			testNoStaleReferences(t, degree)
		})
	}
}

func testNoStaleReferences(t *testing.T, degree int) {
	const n = 3000
	m := aug.NewMonoid[string, *payload, int](strings.Compare, aug.Count[string, *payload]{}, aug.WithDegree(degree))
	weaks := make([]weak.Pointer[payload], n)
	keys := make([]string, n)
	for i := range n {
		keys[i] = fmt.Sprintf("key-%06d", i)
		p := &payload{}
		p.buf[0] = byte(i)
		weaks[i] = weak.Make(p)
		m.Upsert(keys[i], p)
	}
	gone := func(i int) bool { return weaks[i].Value() == nil }
	collect := func() {
		runtime.GC()
		runtime.GC()
	}
	// Everything is reachable through the map.
	collect()
	for i := range n {
		if gone(i) {
			t.Fatalf("value %d collected while in the map", i)
		}
	}
	snap := m.Clone()
	// Remove a third top-down, a third through a cursor, replace the rest.
	rng := rand.New(rand.NewPCG(1, 2))
	perm := rng.Perm(n)
	deleted, cursored, replaced := perm[:n/3], perm[n/3:2*n/3], perm[2*n/3:]
	for _, i := range deleted {
		m.Delete(keys[i])
	}
	c := m.Cursor()
	for _, i := range cursored {
		c.SeekGE(keys[i])
		c.Delete()
	}
	for _, i := range replaced {
		m.Upsert(keys[i], &payload{})
	}
	collect()
	// The snapshot still references every original value.
	for i := range n {
		if gone(i) {
			t.Fatalf("value %d collected while the snapshot holds it", i)
		}
	}
	snap.Clear()
	collect()
	for _, i := range deleted {
		if !gone(i) {
			t.Fatalf("value %d deleted top-down is still reachable", i)
		}
	}
	for _, i := range cursored {
		if !gone(i) {
			t.Fatalf("value %d deleted through a cursor is still reachable", i)
		}
	}
	for _, i := range replaced {
		if !gone(i) {
			t.Fatalf("replaced value %d is still reachable", i)
		}
	}
	if want := n - len(deleted) - len(cursored); m.Len() != want {
		t.Fatalf("len %d, want %d", m.Len(), want)
	}
	if err := m.Verify(); err != nil {
		t.Fatal(err)
	}
	m.Clear()
	collect()
	for i := range n {
		if !gone(i) {
			t.Fatalf("value %d still reachable after Clear", i)
		}
	}
}

// TestPointerTypesUnderGCPressure runs a mixed workload with pointer keys
// and values while the collector runs constantly, under the race detector
// when enabled.
func TestPointerTypesUnderGCPressure(t *testing.T) {
	m := aug.NewMonoid[string, *payload, int](strings.Compare, aug.Count[string, *payload]{}, aug.WithDegree(3))
	ref := map[string]*payload{}
	rng := rand.New(rand.NewPCG(3, 4))
	var clones []*aug.MonoidMap[string, *payload, int]
	c := m.Cursor()
	for step := range 30000 {
		k := fmt.Sprintf("k%04d", rng.IntN(2000))
		switch rng.IntN(6) {
		case 0, 1:
			p := &payload{}
			p.buf[1] = byte(step)
			m.Upsert(k, p)
			ref[k] = p
		case 2:
			m.Delete(k)
			delete(ref, k)
		case 3:
			if c.SeekExact(k) {
				c.Delete()
				delete(ref, k)
			}
		case 4:
			if len(clones) < 3 {
				clones = append(clones, m.Clone())
			} else {
				clones[rng.IntN(len(clones))].Clear()
				clones = clones[:0]
			}
			c.Reset()
		case 5:
			runtime.GC()
		}
		if step%5000 == 4999 {
			if err := m.Verify(); err != nil {
				t.Fatal(err)
			}
			if m.Len() != len(ref) {
				t.Fatalf("len %d, want %d", m.Len(), len(ref))
			}
			for k, v := range ref {
				got, ok := m.Get(k)
				if !ok || got != v {
					t.Fatalf("Get(%s) = %p %v, want %p", k, got, ok, v)
				}
			}
		}
	}
}

// TestVerifyWhileCloningAndClearing runs Verify on a snapshot while the
// original is cloned, written and cleared from another goroutine, which
// changes the reference counts of nodes the snapshot shares.
func TestVerifyWhileCloningAndClearing(t *testing.T) {
	m := aug.NewMonoid[int, int, int](cmp.Compare[int], aug.Count[int, int]{}, aug.WithDegree(4))
	for i := range 5000 {
		m.Upsert(i, i)
	}
	snap := m.Clone()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range 300 {
			c := m.Clone()
			c.Upsert(i, -i)
			c.Delete(i + 1)
			c.Clear()
		}
	}()
	for {
		select {
		case <-done:
			if err := snap.Verify(); err != nil {
				t.Fatal(err)
			}
			snap.Clear()
			m.Clear()
			return
		default:
			if err := snap.Verify(); err != nil {
				t.Fatal(err)
			}
		}
	}
}
