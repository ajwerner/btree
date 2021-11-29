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

package interval

import (
	"testing"

	"github.com/ajwerner/btree/internal/ordered"
	"github.com/stretchr/testify/require"
)

type IntInterval [2]int

func (i IntInterval) Key() int { return i[0] }
func (i IntInterval) End() int { return i[1] }

func TestIntervalTree(t *testing.T) {
	assertEq := func(t *testing.T, exp, got IntInterval) {
		t.Helper()
		if exp != got {
			t.Fatalf("expected %d, got %d", exp, got)
		}
	}
	tree := NewMap[IntInterval, struct{}](
		ordered.Compare[int],
		IntervalCompare[IntInterval](ordered.Compare[int]),
	)
	items := []IntInterval{{1, 4}, {2, 5}, {3, 3}, {3, 6}, {4, 7}}
	for _, item := range items {
		tree.Upsert(item, struct{}{})
	}
	iter := tree.MakeIter()
	iter.First()
	for _, exp := range items {
		assertEq(t, exp, iter.Key())
		iter.Next()
	}

	for _, tc := range []struct {
		q   IntInterval
		res []IntInterval
	}{
		{
			q:   IntInterval{2, 3},
			res: []IntInterval{{1, 4}, {2, 5}},
		},
		{
			q:   IntInterval{2, 4},
			res: []IntInterval{{1, 4}, {2, 5}, {3, 3}, {3, 6}},
		},
	} {
		var res []IntInterval
		for iter.FirstOverlap(tc.q); iter.Valid(); iter.NextOverlap() {
			res = append(res, iter.Key())
		}
		require.Equal(t, tc.res, res)
	}
}
