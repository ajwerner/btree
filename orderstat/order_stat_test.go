package orderstat

import (
	"math/rand"
	"testing"
)

type Int int

func (i Int) Less(o Int) bool {
	return i < o
}

func assertIntEq(t *testing.T, exp, got Int) {
	t.Helper()
	if exp != got {
		t.Fatalf("expected %d, got %d", exp, got)
	}
}

func TestOrderStatTree(t *testing.T) {
	tree := MakeOrderStatTree[Int]()
	tree.Set(2)
	tree.Set(3)
	tree.Set(5)
	tree.Set(4)
	iter := tree.MakeIter()
	iter.First()
	for _, exp := range []Int{2, 3, 4, 5} {
		assertIntEq(t, exp, iter.Cur())
		iter.Next()
	}
	iter.Nth(2)
	assertIntEq(t, 4, iter.Cur())
}

func TestOrderStatNth(t *testing.T) {
	t.Parallel()
	tree := MakeOrderStatTree[Int]()
	const maxN = 1000
	N := rand.Intn(maxN)
	items := make([]int, 0, N)
	for i := 0; i < N; i++ {
		items = append(items, i)
	}
	perm := rand.Perm(N)
	for _, idx := range perm {
		tree.Set(Int(items[idx]))
	}
	removePerm := rand.Perm(N)
	retainAll := rand.Float64() < .25
	var removed []int
	for _, idx := range removePerm {
		if !retainAll && rand.Float64() < .05 {
			continue
		}
		tree.Remove(Int(items[idx]))
		removed = append(removed, items[idx])
	}
	t.Logf("removed %d/%d", len(removed), N)
	for _, i := range removed {
		tree.Set(Int(i))
	}
	perm = rand.Perm(N)

	iter := tree.MakeIter()
	for _, idx := range perm {
		iter.Nth(idx)
		assertIntEq(t, Int(items[idx]), iter.Cur())
		for i := idx + 1; i < N; i++ {
			iter.Next()
			assertIntEq(t, Int(items[i]), iter.Cur())
		}
		iter.Next()
		if iter.Valid() {
			t.Fatal("expected invalid")
		}
	}
}
