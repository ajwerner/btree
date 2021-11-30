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

package interval_test

import (
	"fmt"

	"github.com/ajwerner/btree/internal/ordered"
	"github.com/ajwerner/btree/interval"
)

func ExampleBlog() {
	type pair [2]int
	m := interval.MakeSet(
		ordered.Compare[int],
		func(a, b pair) int {
			if c := ordered.Compare(a[0], b[0]); c != 0 {
				return c
			}
			return ordered.Compare(a[0], b[0])
		},
		func(i pair) int { return i[0] },
		func(i pair) int { return i[1] },
	)
	for _, p := range []pair{
		{1, 2}, {2, 3}, {1, 5}, {0, 6}, {2, 7},
	} {
		m.Upsert(p)
	}
	it := m.Iterator()
	for it.FirstOverlap(pair{4, 5}); it.Valid(); it.NextOverlap() {
		fmt.Println(it.Cur())
	}
	// Output:
	// [0 6]
	// [1 5]
	// [2 7]
}
