// Copyright 2021 Andrew Werner.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package orderstat

import "github.com/ajwerner/btree/new/abstract"

type aug[K any] struct {
	// children is the number of items rooted at the current subtree.
	children int
}

func (a *aug[T]) CopyInto(dest *aug[T]) { *dest = *a }

// Update will update the count for the current node.
func (a *aug[T]) Update(
	n abstract.Node[T, *aug[T]], _ abstract.UpdateMeta[T, struct{}, aug[T]],
) (updated bool) {
	orig := a.children
	var children int
	if !n.IsLeaf() {
		N := n.Count()
		for i := int16(0); i <= N; i++ {
			if child := n.GetChild(i); child != nil {
				children += child.children
			}
		}
	}
	children += int(n.Count())
	a.children = children
	return a.children != orig
}
