package interval

import (
	"constraints"
	"testing"

	"github.com/stretchr/testify/require"
)

func Compare[T constraints.Ordered](a, b T) int {
	switch {
	case a < b:
		return -1
	case a == b:
		return 0
	default:
		return 1
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
	tree := NewMap[IntInterval, struct{}](
		Compare[int],
		IntervalCompare[IntInterval](Compare[int]),
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
