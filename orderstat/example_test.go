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

package orderstat_test

import (
	"cmp"
	"fmt"

	"github.com/ajwerner/btree/orderstat"
)

func Example() {
	scores := orderstat.New[int, string](cmp.Compare[int])
	for name, score := range map[string]int{"ada": 91, "bob": 78, "cy": 85, "dee": 64, "eve": 99} {
		scores.Upsert(score, name)
	}
	rank, found := scores.Rank(85)
	fmt.Println("85 is rank", rank, "found:", found)
	rank, found = scores.Rank(90)
	fmt.Println("90 would be rank", rank, "found:", found)
	fmt.Println("scores in [70, 95):", scores.Count(70, 95))
	if k, v, ok := scores.Nth(scores.Len() - 1); ok {
		fmt.Println("top:", v, k)
	}
	// Output:
	// 85 is rank 2 found: true
	// 90 would be rank 3 found: false
	// scores in [70, 95): 3
	// top: eve 99
}

func ExampleIterator_SeekNth() {
	s := orderstat.NewSet(cmp.Compare[string])
	for _, w := range []string{"pear", "apple", "fig", "kiwi"} {
		s.Upsert(w)
	}
	it := s.Iterator()
	for it.SeekNth(1); it.Valid(); it.Next() {
		fmt.Println(it.Rank(), it.Key())
	}
	// Output:
	// 1 fig
	// 2 kiwi
	// 3 pear
}
