package abstract

// Iterator is responsible for search and traversal within a AugBTree.
type Iterator[K, V, A any, AP Aug[K, A]] struct {
	r *Map[K, V, A, AP]
	iterFrame[K, V, A, AP]
	s iterStack[K, V, A, AP]
	// TODO(ajwerner): Add back augmented search
}

func (i *Iterator[K, V, A, AP]) Reset() {
	i.node = i.r.root
	i.pos = -1
	i.s.reset()
}

// SeekGE seeks to the first key greater-than or equal to the provided
// key.
func (i *Iterator[K, V, A, AP]) SeekGE(key K) {
	i.Reset()
	if i.node == nil {
		return
	}
	for {
		pos, found := i.find(i.r.cmp, key)
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
func (i *Iterator[K, V, A, AP]) SeekLT(key K) {
	i.Reset()
	if i.node == nil {
		return
	}
	for {
		pos, found := i.find(i.r.cmp, key)
		i.pos = int16(pos)
		if found || i.leaf {
			i.Prev()
			return
		}
		i.Descend()
	}
}

// First seeks to the first key in the AugBTree.
func (i *Iterator[K, V, A, AP]) First() {
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
func (i *Iterator[K, V, A, AP]) Last() {
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

func (i *Iterator[K, V, A, AP]) IncrementPos() bool {
	return i.SetPos(i.pos + 1)
}

func (i *Iterator[K, V, A, AP]) SetPos(pos int16) bool {
	if pos <= i.count && pos >= 0 {
		i.pos = pos
		return true
	}
	return false
}

// Next positions the Iterator to the key immediately following
// its current position.
func (i *Iterator[K, V, A, AP]) Next() {
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
func (i *Iterator[K, V, A, AP]) Prev() {
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
func (i *Iterator[K, V, A, AP]) Valid() bool {
	return i.pos >= 0 && i.pos < i.count
}

// Cur returns the key at the Iterator's current position. It is illegal
// to call Cur if the Iterator is not valid.
func (i *Iterator[K, V, A, AP]) Key() K {
	return i.keys[i.pos]
}

func (i *Iterator[K, V, A, AP]) Value() V {
	return i.values[i.pos]
}

func (i *Iterator[K, V, A, AP]) IsLeaf() bool {
	return i.leaf
}

func (i *Iterator[K, V, A, AP]) Node() Node[*A] {
	return i.node
}

func (i *Iterator[K, V, A, AP]) Pos() int16 {
	return i.pos
}

func (i *Iterator[K, V, A, AP]) makeFrame(n *node[K, V, A, AP], pos int16) iterFrame[K, V, A, AP] {
	return iterFrame[K, V, A, AP]{
		node: n,
		pos:  pos,
	}
}

func (i *Iterator[K, V, A, AP]) Child() (a A, ok bool) {
	if i.Pos() < 0 || i.IsLeaf() {
		return a, false
	}
	return i.children[i.pos].aug, true
}

func (i *Iterator[K, V, A, AP]) Descend() {
	i.s.push(i.iterFrame)
	i.iterFrame = i.makeFrame(i.children[i.pos], 0)
}

// ascend ascends up to the current node's parent and resets the position
// to the one previously set for this parent node.
func (i *Iterator[K, V, A, AP]) Ascend() {
	i.iterFrame = i.s.pop()
}
