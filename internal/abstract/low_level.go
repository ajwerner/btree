package abstract

// LowLevelIterator is exposed to developers within this module for use
// implemented augmented search functionality.
type LowLevelIterator[K, V, Aux, A any, AP Aug[K, Aux, A]] Iterator[K, V, Aux, A, AP]

// LowLevel converts an iterator to a LowLevelIterator. Given this package
// is internal, callers outside of this module cannot construct a
// LowLevelIterator.
func LowLevel[K, V, Aux, A any, AP Aug[K, Aux, A]](
	it *Iterator[K, V, Aux, A, AP],
) *LowLevelIterator[K, V, Aux, A, AP] {
	return it.lowLevel()
}

// Config returns the Map's config.
func (i *LowLevelIterator[K, V, Aux, A, AP]) Config() *Config[K, Aux] {
	return &i.r.cfg.Config
}

// IncrementPos increments the iterator's position within the current node.
func (i *LowLevelIterator[K, V, Aux, A, AP]) IncrementPos() {
	i.SetPos(i.pos + 1)
}

// SetPos sets the iterator's position with the current node.
func (i *LowLevelIterator[K, V, Aux, A, AP]) SetPos(pos int16) {
	i.pos = pos
}

// IsLeaf returns true if the current node is a leaf.
func (i *LowLevelIterator[K, V, Aux, A, AP]) IsLeaf() bool {
	return i.node.IsLeaf()
}

// Node returns the current node.
func (i *LowLevelIterator[K, V, Aux, A, AP]) Node() Node[K, *A] {
	return i.node
}

// Pos returns the current position within the current node.
func (i *LowLevelIterator[K, V, Aux, A, AP]) Pos() int16 {
	return i.pos
}

// Depth returns the number of nodes above the current node in the stack.
// It is illegal to call Ascend if this function returns 0.
func (i *LowLevelIterator[K, V, Aux, A, AP]) Depth() int {
	return i.s.len()
}

// Child returns the augmentation of the child node at the current position.
// It is illegal to call if this is a leaf node or there is no child
// node at the current position.
func (i *LowLevelIterator[K, V, Aux, A, AP]) Child() AP {
	return &i.children[i.pos].aug
}

// Descend pushes the current position into the iterators stack and
// descends into the child node currently pointed to by the iterator.
// It is illegal to call if there is no such child. The position in the
// new node will be 0.
func (i *LowLevelIterator[K, V, Aux, A, AP]) Descend() {
	i.s.push(i.iterFrame)
	i.iterFrame = i.makeFrame(i.children[i.pos], 0)
}

// Ascend ascends up to the current node's parent and resets the position
// to the one previously set for this parent node.
func (i *LowLevelIterator[K, V, Aux, A, AP]) Ascend() {
	i.iterFrame = i.s.pop()
}

func (i *LowLevelIterator[K, V, Aux, A, AP]) makeFrame(
	n *node[K, V, Aux, A, AP], pos int16,
) iterFrame[K, V, Aux, A, AP] {
	return iterFrame[K, V, Aux, A, AP]{
		node: n,
		pos:  pos,
	}
}
