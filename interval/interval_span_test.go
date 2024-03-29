// Copyright 2018 The Cockroach Authors.
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

package interval

import (
	"bytes"
	"fmt"
	"math/rand"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/ajwerner/btree/internal/abstract"
	"github.com/ajwerner/btree/internal/ordered"
	"github.com/stretchr/testify/require"
)

type Key []byte

type Span struct {
	key, endKey Key
}

func key(i int) Key {
	if i < 0 || i > 99999 {
		panic("key out of bounds")
	}
	return []byte(fmt.Sprintf("%05d", i))
}

func (k Key) Next() Key {
	return Key(BytesNext(k))
}

// BytesNext returns the next possible byte slice, using the extra capacity
// of the provided slice if possible, and if not, appending an \x00.
func BytesNext(b []byte) []byte {
	if cap(b) > len(b) {
		bNext := b[:len(b)+1]
		if bNext[len(bNext)-1] == 0 {
			return bNext
		}
	}
	// TODO(spencer): Do we need to enforce KeyMaxLength here?
	// Switched to "make and copy" pattern in #4963 for performance.
	bn := make([]byte, len(b)+1)
	copy(bn, b)
	bn[len(bn)-1] = 0
	return bn
}

func span(i int) Span {
	switch i % 10 {
	case 0:
		return Span{key: key(i)}
	case 1:
		return Span{key: key(i), endKey: key(i).Next()}
	case 2:
		return Span{key: key(i), endKey: key(i + 64)}
	default:
		return Span{key: key(i), endKey: key(i + 4)}
	}
}

func spanWithEnd(start, end int) Span {
	if start < end {
		return Span{key: key(start), endKey: key(end)}
	} else if start == end {
		return Span{key: key(start)}
	} else {
		panic("illegal span")
	}
}

func spanWithMemo(i int, memo map[int]Span) Span {
	if s, ok := memo[i]; ok {
		return s
	}
	s := span(i)
	memo[i] = s
	return s
}

func randomSpan(rng *rand.Rand, n int) Span {
	start := rng.Intn(n)
	end := rng.Intn(n + 1)
	if end < start {
		start, end = end, start
	}
	return spanWithEnd(start, end)
}

func newLatch(s Span) *latch {
	return &latch{span: s}
}

type latch struct {
	span Span
	id   int
}

func (l *latch) Key() Key {
	return l.span.key
}

func (l *latch) End() Key {
	if l.span.endKey == nil {
		return l.span.key
	}
	return l.span.endKey
}

func (sp Span) Equal(other Span) bool {
	if bytes.Compare(sp.key, other.key) == 0 {
		return bytes.Compare(sp.endKey, other.endKey) == 0
	}
	return false
}

type iterator = Iterator[*latch, Key, struct{}]

func checkIter(t *testing.T, it iterator, start, end int, spanMemo map[int]Span) {
	i := start
	for it.First(); it.Valid(); it.Next() {
		la := it.Cur()
		expected := spanWithMemo(i, spanMemo)
		if !expected.Equal(la.span) {
			t.Fatalf("expected %s, but found %s", expected, la.span)
		}
		i++
	}
	if i != end {
		t.Fatalf("expected %d, but at %d", end, i)
	}

	for it.Last(); it.Valid(); it.Prev() {
		i--
		la := it.Cur()
		expected := spanWithMemo(i, spanMemo)
		if !expected.Equal(la.span) {
			t.Fatalf("expected %s, but found %s", expected, la.span)
		}
	}
	if i != start {
		t.Fatalf("expected %d, but at %d: %+v", start, i, it)
	}

	all := newLatch(spanWithEnd(start, end))
	for it.FirstOverlap(all); it.Valid(); it.NextOverlap() {
		la := it.Cur()
		expected := spanWithMemo(i, spanMemo)
		if !expected.Equal(la.span) {
			t.Fatalf("expected %s, but found %s", expected, la.span)
		}
		i++
	}
	if i != end {
		t.Fatalf("expected %d, but at %d", end, i)
	}
}

func compareLatches(a, b *latch) int {
	if c := a.span.key.Compare(b.span.key); c != 0 {
		return c
	}
	if c := a.span.endKey.Compare(b.span.endKey); c != 0 {
		return c
	}
	return ordered.Compare(a.id, b.id)
}

func (k Key) Compare(o Key) int {
	return bytes.Compare(k, o)
}

type btree = Map[*latch, Key, struct{}]

func makeBTree() btree {
	return MakeMap[*latch, Key, struct{}](
		Key.Compare,
		compareLatches,
		func(l *latch) Key { return l.span.key },
		func(l *latch) Key { return l.span.endKey },
		func(l *latch) bool { return len(l.span.endKey) > 0 },
	)
}

func TestBTree(t *testing.T) {
	tr := makeBTree()
	spanMemo := make(map[int]Span)

	// With degree == 16 (max-items/node == 31) we need 513 items in order for
	// there to be 3 levels in the tree. The count here is comfortably above
	// that.
	const count = 768

	// Add keys in sorted order.
	for i := 0; i < count; i++ {
		tr.Upsert(newLatch(span(i)), struct{}{})
		//tr.Verify(t)
		if e := i + 1; e != tr.Len() {
			t.Fatalf("expected length %d, but found %d", e, tr.Len())
		}
		checkIter(t, tr.Iterator(), 0, i+1, spanMemo)
	}

	// Delete keys in sorted order.
	for i := 0; i < count; i++ {
		tr.Delete(newLatch(span(i)))
		//tr.Verify(t)
		if e := count - (i + 1); e != tr.Len() {
			t.Fatalf("expected length %d, but found %d", e, tr.Len())
		}
		checkIter(t, tr.Iterator(), i+1, count, spanMemo)
	}

	// Add keys in reverse sorted order.
	for i := 0; i < count; i++ {
		tr.Upsert(newLatch(span(count-i)), struct{}{})
		//tr.Verify(t)
		if e := i + 1; e != tr.Len() {
			t.Fatalf("expected length %d, but found %d", e, tr.Len())
		}
		checkIter(t, tr.Iterator(), count-i, count+1, spanMemo)
	}

	// Delete keys in reverse sorted order.
	for i := 0; i < count; i++ {
		tr.Delete(newLatch(span(count - i)))
		//tr.Verify(t)
		if e := count - (i + 1); e != tr.Len() {
			t.Fatalf("expected length %d, but found %d", e, tr.Len())
		}
		checkIter(t, tr.Iterator(), 1, count-i, spanMemo)
	}
}

func TestBTreeSeek(t *testing.T) {
	const count = 513

	tr := makeBTree()
	for i := 0; i < count; i++ {
		tr.Upsert(newLatch(span(i*2)), struct{}{})
	}

	it := tr.Iterator()
	for i := 0; i < 2*count-1; i++ {
		it.SeekGE(newLatch(span(i)))
		if !it.Valid() {
			t.Fatalf("%d: expected valid iterator", i)
		}
		la := it.Cur()
		expected := span(2 * ((i + 1) / 2))
		if !expected.Equal(la.span) {
			t.Fatalf("%d: expected %s, but found %s", i, expected, la.span)
		}
	}
	it.SeekGE(newLatch(span(2*count - 1)))
	if it.Valid() {
		t.Fatalf("expected invalid iterator")
	}

	for i := 1; i < 2*count; i++ {
		it.SeekLT(newLatch(span(i)))
		if !it.Valid() {
			t.Fatalf("%d: expected valid iterator", i)
		}
		la := it.Cur()
		expected := span(2 * ((i - 1) / 2))
		if !expected.Equal(la.span) {
			t.Fatalf("%d: expected %s, but found %s", i, expected, la.span)
		}
	}
	it.SeekLT(newLatch(span(0)))
	if it.Valid() {
		t.Fatalf("expected invalid iterator")
	}
}

func TestBTreeSeekOverlap(t *testing.T) {
	const count = 513
	const size = 2 * abstract.MaxEntries

	tr := makeBTree()
	for i := 0; i < count; i++ {
		tr.Upsert(newLatch(spanWithEnd(i, i+size+1)), struct{}{})
	}

	// Iterate over overlaps with a point scan.
	it := tr.Iterator()
	for i := 0; i < count+size; i++ {
		it.FirstOverlap(newLatch(spanWithEnd(i, i)))
		for j := 0; j < size+1; j++ {
			expStart := i - size + j
			if expStart < 0 {
				continue
			}
			if expStart >= count {
				continue
			}

			if !it.Valid() {
				t.Fatalf("%d/%d: expected valid iterator", i, j)
			}
			la := it.Cur()
			expected := spanWithEnd(expStart, expStart+size+1)
			if !expected.Equal(la.span) {
				t.Fatalf("%d: expected %s, but found %s", i, expected, la.span)
			}

			it.NextOverlap()
		}
		if it.Valid() {
			t.Fatalf("%d: expected invalid iterator %v", i, it.Cur())
		}
	}
	it.FirstOverlap(newLatch(span(count + size + 1)))
	if it.Valid() {
		t.Fatalf("expected invalid iterator")
	}

	// Iterate over overlaps with a range scan.
	it = tr.Iterator()
	for i := 0; i < count+size; i++ {
		it.FirstOverlap(newLatch(spanWithEnd(i, i+size+1)))
		for j := 0; j < 2*size+1; j++ {
			expStart := i - size + j
			if expStart < 0 {
				continue
			}
			if expStart >= count {
				continue
			}

			if !it.Valid() {
				t.Fatalf("%d/%d: expected valid iterator", i, j)
			}
			la := it.Cur()
			expected := spanWithEnd(expStart, expStart+size+1)
			if !expected.Equal(la.span) {
				t.Fatalf("%d: expected %s, but found %s", i, expected, la.span)
			}

			it.NextOverlap()
		}
		if it.Valid() {
			t.Fatalf("%d: expected invalid iterator %v", i, it.Cur())
		}
	}
	it.FirstOverlap(newLatch(span(count + size + 1)))
	if it.Valid() {
		t.Fatalf("expected invalid iterator")
	}
}

func TestBTreeSeekOverlapRandom(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	const trials = 10
	for i := 0; i < trials; i++ {
		tr := makeBTree()

		const count = 1000
		latches := make([]*latch, count)
		latchSpans := make([]int, count)
		for j := 0; j < count; j++ {
			var la *latch
			end := rng.Intn(count + 10)
			if end <= j {
				end = j
				la = newLatch(spanWithEnd(j, end))
			} else {
				la = newLatch(spanWithEnd(j, end+1))
			}
			tr.Upsert(la, struct{}{})
			latches[j] = la
			latchSpans[j] = end
		}

		const scanTrials = 100
		for j := 0; j < scanTrials; j++ {
			var scanLa *latch
			scanStart := rng.Intn(count)
			scanEnd := rng.Intn(count + 10)
			if scanEnd <= scanStart {
				scanEnd = scanStart
				scanLa = newLatch(spanWithEnd(scanStart, scanEnd))
			} else {
				scanLa = newLatch(spanWithEnd(scanStart, scanEnd+1))
			}

			var exp, found []*latch
			for startKey, endKey := range latchSpans {
				if startKey <= scanEnd && endKey >= scanStart {
					exp = append(exp, latches[startKey])
				}
			}

			it := tr.Iterator()
			it.FirstOverlap(scanLa)
			for it.Valid() {
				found = append(found, it.Cur())
				it.NextOverlap()
			}

			require.Equal(t, len(exp), len(found), "search for %v", scanLa.span)
		}
	}
}

func TestBTreeCloneConcurrentOperations(t *testing.T) {
	const cloneTestSize = 1000
	p := perm(cloneTestSize)

	var trees []*btree
	treeC, treeDone := make(chan *btree), make(chan struct{})
	go func() {
		for b := range treeC {
			trees = append(trees, b)
		}
		close(treeDone)
	}()

	var wg sync.WaitGroup
	var populate func(tr *btree, start int)
	populate = func(tr *btree, start int) {
		t.Logf("Starting new clone at %v", start)
		treeC <- tr
		for i := start; i < cloneTestSize; i++ {
			tr.Upsert(p[i], struct{}{})
			if i%(cloneTestSize/5) == 0 {
				wg.Add(1)
				c := tr.Clone()
				go populate(&c, i+1)
			}
		}
		wg.Done()
	}

	wg.Add(1)
	tr := makeBTree()
	go populate(&tr, 0)
	wg.Wait()
	close(treeC)
	<-treeDone

	t.Logf("Starting equality checks on %d trees", len(trees))
	want := rang(0, cloneTestSize-1)
	for i, tree := range trees {
		if !reflect.DeepEqual(want, all(tree)) {
			t.Errorf("tree %v mismatch", i)
		}
	}

	t.Log("Removing half of latches from first half")
	toRemove := want[cloneTestSize/2:]
	for i := 0; i < len(trees)/2; i++ {
		tree := trees[i]
		wg.Add(1)
		go func() {
			for _, la := range toRemove {
				tree.Delete(la)
			}
			wg.Done()
		}()
	}
	wg.Wait()

	t.Log("Checking all values again")
	for i, tree := range trees {
		var wantpart []*latch
		if i < len(trees)/2 {
			wantpart = want[:cloneTestSize/2]
		} else {
			wantpart = want
		}
		if got := all(tree); !reflect.DeepEqual(wantpart, got) {
			t.Errorf("tree %v mismatch, want %v got %v", i, len(want), len(got))
		}
	}
}

// perm returns a random permutation of latches with spans in the range [0, n).
func perm(n int) (out []*latch) {
	for _, i := range rand.Perm(n) {
		out = append(out, newLatch(spanWithEnd(i, i+1)))
	}
	return out
}

// rang returns an ordered list of latches with spans in the range [m, n].
func rang(m, n int) (out []*latch) {
	for i := m; i <= n; i++ {
		out = append(out, newLatch(spanWithEnd(i, i+1)))
	}
	return out
}

// all extracts all latches from a tree in order as a slice.
func all(tr *Map[*latch, Key, struct{}]) (out []*latch) {
	it := tr.Iterator()
	it.First()
	for it.Valid() {
		out = append(out, it.Cur())
		it.Next()
	}
	return out
}

func forBenchmarkSizes(b *testing.B, f func(b *testing.B, count int)) {
	for _, count := range []int{16, 128, 1024, 8192, 65536} {
		b.Run(fmt.Sprintf("count=%d", count), func(b *testing.B) {
			f(b, count)
		})
	}
}

func BenchmarkBTreeInsert(b *testing.B) {
	forBenchmarkSizes(b, func(b *testing.B, count int) {
		insertP := perm(count)
		b.ResetTimer()
		for i := 0; i < b.N; {
			tr := makeBTree()
			for _, la := range insertP {
				tr.Upsert(la, struct{}{})
				i++
				if i >= b.N {
					return
				}
			}
		}
	})
}

func BenchmarkBTreeDelete(b *testing.B) {
	forBenchmarkSizes(b, func(b *testing.B, count int) {
		insertP, removeP := perm(count), perm(count)
		b.ResetTimer()
		for i := 0; i < b.N; {
			b.StopTimer()
			tr := makeBTree()
			for _, la := range insertP {
				tr.Upsert(la, struct{}{})
			}
			b.StartTimer()
			for _, la := range removeP {
				tr.Delete(la)
				i++
				if i >= b.N {
					return
				}
			}
			if tr.Len() > 0 {
				b.Fatalf("tree not empty: %s", &tr)
			}
		}
	})
}

func BenchmarkBTreeDeleteInsert(b *testing.B) {
	forBenchmarkSizes(b, func(b *testing.B, count int) {
		insertP := perm(count)
		tr := makeBTree()
		for _, la := range insertP {
			tr.Upsert(la, struct{}{})
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			la := insertP[i%count]
			tr.Delete(la)
			tr.Upsert(la, struct{}{})
		}
	})
}

func BenchmarkBTreeDeleteInsertCloneOnce(b *testing.B) {
	forBenchmarkSizes(b, func(b *testing.B, count int) {
		insertP := perm(count)
		tr := makeBTree()
		for _, la := range insertP {
			tr.Upsert(la, struct{}{})
		}
		tr = tr.Clone()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			la := insertP[i%count]
			tr.Delete(la)
			tr.Upsert(la, struct{}{})
		}
	})
}

func BenchmarkBTreeDeleteInsertCloneEachTime(b *testing.B) {
	for _, reset := range []bool{false, true} {
		b.Run(fmt.Sprintf("reset=%t", reset), func(b *testing.B) {
			forBenchmarkSizes(b, func(b *testing.B, count int) {
				insertP := perm(count)
				tr, trReset := makeBTree(), makeBTree()
				for _, la := range insertP {
					tr.Upsert(la, struct{}{})
				}
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					la := insertP[i%count]
					if reset {
						trReset.Reset()
						trReset = tr
					}
					tr = tr.Clone()
					tr.Delete(la)
					tr.Upsert(la, struct{}{})
				}
			})
		})
	}
}

func BenchmarkBTreeMakeIter(b *testing.B) {
	tr := makeBTree()
	for i := 0; i < b.N; i++ {
		it := tr.Iterator()
		it.First()
	}
}

func BenchmarkBTreeIterSeekGE(b *testing.B) {
	forBenchmarkSizes(b, func(b *testing.B, count int) {
		var spans []Span
		tr := makeBTree()

		for i := 0; i < count; i++ {
			s := span(i)
			spans = append(spans, s)
			tr.Upsert(newLatch(s), struct{}{})
		}

		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		it := tr.Iterator()

		var l latch
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			s := spans[rng.Intn(len(spans))]
			l.span = s
			it.SeekGE(&l)
			if testing.Verbose() {
				if !it.Valid() {
					b.Fatal("expected to find key")
				}
				if !s.Equal(it.Cur().span) {
					b.Fatalf("expected %s, but found %s", s, it.Cur().span)
				}
			}
		}
	})
}

func BenchmarkBTreeIterSeekLT(b *testing.B) {
	forBenchmarkSizes(b, func(b *testing.B, count int) {
		var spans []Span
		tr := makeBTree()

		for i := 0; i < count; i++ {
			s := span(i)
			spans = append(spans, s)
			tr.Upsert(newLatch(s), struct{}{})
		}

		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		it := tr.Iterator()

		var l latch
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			j := rng.Intn(len(spans))
			s := spans[j]
			l.span = s
			it.SeekLT(&l)
			if testing.Verbose() {
				if j == 0 {
					if it.Valid() {
						b.Fatal("unexpected key")
					}
				} else {
					if !it.Valid() {
						b.Fatal("expected to find key")
					}
					s := spans[j-1]
					if !s.Equal(it.Cur().span) {
						b.Fatalf("expected %s, but found %s", s, it.Cur().span)
					}
				}
			}
		}
	})
}

func BenchmarkBTreeIterFirstOverlap(b *testing.B) {
	forBenchmarkSizes(b, func(b *testing.B, count int) {
		var spans []Span
		var latches []*latch
		tr := makeBTree()

		for i := 0; i < count; i++ {
			s := spanWithEnd(i, i+1)
			spans = append(spans, s)
			la := newLatch(s)
			latches = append(latches, la)
			tr.Upsert(la, struct{}{})
		}

		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		it := tr.Iterator()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			j := rng.Intn(len(spans))
			s := spans[j]
			la := latches[j]
			it.FirstOverlap(la)
			if testing.Verbose() {
				if !it.Valid() {
					b.Fatal("expected to find key")
				}
				if !s.Equal(it.Cur().span) {
					b.Fatalf("expected %s, but found %s", s, it.Cur().span)
				}
			}
		}
	})
}

func BenchmarkBTreeIterNext(b *testing.B) {
	tr := makeBTree()

	const count = 8 << 10
	const size = 2 * abstract.MaxEntries
	for i := 0; i < count; i++ {
		la := newLatch(spanWithEnd(i, i+size+1))
		tr.Upsert(la, struct{}{})
	}

	it := tr.Iterator()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !it.Valid() {
			it.First()
		}
		it.Next()
	}
}

func BenchmarkBTreeIterPrev(b *testing.B) {
	tr := makeBTree()

	const count = 8 << 10
	const size = 2 * abstract.MaxEntries
	for i := 0; i < count; i++ {
		la := newLatch(spanWithEnd(i, i+size+1))
		tr.Upsert(la, struct{}{})
	}

	it := tr.Iterator()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !it.Valid() {
			it.First()
		}
		it.Prev()
	}
}

func BenchmarkBTreeIterNextOverlap(b *testing.B) {
	tr := makeBTree()

	const count = 8 << 10
	const size = 2 * abstract.MaxEntries
	for i := 0; i < count; i++ {
		la := newLatch(spanWithEnd(i, i+size+1))
		tr.Upsert(la, struct{}{})
	}

	allCmd := newLatch(spanWithEnd(0, count+1))
	it := tr.Iterator()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !it.Valid() {
			it.FirstOverlap(allCmd)
		}
		it.NextOverlap()
	}
}

func BenchmarkBTreeIterOverlapScan(b *testing.B) {
	tr := makeBTree()
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	const count = 8 << 10
	const size = 2 * abstract.MaxEntries
	for i := 0; i < count; i++ {
		tr.Upsert(newLatch(spanWithEnd(i, i+size+1)), struct{}{})
	}

	la := new(latch)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		la.span = randomSpan(rng, count)
		it := tr.Iterator()
		it.FirstOverlap(la)
		for it.Valid() {
			it.NextOverlap()
		}
	}
}
