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

package btree_test

import (
	"cmp"
	"fmt"
	"strings"

	"github.com/ajwerner/btree"
)

func ExampleMap() {
	m := btree.New[string, int](strings.Compare)
	m.Upsert("foo", 1)
	m.Upsert("bar", 2)
	fmt.Println(m.Get("foo"))
	fmt.Println(m.Get("baz"))
	it := m.Iterator()
	for it.First(); it.Valid(); it.Next() {
		fmt.Println(it.Key(), it.Value())
	}

	// Output:
	// 1 true
	// 0 false
	// bar 2
	// foo 1
}

// ExampleMapCursor charges the least loaded host and moves it to its new
// position without a second descent.
func ExampleMapCursor() {
	type host struct {
		load int
		name string
	}
	byLoad := func(a, b host) int {
		if c := cmp.Compare(a.load, b.load); c != 0 {
			return c
		}
		return strings.Compare(a.name, b.name)
	}
	fleet := btree.NewSet(byLoad)
	fleet.Upsert(host{3, "a"})
	fleet.Upsert(host{1, "b"})
	fleet.Upsert(host{2, "c"})

	c := fleet.Cursor()
	for range 3 {
		c.First()
		h := c.Item()
		h.load += 2
		c.Rekey(h)
		fmt.Println("charged", h.name, "to", h.load)
	}
	for h := range fleet.All() {
		fmt.Println(h.name, h.load)
	}
	// Output:
	// charged b to 3
	// charged c to 4
	// charged a to 5
	// b 3
	// c 4
	// a 5
}
