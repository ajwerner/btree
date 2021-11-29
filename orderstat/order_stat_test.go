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

package orderstat

import (
	"math/rand"
	"testing"

	"github.com/ajwerner/btree/internal/ordered"
	"github.com/stretchr/testify/require"
)

func TestOrderStatTree(t *testing.T) {
	tree := NewMap[int, int](ordered.Compare[int])
	tree.Upsert(2, 1)
	tree.Upsert(3, 2)
	tree.Upsert(5, 4)
	tree.Upsert(4, 3)
	iter := tree.Iterator()
	iter.First()
	for i, exp := range []int{2, 3, 4, 5} {
		require.Equal(t, exp, iter.Key())
		require.Equal(t, i, iter.Rank())
		iter.Next()
	}
	iter.SeekNth(2)
	require.Equal(t, 4, iter.Key())
}

func TestOrderStatNth(t *testing.T) {
	t.Parallel()
	tree := NewSet(ordered.Compare[int])
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
		iter.SeekNth(idx)
		require.Equal(t, items[idx], iter.Key())
		for i := idx + 1; i < N; i++ {
			iter.Next()
			require.Equal(t, items[i], iter.Key())
		}
		require.True(t, iter.Valid())
		iter.Next()
		require.False(t, iter.Valid())
	}

	clone := tree.Clone()
	clone.Reset()
	require.Equal(t, tree.Len(), len(perm))

}
