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
			for s := range m.Overlapping(q) {
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
				removed := m.Delete(s)
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

// TestOverlapScanAfterSeek checks that a plain seek ends an overlap scan
// in progress, so that a later NextOverlap does not continue with stale
// constraints.
func TestOverlapScanAfterSeek(t *testing.T) {
	m := interval.NewSet(spanBounds())
	for i := 0; i < 100; i += 2 {
		m.Upsert(span{i, i + 3})
	}
	it := m.Iterator()
	it.FirstOverlap(span{10, 12})
	if !it.Valid() || it.Key() != (span{8, 11}) {
		t.Fatalf("FirstOverlap at %v", it.Key())
	}
	it.SeekGE(span{50, 53})
	if !it.Valid() || it.Key() != (span{50, 53}) {
		t.Fatalf("SeekGE at %v", it.Key())
	}
	// With the scan ended, NextOverlap invalidates rather than scanning.
	it.NextOverlap()
	if it.Valid() {
		t.Fatalf("NextOverlap after SeekGE is valid at %v", it.Key())
	}
	// And a fresh scan works.
	var got []span
	for it.FirstOverlap(span{50, 51}); it.Valid(); it.NextOverlap() {
		got = append(got, it.Key())
	}
	if want := []span{{48, 51}, {50, 53}}; !slices.Equal(got, want) {
		t.Fatalf("scan after seek: %v, want %v", got, want)
	}
}

// TestOverlapScanEndsOnStep checks that Next and Prev end an overlap scan,
// so NextOverlap afterwards does not apply stale constraints.
func TestOverlapScanEndsOnStep(t *testing.T) {
	m := interval.NewSet(spanBounds())
	for _, s := range []span{{0, 1}, {5, 6}, {10, 11}, {15, 16}} {
		m.Upsert(s)
	}
	it := m.Iterator()
	it.FirstOverlap(span{10, 16})
	if !it.Valid() || it.Key() != (span{10, 11}) {
		t.Fatalf("FirstOverlap at %v", it.Key())
	}
	it.Prev()
	it.Prev()
	if !it.Valid() || it.Key() != (span{0, 1}) {
		t.Fatalf("after two Prev at %v", it.Key())
	}
	it.NextOverlap()
	if it.Valid() {
		t.Fatalf("NextOverlap after Prev returned %v", it.Key())
	}
	it.FirstOverlap(span{10, 16})
	it.Next()
	if !it.Valid() || it.Key() != (span{15, 16}) {
		t.Fatalf("Next after FirstOverlap at %v", it.Key())
	}
	it.NextOverlap()
	if it.Valid() {
		t.Fatalf("NextOverlap after Next returned %v", it.Key())
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
	for range m.Overlapping(mk(-10, -8)) {
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
		for s := range m.Overlapping(q) {
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
	for s := range m.Overlapping(span{6, 7}) {
		got = append(got, s)
	}
	if want := []span{{0, 8}, {5, 20}}; !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
