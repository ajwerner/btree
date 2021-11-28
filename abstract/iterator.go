package abstract

// Iterator is responsible for search and traversal within a AugBTree.
type Iterator[T Item[T], A any, AP Aug[T, A]] struct {
	r *node[T, A, AP]
	iterFrame[T, A, AP]
	s iterStack[T, A, AP]
	// TODO(ajwerner): Add back augmented search
}

func (i *Iterator[T, A, AP]) Reset() {
	i.node = i.r
	i.pos = -1
	i.s.reset()
}

func (i *Iterator[T, A, AP]) Pos() int16 {
	return i.pos
}

func (i *Iterator[T, A, AP]) makeFrame(n *node[T, A, AP], pos int16) iterFrame[T, A, AP] {
	return iterFrame[T, A, AP]{
		node: n,
		pos:  pos,
	}
}

func (i *Iterator[T, A, AP]) CurChild() Node[*A] {
	if i.Pos() < 0 || i.IsLeaf() {
		return nil
	}
	return i.children[i.pos]
}

func (i *Iterator[T, A, AP]) Descend() {
	i.s.push(i.iterFrame)
	i.iterFrame = i.makeFrame(i.children[i.pos], 0)
}

// ascend ascends up to the current node's parent and resets the position
// to the one previously set for this parent node.
func (i *Iterator[T, A, AP]) Ascend() {
	i.iterFrame = i.s.pop()
}

// SeekGE seeks to the first item greater-than or equal to the provided
// item.
func (i *Iterator[T, A, AP]) SeekGE(item T) {
	i.Reset()
	if i.node == nil {
		return
	}
	for {
		pos, found := i.find(item)
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

// SeekLT seeks to the first item less-than the provided item.
func (i *Iterator[T, A, AP]) SeekLT(item T) {
	i.Reset()
	if i.node == nil {
		return
	}
	for {
		pos, found := i.find(item)
		i.pos = int16(pos)
		if found || i.leaf {
			i.Prev()
			return
		}
		i.Descend()
	}
}

// First seeks to the first item in the AugBTree.
func (i *Iterator[T, A, AP]) First() {
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

// Last seeks to the last item in the AugBTree.
func (i *Iterator[T, A, AP]) Last() {
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

func (i *Iterator[T, A, AP]) IncrementPos() bool {
	return i.SetPos(i.pos + 1)
}

func (i *Iterator[T, A, AP]) SetPos(pos int16) bool {
	if pos <= i.count && pos >= 0 {
		i.pos = pos
		return true
	}
	return false
}

// Next positions the Iterator to the item immediately following
// its current position.
func (i *Iterator[T, A, AP]) Next() {
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

func (i *Iterator[T, A, AP]) IsLeaf() bool {
	return i.leaf
}

func (i *Iterator[T, A, AP]) Node() Node[*A] {
	return i.node
}

// Prev positions the Iterator to the item immediately preceding
// its current position.
func (i *Iterator[T, A, AP]) Prev() {
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
func (i *Iterator[T, A, AP]) Valid() bool {
	return i.pos >= 0 && i.pos < i.count
}

// Cur returns the item at the Iterator's current position. It is illegal
// to call Cur if the Iterator is not valid.
func (i *Iterator[T, A, AP]) Cur() T {
	return i.items[i.pos]
}

// iterStack represents a stack of (node, pos) tuples, which captures
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
	*node[T, A, AP]
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
