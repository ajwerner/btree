package abstract

type LowLevelIterator[K, V, Aux, A any, AP Aug[K, Aux, A]] Iterator[K, V, Aux, A, AP]

func LowLevel[K, V, Aux, A any, AP Aug[K, Aux, A]](it *Iterator[K, V, Aux, A, AP]) *LowLevelIterator[K, V, Aux, A, AP] {
	return it.lowLevel()
}

func (i *LowLevelIterator[K, V, Aux, A, AP]) Aux() Aux {
	return i.r.td.aux
}

func (i *LowLevelIterator[K, V, Aux, A, AP]) IncrementPos() {
	i.SetPos(i.pos + 1)
}

func (i *LowLevelIterator[K, V, Aux, A, AP]) SetPos(pos int16) {
	i.pos = pos
}

func (i *LowLevelIterator[K, V, Aux, A, AP]) IsLeaf() bool {
	return i.leaf
}

func (i *LowLevelIterator[K, V, Aux, A, AP]) Node() Node[K, *A] {
	return i.node
}

func (i *LowLevelIterator[K, V, Aux, A, AP]) Pos() int16 {
	return i.pos
}

func (i *LowLevelIterator[K, V, Aux, A, AP]) Depth() int {
	return i.s.len()
}

func (i *LowLevelIterator[K, V, Aux, A, AP]) makeFrame(n *node[K, V, Aux, A, AP], pos int16) iterFrame[K, V, Aux, A, AP] {
	return iterFrame[K, V, Aux, A, AP]{
		node: n,
		pos:  pos,
	}
}

func (i *LowLevelIterator[K, V, Aux, A, AP]) Child() AP {
	return &i.children[i.pos].aug
}

func (i *LowLevelIterator[K, V, Aux, A, AP]) Descend() {
	i.s.push(i.iterFrame)
	i.iterFrame = i.makeFrame(i.children[i.pos], 0)
}

// ascend ascends up to the current node's parent and resets the position
// to the one previously set for this parent node.
func (i *LowLevelIterator[K, V, Aux, A, AP]) Ascend() {
	i.iterFrame = i.s.pop()
}
