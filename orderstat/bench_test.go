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

package orderstat

import (
	"cmp"
	"fmt"
	"math/rand/v2"
	"testing"
)

func benchSet(n int) (*Set[int], []int) {
	s := NewSet(cmp.Compare[int])
	rng := rand.New(rand.NewPCG(1, 2))
	p := rng.Perm(n)
	for _, k := range p {
		s.Upsert(k)
	}
	return s, p
}

func BenchmarkRank(b *testing.B) {
	for _, n := range []int{10_000, 1_000_000} {
		s, p := benchSet(n)
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			i := 0
			for b.Loop() {
				s.Rank(p[i%n])
				i++
			}
		})
	}
}

func BenchmarkIteratorRank(b *testing.B) {
	for _, n := range []int{10_000, 1_000_000} {
		s, p := benchSet(n)
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			it := s.Iterator()
			i := 0
			for b.Loop() {
				it.SeekGE(p[i%n])
				it.Rank()
				i++
			}
		})
	}
}

func BenchmarkSeekNth(b *testing.B) {
	for _, n := range []int{10_000, 1_000_000} {
		s, p := benchSet(n)
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			it := s.Iterator()
			i := 0
			for b.Loop() {
				it.SeekNth(p[i%n])
				i++
			}
		})
	}
}

func BenchmarkSeekNthViaSeekPrefix(b *testing.B) {
	for _, n := range []int{10_000, 1_000_000} {
		m := New[int, struct{}](cmp.Compare[int])
		rng := rand.New(rand.NewPCG(1, 2))
		p := rng.Perm(n)
		for _, k := range p {
			m.Upsert(k, struct{}{})
		}
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			it := m.Iterator()
			i := 0
			for b.Loop() {
				nth := p[i%n]
				it.SeekPrefix(func(inclusive int) bool { return inclusive > nth })
				i++
			}
		})
	}
}

func BenchmarkCount(b *testing.B) {
	for _, n := range []int{10_000, 1_000_000} {
		s, p := benchSet(n)
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			i := 0
			for b.Loop() {
				lo := p[i%n]
				s.Count(lo, lo+n/10)
				i++
			}
		})
	}
}

func BenchmarkSeekGE(b *testing.B) {
	for _, n := range []int{10_000, 1_000_000} {
		s, p := benchSet(n)
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			it := s.Iterator()
			i := 0
			for b.Loop() {
				it.SeekGE(p[i%n])
				i++
			}
		})
	}
}

// BenchmarkRankAfterNext measures Rank at the cursor while stepping, the
// pattern an eager per-frame prefix would speed up.
func BenchmarkRankAfterNext(b *testing.B) {
	for _, n := range []int{10_000, 1_000_000} {
		s, _ := benchSet(n)
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			it := s.Iterator()
			it.First()
			for b.Loop() {
				it.Next()
				if !it.Valid() {
					it.First()
				}
				it.Rank()
			}
		})
		b.Run(fmt.Sprintf("NextOnly/n=%d", n), func(b *testing.B) {
			it := s.Iterator()
			it.First()
			for b.Loop() {
				it.Next()
				if !it.Valid() {
					it.First()
				}
			}
		})
	}
}
