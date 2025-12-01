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
	"fmt"
	"strings"

	"github.com/anacrolix/btree"
)

func ExampleMap() {
	m := btree.MakeMap[string, int](strings.Compare)
	m.Upsert("foo", 1)
	m.Upsert("bar", 2)
	fmt.Println(m.Get("foo"))
	fmt.Println(m.Get("baz"))
	it := m.Iterator()
	for it.First(); it.Valid(); it.Next() {
		fmt.Println(it.Cur(), it.Value())
	}

	// Output:
	// 1 true
	// 0 false
	// bar 2
	// foo 1
}
