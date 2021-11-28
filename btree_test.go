package btree

import (
	"constraints"
	"testing"
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

func TestBTree(t *testing.T) {
	assertEq := func(t *testing.T, exp, got int) {
		t.Helper()
		if exp != got {
			t.Fatalf("expected %d, got %d", exp, got)
		}
	}

	tree := New[int, struct{}](Compare[int])
	tree.Insert(2, struct{}{})
	tree.Insert(12, struct{}{})
	tree.Insert(1, struct{}{})

	iter := tree.MakeIter()
	iter.First()
	for _, exp := range []int{1, 2, 12} {
		assertEq(t, exp, iter.Key())
		iter.Next()
	}

}
