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

package interval_test

import (
	"cmp"
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/ajwerner/btree/aug"
	"github.com/ajwerner/btree/interval"
)

// span is [lo, hi) when hi > lo and the point lo when hi == lo.
type span struct{ lo, hi int }

func (s span) Key() int { return s.lo }
func (s span) End() int { return s.hi }

func (s span) String() string {
	if s.hi == s.lo {
		return fmt.Sprintf("{%d}", s.lo)
	}
	return fmt.Sprintf("[%d,%d)", s.lo, s.hi)
}

// toSpan is the query for a test span: a point when hi is not after lo.
func toSpan(s span) interval.Span[int] {
	if s.hi > s.lo {
		return interval.HalfOpen(s.lo, s.hi)
	}
	return interval.Point(s.lo)
}

func spanBounds() interval.Bounds[span, int] {
	b := interval.BoundsOf[span](cmp.Compare[int])
	b.HasEnd = func(s span) bool { return s.hi > s.lo }
	return b
}

// overlaps is the reference definition: a range [lo, hi) covers keys lo
// up to but excluding hi; a point covers just its key.
func overlaps(a, b span) bool {
	upper := func(s span) (int, bool) { // bound, inclusive
		if s.hi > s.lo {
			return s.hi, false
		}
		return s.lo, true
	}
	below := func(k int, s span) bool { // k below the upper bound of s
		u, incl := upper(s)
		if incl {
			return k <= u
		}
		return k < u
	}
	return below(a.lo, b) && below(b.lo, a)
}

func compareSpans(a, b span) int {
	if c := cmp.Compare(a.lo, b.lo); c != 0 {
		return c
	}
	return cmp.Compare(a.hi, b.hi)
}

func randomSpan(rng *rand.Rand) span {
	lo := rng.IntN(300) - 100 // negative endpoints too
	switch rng.IntN(4) {
	case 0:
		return span{lo, lo}
	case 1:
		return span{lo, lo + 1 + rng.IntN(3)}
	default:
		return span{lo, lo + 1 + rng.IntN(40)}
	}
}

func TestOverlapProperty(t *testing.T) {
	for _, degree := range []int{2, 3, 16} {
		t.Run(fmt.Sprintf("degree=%d", degree), func(t *testing.T) {
			testOverlapProperty(t, degree)
		})
	}
}

func testOverlapProperty(t *testing.T, degree int) {
	seed := rand.Uint64()
	t.Logf("seed %d", seed)
	rng := rand.New(rand.NewPCG(seed, 0))
	m := interval.NewSet(spanBounds(), aug.WithDegree(degree))
	ref := map[span]struct{}{}
	type snapshot struct {
		m   *interval.Set[span, int]
		ref map[span]struct{}
	}
	var snaps []snapshot
	check := func(m *interval.Set[span, int], ref map[span]struct{}) {
		t.Helper()
		if err := m.Verify(); err != nil {
			t.Fatal(err)
		}
		if m.Len() != len(ref) {
			t.Fatalf("len %d, want %d", m.Len(), len(ref))
		}
		for range 25 {
			q := randomSpan(rng)
			var want []span
			for s := range ref {
				if overlaps(s, q) {
					want = append(want, s)
				}
			}
			slices.SortFunc(want, compareSpans)
			var got []span
			for s := range m.Overlapping(toSpan(q)) {
				got = append(got, s)
			}
			if !slices.Equal(got, want) {
				t.Fatalf("Overlapping(%v):\n got  %v\n want %v", q, got, want)
			}
		}
	}
	for step := range 20000 {
		switch op := rng.IntN(100); {
		case op < 55:
			s := randomSpan(rng)
			m.Upsert(s)
			ref[s] = struct{}{}
		case op < 90:
			if len(ref) > 0 {
				// delete a random existing span, or occasionally a missing one
				var s span
				if rng.IntN(4) == 0 {
					s = randomSpan(rng)
				} else {
					for s = range ref {
						break
					}
				}
				_, removed := m.Delete(s)
				_, had := ref[s]
				if removed != had {
					t.Fatalf("step %d: Delete(%v) = %v, want %v", step, s, removed, had)
				}
				delete(ref, s)
			}
		default:
			if len(snaps) < 3 {
				cr := make(map[span]struct{}, len(ref))
				for s := range ref {
					cr[s] = struct{}{}
				}
				snaps = append(snaps, snapshot{m.Clone(), cr})
			}
		}
		if step%1000 == 999 {
			check(m, ref)
			for _, s := range snaps {
				check(s.m, s.ref)
			}
		}
	}
	for _, s := range snaps {
		s.m.Clear()
	}
	check(m, ref)
}

// TestOverlapIteratorIsIndependent checks that an OverlapIterator and a
// plain Iterator on the same map do not disturb each other, and that an
// empty or reversed query span overlaps nothing.
func TestOverlapIteratorIsIndependent(t *testing.T) {
	m := interval.NewSet(spanBounds())
	for _, s := range []span{{0, 1}, {5, 6}, {10, 11}, {15, 16}} {
		m.Upsert(s)
	}
	ov := m.Overlaps(interval.HalfOpen(10, 16))
	if !ov.Valid() || ov.Item() != (span{10, 11}) {
		t.Fatalf("Overlaps at %v", ov.Item())
	}
	it := m.Iterator()
	it.SeekGE(span{5, 6})
	it.Prev()
	it.Next()
	if !ov.Next() || ov.Item() != (span{15, 16}) {
		t.Fatalf("Next after plain iteration at valid=%v %v", ov.Valid(), ov.Item())
	}
	if ov.Next() || ov.Valid() {
		t.Fatalf("Next past the last overlap is valid at %v", ov.Item())
	}
	if !it.Valid() || it.Item() != (span{5, 6}) {
		t.Fatalf("plain iterator disturbed: valid=%v %v", it.Valid(), it.Item())
	}
	for _, q := range []interval.Span[int]{interval.HalfOpen(5, 5), interval.HalfOpen(6, 5)} {
		if e := m.Overlaps(q); e.Valid() {
			t.Fatalf("empty span %v overlaps %v", q, e.Item())
		}
	}
	if e := m.Overlaps(interval.Point(5)); !e.Valid() || e.Item() != (span{5, 6}) {
		t.Fatalf("Point(5) = valid %v %v", e.Valid(), e.Item())
	}
	// Seek reuses the iterator for another query.
	if !ov.Seek(interval.HalfOpen(0, 6)) || ov.Item() != (span{0, 1}) || !ov.Next() || ov.Item() != (span{5, 6}) || ov.Next() {
		t.Fatal("Seek did not restart the scan")
	}
}

// TestPointerEndpoints uses endpoints whose comparator cannot take the zero
// value, which the augmentation must therefore never compare.
func TestPointerEndpoints(t *testing.T) {
	type ps struct{ lo, hi *int }
	deref := func(a, b *int) int { return cmp.Compare(*a, *b) }
	m := interval.NewSet(interval.Bounds[ps, *int]{
		Compare: deref,
		Key:     func(s ps) *int { return s.lo },
		End:     func(s ps) *int { return s.hi },
		HasEnd:  func(s ps) bool { return s.hi != nil },
	}, aug.WithDegree(2))
	mk := func(lo, hi int) ps {
		p := ps{lo: &lo}
		if hi > lo {
			p.hi = &hi
		}
		return p
	}
	for i := -50; i < 50; i++ {
		m.Upsert(mk(i, i+3))
		m.Upsert(mk(i, i)) // a point
	}
	if err := m.Verify(); err != nil {
		t.Fatal(err)
	}
	n := 0
	lo, hi := -10, -8
	for range m.Overlapping(interval.HalfOpen(&lo, &hi)) {
		n++
	}
	// Ranges starting at -12..-9 and points -10, -9.
	if n != 6 {
		t.Fatalf("overlapping %d, want 6", n)
	}
	for i := -50; i < 50; i += 2 {
		m.Delete(mk(i, i+3))
	}
	if err := m.Verify(); err != nil {
		t.Fatal(err)
	}
}

// TestEmptyIntervalsArePoints checks that an interval whose end is not
// after its start behaves as a point, wherever it sits in the tree.
func TestEmptyIntervalsArePoints(t *testing.T) {
	b := interval.BoundsOf[span](cmp.Compare[int])
	b.HasEnd = func(span) bool { return true } // [3,3) claims an end
	m := interval.NewSet(b, aug.WithDegree(2))
	m.Upsert(span{3, 3})
	collect := func(q span) (got []span) {
		for s := range m.Overlapping(toSpan(q)) {
			got = append(got, s)
		}
		return got
	}
	alone := collect(span{3, 4})
	if !slices.Equal(alone, []span{{3, 3}}) {
		t.Fatalf("alone in a leaf: %v", alone)
	}
	if got := collect(span{2, 3}); len(got) != 0 {
		t.Fatalf("[2,3) should not cover the point 3: %v", got)
	}
	// Split the leaf many times over; the answer must not change.
	for i := range 200 {
		m.Upsert(span{i * 10, i*10 + 1})
	}
	if got := collect(span{3, 4}); !slices.Equal(got, []span{{0, 1}, {3, 3}}) && !slices.Equal(got, []span{{3, 3}}) {
		t.Fatalf("after splits: %v", got)
	}
	found := false
	for _, s := range collect(span{3, 4}) {
		found = found || s == span{3, 3}
	}
	if !found {
		t.Fatal("[3,3) lost after the leaf split")
	}
	// A reversed interval is also a point at its start.
	m.Upsert(span{50, 40})
	if got := collect(span{50, 51}); !slices.Contains(got, span{50, 40}) {
		t.Fatalf("reversed interval not found as a point: %v", got)
	}
	if got := collect(span{41, 49}); slices.Contains(got, span{50, 40}) {
		t.Fatalf("reversed interval matched inside its reversed range: %v", got)
	}
	if err := m.Verify(); err != nil {
		t.Fatal(err)
	}
}

// TestTieBreakCannotReorderStarts checks that a tie-breaker preferring
// ends cannot break the start ordering overlap searches rely on.
func TestTieBreakCannotReorderStarts(t *testing.T) {
	b := interval.BoundsOf[span](cmp.Compare[int])
	b.TieBreak = func(a, c span) int { return cmp.Compare(a.hi, c.hi) }
	m := interval.NewSet(b)
	for _, s := range []span{{0, 8}, {10, 11}, {5, 20}} {
		m.Upsert(s)
	}
	var got []span
	for s := range m.Overlapping(interval.HalfOpen(6, 7)) {
		got = append(got, s)
	}
	if want := []span{{0, 8}, {5, 20}}; !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// TestDefaultEqualityNormalizesPoints checks that the default tie-break
// treats every representation of a point at the same key as the same item.
func TestDefaultEqualityNormalizesPoints(t *testing.T) {
	m := interval.NewSet(interval.BoundsOf[span](cmp.Compare[int]))
	m.Upsert(span{5, 2}) // reversed: a point at 5
	if _, replaced := m.Upsert(span{5, 5}); !replaced {
		t.Fatal("{5,5} did not replace the reversed point {5,2}")
	}
	if m.Len() != 1 {
		t.Fatalf("len %d", m.Len())
	}
	if got, ok := m.Get(span{5, 0}); !ok || got != (span{5, 5}) {
		t.Fatalf("Get = %v %v", got, ok)
	}
	m.Upsert(span{5, 9}) // a range at the same start is a different item
	if m.Len() != 2 {
		t.Fatalf("len %d after adding a range", m.Len())
	}
	// The set API: iterators and cursors expose items only.
	it := m.Iterator()
	if !it.First() || it.Item() != (span{5, 5}) || !it.Next() || it.Item() != (span{5, 9}) {
		t.Fatal("set iterator order")
	}
	c := m.Cursor()
	if !c.SeekExact(span{5, 5}) {
		t.Fatal("SeekExact")
	}
	if removed := c.Delete(); removed != (span{5, 5}) || !c.Valid() || c.Item() != (span{5, 9}) {
		t.Fatalf("cursor Delete = %v, now at valid=%v %v", removed, c.Valid(), c.Item())
	}
	if removed, ok := m.Delete(span{5, 9}); !ok || removed != (span{5, 9}) {
		t.Fatalf("Delete = %v %v", removed, ok)
	}
}
