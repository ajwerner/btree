package abstract

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
