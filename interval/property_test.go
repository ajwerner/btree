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
	lo := rng.IntN(200)
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
