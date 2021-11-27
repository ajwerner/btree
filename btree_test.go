package btree

import (
	"testing"
)

type Int int

func (i Int) Less(o Int) bool {
	return i < o
}

func TestBTree(t *testing.T) {
	assertEq := func(t *testing.T, exp, got Int) {
		t.Helper()
		if exp != got {
			t.Fatalf("expected %d, got %d", exp, got)
		}
	}

	t.Log("hi")
	tree := MakeBTree[Int]()
	tree.Set(2)
	tree.Set(12)
	tree.Set(1)

	iter := tree.MakeIter()
	iter.First()
	for _, exp := range []Int{1, 2, 12} {
		assertEq(t, exp, iter.Cur())
		iter.Next()
	}

}
