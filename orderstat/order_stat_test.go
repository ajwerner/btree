package orderstat

import (
	"constraints"
	"math/rand"
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

func TestOrderStatTree(t *testing.T) {
	tree := NewMap[int, int](Compare[int])
	tree.Upsert(2, 1)
	tree.Upsert(3, 2)
	tree.Upsert(5, 4)
	tree.Upsert(4, 3)
	iter := tree.Iterator()
	iter.First()
	for _, exp := range []int{2, 3, 4, 5} {
		require.Equal(t, exp, iter.Key())
		iter.Next()
	}
	iter.Nth(2)
	require.Equal(t, 4, iter.Key())
}

func TestOrderStatNth(t *testing.T) {
	t.Parallel()
	tree := NewSet(Compare[int])
	const maxN = 1000
	N := rand.Intn(maxN)
	items := make([]int, 0, N)
	for i := 0; i < N; i++ {
		items = append(items, i)
	}
	perm := rand.Perm(N)
	for _, idx := range perm {
		tree.Upsert(items[idx])
	}
	removePerm := rand.Perm(N)
	retainAll := rand.Float64() < .25
	var removed []int
	for _, idx := range removePerm {
		if !retainAll && rand.Float64() < .05 {
			continue
		}
		tree.Delete(items[idx])
		removed = append(removed, items[idx])
	}
	t.Logf("removed %d/%d", len(removed), N)
	for _, i := range removed {
		tree.Upsert(i)
	}
	perm = rand.Perm(N)

	iter := tree.Iterator()
	for _, idx := range perm {
		iter.Nth(idx)
		require.Equal(t, items[idx], iter.Key())
		for i := idx + 1; i < N; i++ {
			iter.Next()
			require.Equal(t, items[i], iter.Key())
		}
		require.True(t, iter.Valid())
		iter.Next()
		require.False(t, iter.Valid())
	}
}
