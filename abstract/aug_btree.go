package abstract

import (
	"strings"
)

// TODO(ajwerner): It'd be amazing to find a way to make this not a single
// compile-time constant.

const (
	Degree   = 2
	MaxItems = 2*Degree - 1
	MinItems = Degree - 1
)

// TODO(ajwerner): Probably we want to have comparison return an integer result
// TODO(ajwerner): Probably we want comparison to occur on pointers to the
// objects rather than the objects themselves, at least in some cases. For very
// large objects, probably it's better to just store the objects as pointers
// in the btree itself and to use a sync.Pool to pool allocations. For very
// small objects, directly calling less on the object is probably ideal. The
// question is mid-sized objects.
// TODO(ajwerner): I think it'd be better to take a comparison function.
// TODO(ajwerner): A KV mapping with comparisons over keys is more general
// than an object which is both.

// AugBTree is an implementation of an augmented B-Tree.
//
// Write operations are not safe for concurrent mutation by multiple
// goroutines, but Read operations are.
type AugBTree[T Item[T], A any, AP Aug[T, A]] struct {
	root   *node[T, A, AP]
	length int
}

// Reset removes all items from the AugBTree. In doing so, it allows memory
// held by the AugBTree to be recycled. Failure to call this method before
// letting a AugBTree be GCed is safe in that it won't cause a memory leak,
// but it will prevent AugBTree nodes from being efficiently re-used.
func (t *AugBTree[T, A, AP]) Reset() {
	if t.root != nil {
		t.root.decRef(true /* recursive */)
		t.root = nil
	}
	t.length = 0
}

// Clone clones the AugBTree, lazily. It does so in constant time.
func (t *AugBTree[T, A, AP]) Clone() *AugBTree[T, A, AP] {
	c := *t
	if c.root != nil {
		// Incrementing the reference count on the root node is sufficient to
		// ensure that no node in the cloned tree can be mutated by an actor
		// holding a reference to the original tree and vice versa. This
		// property is upheld because the root node in the receiver AugBTree and
		// the returned AugBTree will both necessarily have a reference count of at
		// least 2 when this method returns. All tree mutations recursively
		// acquire mutable node references (see mut) as they traverse down the
		// tree. The act of acquiring a mutable node reference performs a clone
		// if a node's reference count is greater than one. Cloning a node (see
		// clone) increases the reference count on each of its children,
		// ensuring that they have a reference count of at least 2. This, in
		// turn, ensures that any of the child nodes that are modified will also
		// be copied-on-write, recursively ensuring the immutability property
		// over the entire tree.
		c.root.incRef()
	}
	return &c
}

// Delete removes an item equal to the passed in item from the tree.
func (t *AugBTree[T, A, AP]) Delete(item T) (found bool) {
	if t.root == nil || t.root.count == 0 {
		return false
	}
	if _, found, _ = mut(&t.root).remove(item); found {
		t.length--
	}
	if t.root.count == 0 {
		old := t.root
		if t.root.leaf {
			t.root = nil
		} else {
			t.root = t.root.children[0]
		}
		old.decRef(false /* recursive */)
	}
	return found
}

// Set adds the given item to the tree. If an item in the tree already equals
// the given one, it is replaced with the new item.
func (t *AugBTree[T, A, AP]) Set(item T) {
	if t.root == nil {
		t.root = newLeafNode[T, A, AP]()
	} else if t.root.count >= MaxItems {
		splitLa, splitNode := mut(&t.root).split(MaxItems / 2)
		newRoot := newNode[T, A, AP]()
		newRoot.count = 1
		newRoot.items[0] = splitLa
		newRoot.children[0] = t.root
		AP(&t.root.aug).Update(t.root)
		AP(&splitNode.aug).Update(splitNode)
		newRoot.children[1] = splitNode
		AP(&newRoot.aug).Update(newRoot)
		t.root = newRoot
	}
	if replaced, _ := mut(&t.root).insert(item); !replaced {
		t.length++
	}
}

// MakeIter returns a new Iterator object. It is not safe to continue using an
// Iterator after modifications are made to the tree. If modifications are made,
// create a new Iterator.
func (t *AugBTree[T, A, AP]) MakeIter() Iterator[T, A, AP] {
	it := Iterator[T, A, AP]{r: t.root}
	it.Reset()
	return it
}

// Height returns the height of the tree.
func (t *AugBTree[T, A, AP]) Height() int {
	if t.root == nil {
		return 0
	}
	h := 1
	n := t.root
	for !n.leaf {
		n = n.children[0]
		h++
	}
	return h
}

// Len returns the number of items currently in the tree.
func (t *AugBTree[T, A, AP]) Len() int {
	return t.length
}

// String returns a string description of the tree. The format is
// similar to the https://en.wikipedia.org/wiki/Newick_format.
func (t *AugBTree[T, A, AP]) String() string {
	if t.length == 0 {
		return ";"
	}
	var b strings.Builder
	t.root.writeString(&b)
	return b.String()
}
