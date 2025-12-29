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
	"cmp"
	"fmt"

	"github.com/ajwerner/btree/interval"
)

type pair[T cmp.Ordered] [2]T

func (p pair[T]) compare(o pair[T]) int {
	if c := cmp.Compare(p.first(), o.first()); c != 0 {
		return c
	}
	return cmp.Compare(p.second(), o.second())
}

func (p pair[T]) first() T  { return p[0] }
func (p pair[T]) second() T { return p[1] }

func Example() {
	m := interval.MakeSet(
		cmp.Compare[int],
		pair[int].compare,
		pair[int].first,
		pair[int].second,
		nil,
	)
	for _, p := range []pair[int]{
		{1, 2}, {2, 3}, {1, 5}, {0, 6}, {2, 7},
	} {
		m.Upsert(p)
	}
	it := m.Iterator()
	for it.FirstOverlap(pair[int]{4, 5}); it.Valid(); it.NextOverlap() {
		fmt.Println(it.Cur())
	}
	// Output:
	// [0 6]
	// [1 5]
	// [2 7]
}
