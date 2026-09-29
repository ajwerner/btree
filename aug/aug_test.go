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
	"strings"
	"sync"
	"testing"

	"github.com/ajwerner/btree/aug"
)

// countingFreeList wraps a FreeList and counts traffic.
type countingFreeList[K, V, A any] struct {
	aug.FreeList[K, V, A]
	mu        sync.Mutex
	gets, hit int
	puts, ret int
}

func (f *countingFreeList[K, V, A]) Get() *aug.Node[K, V, A] {
	n := f.FreeList.Get()
	f.mu.Lock()
	f.gets++
	if n != nil {
		f.hit++
	}
	f.mu.Unlock()
	return n
}

func (f *countingFreeList[K, V, A]) Put(n *aug.Node[K, V, A]) bool {
	ok := f.FreeList.Put(n)
	f.mu.Lock()
	f.puts++
	if ok {
		f.ret++
	}
	f.mu.Unlock()
	return ok
}

func TestClearRecyclesThroughFreeList(t *testing.T) {
	fl := &countingFreeList[int, int, struct{}]{FreeList: aug.NewFreeList[int, int, struct{}](1000)}
	m := aug.New[int, int, struct{}](cmp.Compare[int], nil, aug.WithFreeList(fl))
	for i := range 10000 {
		m.Upsert(i, i)
	}
	if fl.puts != 0 {
		t.Fatalf("puts before clear: %d", fl.puts)
	}
	c := m.Clone()
	for i := 0; i < 10000; i += 100 {
		c.Upsert(i, -i)
	}
	// Clearing the clone releases only the nodes it copied.
	c.Clear()
	copied := fl.ret
	if copied == 0 {
		t.Fatal("clearing the clone returned nothing")
	}
	if copied > 400 {
		t.Fatalf("clearing the clone returned %d nodes, expected roughly the touched paths", copied)
	}
	if err := m.Verify(); err != nil {
		t.Fatal(err)
	}
	if m.Len() != 10000 {
		t.Fatalf("len %d", m.Len())
	}
	// Clearing the original releases everything else.
	m.Clear()
	if fl.ret == copied {
		t.Fatal("clearing the original returned nothing")
	}
	// Building again reuses the recycled nodes.
	before := fl.hit
	for i := range 10000 {
		m.Upsert(i, i)
	}
	if fl.hit == before {
		t.Fatal("rebuild did not reuse nodes")
	}
	if err := m.Verify(); err != nil {
		t.Fatal(err)
	}
}

func TestFreeListSharedAcrossDegrees(t *testing.T) {
	fl := aug.NewFreeList[int, int, struct{}](64)
	small := aug.New[int, int, struct{}](cmp.Compare[int], nil, aug.WithDegree(2), aug.WithFreeList(fl))
	large := aug.New[int, int, struct{}](cmp.Compare[int], nil, aug.WithDegree(32), aug.WithFreeList(fl))
	for i := range 2000 {
		small.Upsert(i, i)
	}
	small.Clear()
	for i := range 2000 {
		large.Upsert(i, i)
	}
	if err := large.Verify(); err != nil {
		t.Fatal(err)
	}
	large.Clear()
	for i := range 2000 {
		small.Upsert(i, i)
	}
	if err := small.Verify(); err != nil {
		t.Fatal(err)
	}
	if small.Degree() != 2 || large.Degree() != 32 {
		t.Fatalf("degrees %d %d", small.Degree(), large.Degree())
	}
}

func TestSyncPoolFreeList(t *testing.T) {
	fl := aug.SyncPoolFreeList[int, string, struct{}]()
	if fl != aug.SyncPoolFreeList[int, string, struct{}]() {
		t.Fatal("expected the same pool for the same instantiation")
	}
	m := aug.New[int, string, struct{}](cmp.Compare[int], nil, aug.WithFreeList(fl))
	for i := range 5000 {
		m.Upsert(i, "x")
	}
	c := m.Clone()
	c.Delete(7)
	m.Clear()
	if v, ok := c.Get(8); !ok || v != "x" {
		t.Fatalf("clone lost data: %q %v", v, ok)
	}
	if err := c.Verify(); err != nil {
		t.Fatal(err)
	}
	c.Clear()
}

func TestBadOptions(t *testing.T) {
	expectPanic := func(name, want string, f func()) {
		t.Helper()
		defer func() {
			r := recover()
			if r == nil {
				t.Fatalf("%s: no panic", name)
			}
			if !strings.Contains(r.(string), want) {
				t.Fatalf("%s: panic %q does not mention %q", name, r, want)
			}
		}()
		f()
	}
	expectPanic("degree", "degree", func() {
		aug.New[int, int, struct{}](cmp.Compare[int], nil, aug.WithDegree(1))
	})
	expectPanic("freelist", "WithFreeList", func() {
		fl := aug.NewFreeList[int, string, struct{}](1)
		aug.New[int, int, struct{}](cmp.Compare[int], nil, aug.WithFreeList(fl))
	})
}

func TestConcurrentReadersOnSnapshot(t *testing.T) {
	m := aug.New[int, int, struct{}](cmp.Compare[int], nil)
	for i := range 20000 {
		m.Upsert(i, i)
	}
	snap := m.Clone()
	var wg sync.WaitGroup
	for r := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			it := snap.Iterator()
			n := 0
			for it.First(); it.Valid(); it.Next() {
				if it.Cur() != n || it.Value() != n {
					t.Errorf("reader %d: position %d holds (%d, %d)", r, n, it.Cur(), it.Value())
					return
				}
				n++
			}
			if n != 20000 {
				t.Errorf("reader %d: saw %d items", r, n)
			}
		}()
	}
	// Meanwhile the writer keeps mutating its own tree, which shares nodes
	// with the snapshot.
	for i := 0; i < 20000; i += 3 {
		m.Delete(i)
		m.Upsert(i+100000, i)
	}
	wg.Wait()
	snap.Clear()
	if err := m.Verify(); err != nil {
		t.Fatal(err)
	}
	m.Clear()
}
