package orderstat

import "github.com/ajwerner/btree/new/abstract"

type aug[K any] struct {
	// children is the number of items rooted at the current subtree.
	children int
}

func (a *aug[T]) CopyInto(dest *aug[T]) { *dest = *a }

// Update will update the count for the current node.
func (a *aug[T]) Update(
	n abstract.Node[*aug[T]], _ abstract.UpdateMeta[T, struct{}, aug[T]],
) (updated bool) {
	orig := a.children
	var children int
	if !n.IsLeaf() {
		N := n.Count()
		for i := int16(0); i <= N; i++ {
			if child := n.GetChild(i); child != nil {
				children += child.GetA().children
			}
		}
	}
	children += int(n.Count())
	a.children = children
	return a.children != orig
}
