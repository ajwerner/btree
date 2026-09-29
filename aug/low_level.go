package aug

// LowLevelIterator exposes the structure of the tree to augmentation code
// implementing custom search operations.
type LowLevelIterator[K, V, A any] Iterator[K, V, A]

// LowLevel converts an iterator to a LowLevelIterator. The two share state:
// operations on either are visible through the other.
func LowLevel[K, V, A any](
	it *Iterator[K, V, A],
) *LowLevelIterator[K, V, A] {
	return it.lowLevel()
}

// Config returns a copy of the Map's configuration. A Map's Updater and
// comparison function are fixed at construction.
func (i *LowLevelIterator[K, V, A]) Config() Config[K, V, A] {
	return i.r.cfg.Config
}

// IncrementPos increments the iterator's position within the current node.
func (i *LowLevelIterator[K, V, A]) IncrementPos() {
	i.SetPos(i.pos + 1)
}

// SetPos sets the iterator's position within the current node.
func (i *LowLevelIterator[K, V, A]) SetPos(pos int16) {
	i.pos = pos
}

// Node returns the current Node.
func (i *LowLevelIterator[K, V, A]) Node() *Node[K, V, A] {
	return i.node
}

// IsLeaf returns true if the current node is a leaf.
func (i *LowLevelIterator[K, V, A]) IsLeaf() bool {
	return i.node.IsLeaf()
}

// Pos returns the current position within the current node.
func (i *LowLevelIterator[K, V, A]) Pos() int16 {
	return i.pos
}

// Depth returns the number of nodes above the current node in the stack.
// It is illegal to call Ascend if this function returns 0.
func (i *LowLevelIterator[K, V, A]) Depth() int {
	return i.s.len()
}

// Frame returns the node and position of the ancestor frame at depth d,
// where 0 is the root frame and Depth()-1 is the parent of the current
// node. The position is the index of the child through which the iterator
// descended. It is illegal to call with d outside [0, Depth()).
func (i *LowLevelIterator[K, V, A]) Frame(d int) (*Node[K, V, A], int16) {
	f := i.s.at(d)
	return f.node, f.pos
}

// ChildAug returns the augmentation of the child node at the current position.
// It is illegal to call if this is a leaf node or there is no child
// node at the current position.
func (i *LowLevelIterator[K, V, A]) ChildAug() *A {
	return &i.node.children[i.pos].aug
}

// Descend pushes the current position into the iterators stack and
// descends into the child node currently pointed to by the iterator.
// It is illegal to call if there is no such child. The position in the
// new node will be 0.
func (i *LowLevelIterator[K, V, A]) Descend() {
	i.s.push(i.iterFrame)
	i.iterFrame = i.makeFrame(i.node.children[i.pos], 0)
}

// Ascend ascends up to the current node's parent and resets the position
// to the one previously set for this parent node.
func (i *LowLevelIterator[K, V, A]) Ascend() {
	i.iterFrame = i.s.pop()
}

func (i *LowLevelIterator[K, V, A]) makeFrame(
	n *Node[K, V, A], pos int16,
) iterFrame[K, V, A] {
	return iterFrame[K, V, A]{
		node: n,
		pos:  pos,
	}
}
