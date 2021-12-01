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

// Interval represents an interval with bounds from [Key(), End()) where
// Key() is inclusive and End() is exclusive. If Key() == End(), then the
// Inteval represents a point that only includes that value. Intervals with
// Key() which is larger than End() are invalid and may result in panics
// upon insertion.
type Interval[K any] interface {
	Key() K
	End() K
}

type IntervalWithID[K, ID any] interface {
	Interval[K]
	ID() ID
}

func IntervalIDCompare[K, ID any, I IntervalWithID[K, ID]](cmpK Cmp[K], cmpID Cmp[ID]) Cmp[I] {
	return func(a, b I) int {
		if c := cmpK(a.Key(), b.Key()); c != 0 {
			return c
		}
		if c := cmpK(a.End(), b.End()); c != 0 {
			return c
		}
		return cmpID(a.ID(), b.ID())
	}
}

func IntervalCompare[I Interval[K], K any](f func(K, K) int) func(I, I) int {
	return func(a, b I) int {
		if c := f(a.Key(), b.Key()); c != 0 {
			return c
		}
		return f(a.End(), b.End())
	}
}

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
	tree := MakeMap[IntInterval, int, struct{}](
		ordered.Compare[int],
		IntervalCompare[IntInterval](ordered.Compare[int]),
		IntInterval.Key,
		IntInterval.End,
		nil,
	)
	items := []IntInterval{{1, 4}, {2, 5}, {3, 3}, {3, 6}, {4, 7}}
	for _, item := range items {
		tree.Upsert(item, struct{}{})
	}
	iter := tree.Iterator()
	iter.First()
	for _, exp := range items {
		assertEq(t, exp, iter.Cur())
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
			res = append(res, iter.Cur())
		}
		require.Equal(t, tc.res, res)
	}
}
