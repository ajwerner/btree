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
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/ajwerner/btree/aug"
)

func benchValueSize[V any](b *testing.B, name string, mk func(int) V) {
	for _, n := range []int{100_000, 1_000_000} {
		m := aug.NewOrdered[int, V, struct{}](nil)
		rng := rand.New(rand.NewPCG(1, 2))
		p := rng.Perm(n)
		for _, k := range p {
			m.Upsert(k, mk(k))
		}
		b.Run(fmt.Sprintf("%s/Get/n=%d", name, n), func(b *testing.B) {
			i := 0
			for b.Loop() {
				m.Get(p[i%n])
				i++
			}
		})
		b.Run(fmt.Sprintf("%s/DeleteInsert/n=%d", name, n), func(b *testing.B) {
			i := 0
			for b.Loop() {
				k := p[i%n]
				m.Delete(k)
				m.Upsert(k, mk(k))
				i++
			}
		})
	}
}

// BenchmarkValueSize shows how the value size affects searches and updates;
// it is why keys and values live in separate slices.
func BenchmarkValueSize(b *testing.B) {
	benchValueSize(b, "v8", func(k int) int { return k })
	benchValueSize(b, "v64", func(k int) [8]int64 { return [8]int64{int64(k)} })
	benchValueSize(b, "v256", func(k int) [32]int64 { return [32]int64{int64(k)} })
}
