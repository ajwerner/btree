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

package btree

import (
	"testing"

	"github.com/ajwerner/btree/internal/ordered"
)

func TestBTree(t *testing.T) {
	assertEq := func(t *testing.T, exp, got int) {
		t.Helper()
		if exp != got {
			t.Fatalf("expected %d, got %d", exp, got)
		}
	}

	tree := NewSet(ordered.Compare[int])
	tree.Upsert(2)
	tree.Upsert(12)
	tree.Upsert(1)

	it := tree.Iterator()
	it.First()
	expected := []int{1, 2, 12}
	for _, exp := range expected {
		assertEq(t, exp, it.Key())
		it.Next()
	}
}
