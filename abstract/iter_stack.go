package abstract

// iterStack represents a stack of (node, pos) tuples, which captures
// iteration state as an Iterator descends a AugBTree.
type iterStack[K, V, A any, AP Aug[K, A]] struct {
	a    iterStackArr[K, V, A, AP]
	aLen int16 // -1 when using s
	s    []iterFrame[K, V, A, AP]
}

const iterStackDepth = 6

// Used to avoid allocations for stacks below a certain size.
type iterStackArr[K, V, A any, AP Aug[K, A]] [iterStackDepth]iterFrame[K, V, A, AP]

type iterFrame[K, V, A any, AP Aug[K, A]] struct {
	*node[K, V, A, AP]
	pos int16
}

func (is *iterStack[K, V, A, AP]) push(f iterFrame[K, V, A, AP]) {
	if is.aLen == -1 {
		is.s = append(is.s, f)
	} else if int(is.aLen) == len(is.a) {
		is.s = make([](iterFrame[K, V, A, AP]), int(is.aLen)+1, 2*int(is.aLen))
		copy(is.s, is.a[:])
		is.s[int(is.aLen)] = f
		is.aLen = -1
	} else {
		is.a[is.aLen] = f
		is.aLen++
	}
}

func (is *iterStack[K, V, A, AP]) pop() iterFrame[K, V, A, AP] {
	if is.aLen == -1 {
		f := is.s[len(is.s)-1]
		is.s = is.s[:len(is.s)-1]
		return f
	}
	is.aLen--
	return is.a[is.aLen]
}

func (is *iterStack[K, V, A, AP]) len() int {
	if is.aLen == -1 {
		return len(is.s)
	}
	return int(is.aLen)
}

func (is *iterStack[K, V, A, AP]) reset() {
	if is.aLen == -1 {
		is.s = is.s[:0]
	} else {
		is.aLen = 0
	}
}
