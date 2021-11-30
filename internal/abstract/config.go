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

package abstract

// Config is used to configure the tree. It consists of a comparison function
// for keys and any auxiliary data provided by the instantiator. It is provided
// on the iterator and passed to the augmentation's Update method.
type Config[K, A any] struct {

	// Updater is used to update the augmentations to the tree.
	Updater Updater[K, A]

	cmp func(K, K) int
}

type Updater[K, A any] interface {
	Update(Node[K, A], UpdateMeta[K, A]) (changed bool)
}

// Compare compares two values using the same comparison function as the Map.
func (c *Config[K, Aux]) Compare(a, b K) int { return c.cmp(a, b) }

type config[K, V, A any] struct {
	Config[K, A]
	np *nodePool[K, V, A]
}

func makeConfig[K, V, A any](
	cmp func(K, K) int, up Updater[K, A],
) (c config[K, V, A]) {
	c.Updater = up
	c.cmp = cmp
	c.np = getNodePool[K, V, A]()
	return c
}
