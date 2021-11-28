package interval

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
	tree := NewMap[int, struct{}, IntInterval](Compare[int])
	items := []IntInterval{{1, 2}, {2, 3}, {2, 4}, {3, 3}, {3, 4}}
	for _, item := range items {
		tree.Set(item, struct{}{})
	}
	iter := tree.MakeIter()
	iter.First()
	for _, exp := range items {
		assertEq(t, exp, iter.Key())
		iter.Next()
	}
}
