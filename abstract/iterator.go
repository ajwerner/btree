package abstract

// Iterator is responsible for search and traversal within a AugBTree.
type Iterator[K, V, Aux, A any, AP Aug[K, Aux, A]] struct {
	r *Map[K, V, Aux, A, AP]
	iterFrame[K, V, Aux, A, AP]
	s iterStack[K, V, Aux, A, AP]
	// TODO(ajwerner): Add back augmented search
}

func (i *Iterator[K, V, Aux, A, AP]) Aux() Aux {
	return i.r.td.aux
}

func (i *Iterator[K, V, Aux, A, AP]) Reset() {
	i.node = i.r.root
	i.pos = -1
	i.s.reset()
}

// SeekGE seeks to the first key greater-than or equal to the provided
// key.
func (i *Iterator[K, V, Aux, A, AP]) SeekGE(key K) {
	i.Reset()
	if i.node == nil {
		return
	}
	for {
		pos, found := i.find(i.r.td.cmp, key)
		i.pos = int16(pos)
		if found {
			return
		}
		if i.leaf {
			if i.pos == i.count {
				i.Next()
			}
			return
		}
		i.Descend()
	}
}

// SeekLT seeks to the first key less-than the provided key.
func (i *Iterator[K, V, Aux, A, AP]) SeekLT(key K) {
	i.Reset()
	if i.node == nil {
		return
	}
	for {
		pos, found := i.find(i.r.td.cmp, key)
		i.pos = int16(pos)
		if found || i.leaf {
			i.Prev()
			return
		}
		i.Descend()
	}
}

// First seeks to the first key in the AugBTree.
func (i *Iterator[K, V, Aux, A, AP]) First() {
	i.Reset()
	i.pos = 0
	if i.node == nil {
		return
	}
	for !i.leaf {
		i.Descend()
	}
	i.pos = 0
}

// Last seeks to the last key in the AugBTree.
func (i *Iterator[K, V, Aux, A, AP]) Last() {
	i.Reset()
	if i.node == nil {
		return
	}
	for !i.leaf {
		i.pos = i.count
		i.Descend()
	}
	i.pos = i.count - 1
}

func (i *Iterator[K, V, Aux, A, AP]) IncrementPos() {
	i.SetPos(i.pos + 1)
}

func (i *Iterator[K, V, Aux, A, AP]) SetPos(pos int16) { i.pos = pos }

// Next positions the Iterator to the key immediately following
// its current position.
func (i *Iterator[K, V, Aux, A, AP]) Next() {
	if i.node == nil {
		return
	}

	if i.leaf {
		i.pos++
		if i.pos < i.count {
			return
		}
		for i.s.len() > 0 && i.pos >= i.count {
			i.Ascend()
		}
		return
	}
	i.pos++
	i.Descend()
	for !i.leaf {
		i.pos = 0
		i.Descend()
	}
	i.pos = 0
}

// Prev positions the Iterator to the key immediately preceding
// its current position.
func (i *Iterator[K, V, Aux, A, AP]) Prev() {
	if i.node == nil {
		return
	}

	if i.leaf {
		i.pos--
		if i.pos >= 0 {
			return
		}
		for i.s.len() > 0 && i.pos < 0 {
			i.Ascend()
			i.pos--
		}
		return
	}

	i.Descend()
	for !i.leaf {
		i.pos = i.count
		i.Descend()
	}
	i.pos = i.count - 1
}

// Valid returns whether the Iterator is positioned at a valid position.
func (i *Iterator[K, V, Aux, A, AP]) Valid() bool {
	return i.pos >= 0 && i.pos < i.count
}

// Cur returns the key at the Iterator's current position. It is illegal
// to call Cur if the Iterator is not valid.
func (i *Iterator[K, V, Aux, A, AP]) Key() K {
	return i.keys[i.pos]
}

func (i *Iterator[K, V, Aux, A, AP]) Value() V {
	return i.values[i.pos]
}

func (i *Iterator[K, V, Aux, A, AP]) IsLeaf() bool {
	return i.leaf
}

func (i *Iterator[K, V, Aux, A, AP]) Node() Node[K, *A] {
	return i.node
}

func (i *Iterator[K, V, Aux, A, AP]) Pos() int16 {
	return i.pos
}

func (i *Iterator[K, V, Aux, A, AP]) makeFrame(n *node[K, V, Aux, A, AP], pos int16) iterFrame[K, V, Aux, A, AP] {
	return iterFrame[K, V, Aux, A, AP]{
		node: n,
		pos:  pos,
	}
}

func (i *Iterator[K, V, Aux, A, AP]) Child() AP {
	return &i.children[i.pos].aug
}

func (i *Iterator[K, V, Aux, A, AP]) Descend() {
	i.s.push(i.iterFrame)
	i.iterFrame = i.makeFrame(i.children[i.pos], 0)
}

// ascend ascends up to the current node's parent and resets the position
// to the one previously set for this parent node.
func (i *Iterator[K, V, Aux, A, AP]) Ascend() {
	i.iterFrame = i.s.pop()
}
