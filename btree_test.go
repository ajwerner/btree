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

	tree := NewSet(Compare[int])
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
