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
type Aug[K, Aux, A any] interface {
	*A
	Update(Node[*A], UpdateMeta[K, Aux, A]) (changed bool)
}

type Action int

const (
	Default Action = iota
	Split
	Removal
	Insertion
)

type UpdateMeta[K, Aux, A any] struct {
	Aux Aux

	// Action indicates the semantics of the below fields. If Default,
	// no fields will be populated.
	Action        Action
	ModifiedOther *A
	RelevantKey   K
}
