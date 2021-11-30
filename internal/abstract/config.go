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
type Config[K, Aux any] struct {

	// Config is the configuration provided by the instantiator of the
	// tree.
	Aux Aux

	cmp func(K, K) int
}

// Compare compares two values using the same comparison function as the Map.
func (c *Config[K, Aux]) Compare(a, b K) int { return c.cmp(a, b) }

type config[K, V, Aux, A any, AP Aug[K, Aux, A]] struct {
	Config[K, Aux]
	np *nodePool[K, V, Aux, A, AP]
}

func makeConfig[K, V, Aux, A any, AP Aug[K, Aux, A]](
	aux Aux, cmp func(K, K) int,
) (c config[K, V, Aux, A, AP]) {
	c.Aux = aux
	c.cmp = cmp
	c.np = getNodePool[K, V, Aux, A, AP]()
	return c
}
