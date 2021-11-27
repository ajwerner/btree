package abstract

// Iterator is responsible for search and traversal within a AugBTree.
type Iterator[T Item[T], A any, AP Aug[T, A]] struct {
	r   *node[T, A, AP]
	n   *node[T, A, AP]
	Pos int16
	s   iterStack[T, A, AP]
	// TODO(ajwerner): Add back augmented search
}

func (i *Iterator[T, A, AP]) Reset() {
	i.n = i.r
	i.Pos = -1
	i.s.reset()
}

func (i *Iterator[T, A, AP]) Descend(n *node[T, A, AP], pos int16) {
	i.s.push(iterFrame[T, A, AP]{n: n, pos: pos})
	i.n = n.children[pos]
	i.Pos = 0
}

// ascend ascends up to the current node's parent and resets the position
// to the one previously set for this parent node.
func (i *Iterator[T, A, AP]) Ascend() {
	f := i.s.pop()
	i.n = f.n
	i.Pos = f.pos
}

// SeekGE seeks to the first item greater-than or equal to the provided
// item.
func (i *Iterator[T, A, AP]) SeekGE(item T) {
	i.Reset()
	if i.n == nil {
		return
	}
	for {
		pos, found := i.n.find(item)
		i.Pos = int16(pos)
		if found {
			return
		}
		if i.n.leaf {
			if i.Pos == i.n.count {
				i.Next()
			}
			return
		}
		i.Descend(i.n, i.Pos)
	}
}

// SeekLT seeks to the first item less-than the provided item.
func (i *Iterator[T, A, AP]) SeekLT(item T) {
	i.Reset()
	if i.n == nil {
		return
	}
	for {
		pos, found := i.n.find(item)
		i.Pos = int16(pos)
		if found || i.n.leaf {
			i.Prev()
			return
		}
		i.Descend(i.n, i.Pos)
	}
}

// First seeks to the first item in the AugBTree.
func (i *Iterator[T, A, AP]) First() {
	i.Reset()
	if i.n == nil {
		return
	}
	for !i.n.leaf {
		i.Descend(i.n, 0)
	}
	i.Pos = 0
}

// Last seeks to the last item in the AugBTree.
func (i *Iterator[T, A, AP]) Last() {
	i.Reset()
	if i.n == nil {
		return
	}
	for !i.n.leaf {
		i.Descend(i.n, i.n.count)
	}
	i.Pos = i.n.count - 1
}

// Next positions the Iterator to the item immediately following
// its current position.
func (i *Iterator[T, A, AP]) Next() {
	if i.n == nil {
		return
	}

	if i.n.leaf {
		i.Pos++
		if i.Pos < i.n.count {
			return
		}
		for i.s.len() > 0 && i.Pos >= i.n.count {
			i.Ascend()
		}
		return
	}

	i.Descend(i.n, i.Pos+1)
	for !i.n.leaf {
		i.Descend(i.n, 0)
	}
	i.Pos = 0
}

func (i *Iterator[T, A, AP]) IsLeaf() bool {
	return i.n.leaf
}

func (i *Iterator[T, A, AP]) Node() Node[*A] {
	return i.n
}

// Prev positions the Iterator to the item immediately preceding
// its current position.
func (i *Iterator[T, A, AP]) Prev() {
	if i.n == nil {
		return
	}

	if i.n.leaf {
		i.Pos--
		if i.Pos >= 0 {
			return
		}
		for i.s.len() > 0 && i.Pos < 0 {
			i.Ascend()
			i.Pos--
		}
		return
	}

	i.Descend(i.n, i.Pos)
	for !i.n.leaf {
		i.Descend(i.n, i.n.count)
	}
	i.Pos = i.n.count - 1
}

// Valid returns whether the Iterator is positioned at a valid position.
func (i *Iterator[T, A, AP]) Valid() bool {
	return i.Pos >= 0 && i.Pos < i.n.count
}

// Cur returns the item at the Iterator's current position. It is illegal
// to call Cur if the Iterator is not valid.
func (i *Iterator[T, A, AP]) Cur() T {
	return i.n.items[i.Pos]
}

// iterStack represents a stack of (node, Pos) tuples, which captures
// iteration state as an Iterator descends a AugBTree.
type iterStack[T Item[T], A any, AP Aug[T, A]] struct {
	a    iterStackArr[T, A, AP]
	aLen int16 // -1 when using s
	s    []iterFrame[T, A, AP]
}

const iterStackDepth = 6

// Used to avoid allocations for stacks below a certain size.
type iterStackArr[T Item[T], A any, AP Aug[T, A]] [iterStackDepth]iterFrame[T, A, AP]

type iterFrame[T Item[T], A any, AP Aug[T, A]] struct {
	n   *node[T, A, AP]
	pos int16
}

func (is *iterStack[T, A, AP]) push(f iterFrame[T, A, AP]) {
	if is.aLen == -1 {
		is.s = append(is.s, f)
	} else if int(is.aLen) == len(is.a) {
		is.s = make([](iterFrame[T, A, AP]), int(is.aLen)+1, 2*int(is.aLen))
		copy(is.s, is.a[:])
		is.s[int(is.aLen)] = f
		is.aLen = -1
	} else {
		is.a[is.aLen] = f
		is.aLen++
	}
}

func (is *iterStack[T, A, AP]) pop() iterFrame[T, A, AP] {
	if is.aLen == -1 {
		f := is.s[len(is.s)-1]
		is.s = is.s[:len(is.s)-1]
		return f
	}
	is.aLen--
	return is.a[is.aLen]
}

func (is *iterStack[T, A, AP]) len() int {
	if is.aLen == -1 {
		return len(is.s)
	}
	return int(is.aLen)
}

func (is *iterStack[T, A, AP]) reset() {
	if is.aLen == -1 {
		is.s = is.s[:0]
	} else {
		is.aLen = 0
	}
}
