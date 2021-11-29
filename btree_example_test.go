package btree_test

import (
	"fmt"
	"strings"

	"github.com/ajwerner/btree"
)

func ExampleMap() {
	m := btree.NewMap[string, int](strings.Compare)
	m.Upsert("foo", 1)
	m.Upsert("bar", 2)
	fmt.Println(m.Get("foo"))
	fmt.Println(m.Get("baz"))
	it := m.Iterator()
	for it.First(); it.Valid(); it.Next() {
		fmt.Println(it.Key(), it.Value())
	}

	// Output:
	// 1 true
	// 0 false
	// bar 2
	// foo 1
}
