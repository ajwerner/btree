package abstract

type Item[T any] interface {
	Less(T) bool
}

// TODO(ajwerner): Tighten up this interface and make this more generally
// efficient for updates where possible.
type Node[A any] interface {
	GetA() A
	Count() int16
	IsLeaf() bool
	GetChild(i int16) Node[A]
}

// Aug is a data structure which augments a node of the tree. It is updated
// when the structure or contents of the subtree rooted at the current node
// changes.
type Aug[T Item[T], A any] interface {
	*A
	Update(n Node[*A])
	UpdateOnInsert(item T, n, child Node[*A]) (updated bool)
	UpdateOnRemoval(item T, n, child Node[*A]) (updated bool)
}
