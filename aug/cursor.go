// Copyright 2026 Andrew Werner.
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

package aug

// Cursor is an Iterator that can also mutate the Map at its position and
// remain valid afterwards. Its mutations are writes to the Map: they must be
// serialized with every other write, and they invalidate every other
// Iterator or Cursor on the Map. Any number of Cursors may be used one after
// another; positioning a Cursor (First, SeekGE, ...) after a mutation through
// another one is always safe.
//
// Mutations act in place when the cursor is on a leaf that can absorb the
// change, maintaining augmentations along the path from the root, and
// otherwise fall back to the ordinary top-down algorithm followed by a
// re-seek. Either way the cursor ends up positioned as documented on each
// method.
type Cursor[K, V, A any] struct {
	Iterator[K, V, A]
}

// Cursor returns a new Cursor positioned before the first entry.
func (t *Map[K, V, A]) Cursor() Cursor[K, V, A] {
	return Cursor[K, V, A]{Iterator: t.Iterator()}
}

// pin makes every node on the cursor's path uniquely owned by the Map,
// copying shared nodes, so that they can be mutated in place.
func (c *Cursor[K, V, A]) pin() {
	t := c.r
	depth := c.s.len()
	if depth == 0 {
		c.node = mut(&t.cfg, &t.root)
		return
	}
	parent := mut(&t.cfg, &t.root)
	c.s.setNode(0, parent)
	for d := 1; d < depth; d++ {
		f := c.s.at(d - 1)
		parent = mut(&t.cfg, &parent.children[f.pos])
		c.s.setNode(d, parent)
	}
	f := c.s.at(depth - 1)
	c.node = mut(&t.cfg, &parent.children[f.pos])
}

// updatePath applies md to the current node and then to each ancestor,
// stopping as soon as an Updater reports no change.
func (c *Cursor[K, V, A]) updatePath(md UpdateInfo[K, V, A]) {
	up := c.r.cfg.Updater
	if up == nil {
		return
	}
	if !up.Update(c.node, md) {
		return
	}
	for d := c.s.len() - 1; d >= 0; d-- {
		if !up.Update(c.s.at(d).node, md) {
			return
		}
	}
}

// bounds returns the keys bracketing the current node's key range, taken
// from the ancestors' separator entries. A nil bound is unbounded.
func (c *Cursor[K, V, A]) bounds() (lo, hi *K) {
	for d := c.s.len() - 1; d >= 0 && (lo == nil || hi == nil); d-- {
		f := c.s.at(d)
		if lo == nil && f.pos > 0 {
			lo = &f.node.keys[f.pos-1]
		}
		if hi == nil && int(f.pos) < len(f.node.keys) {
			hi = &f.node.keys[f.pos]
		}
	}
	return lo, hi
}

// within reports whether k lies strictly between lo and hi.
func (c *Cursor[K, V, A]) within(k K, lo, hi *K) bool {
	cmp := c.r.cfg.cmp
	return (lo == nil || cmp(*lo, k) < 0) && (hi == nil || cmp(k, *hi) < 0)
}

// SetValue replaces the value of the current entry. The cursor must be
// valid. The cursor stays on the entry.
func (c *Cursor[K, V, A]) SetValue(v V) {
	if !c.Valid() {
		panic("aug: SetValue on an invalid Cursor")
	}
	c.pin()
	n := c.node
	k, prev := n.keys[c.pos], n.values[c.pos]
	n.values[c.pos] = v
	c.updatePath(UpdateInfo[K, V, A]{
		Action:        Replacement,
		RelevantKey:   k,
		RelevantValue: v,
		PrevKey:       k,
		PrevValue:     prev,
	})
}

// Delete removes the current entry and returns it. The cursor must be
// valid. Afterwards the cursor is on the entry that followed the removed
// one, or past the end if there was none, so a loop that deletes must not
// also call Next for the deleted entry:
//
//	for ok := c.First(); ok; {
//		if drop(c.Key()) {
//			c.Delete()
//			ok = c.Valid()
//		} else {
//			ok = c.Next()
//		}
//	}
func (c *Cursor[K, V, A]) Delete() (K, V) {
	if !c.Valid() {
		panic("aug: Delete on an invalid Cursor")
	}
	t := c.r
	n := c.node
	// In place: a leaf that stays at or above the minimum, or the root leaf.
	if n.IsLeaf() && (len(n.keys) > t.cfg.minEntries || c.s.len() == 0) {
		c.pin()
		n = c.node
		k, v, _ := n.removeAt(int(c.pos))
		t.length--
		if len(n.keys) == 0 {
			// The root leaf is now empty.
			t.root = nil
			n.decRef(&t.cfg, false /* recursive */)
			c.node = nil
			return k, v
		}
		c.updatePath(UpdateInfo[K, V, A]{Action: Removal, RelevantKey: k, RelevantValue: v})
		// The successor is at the same position, unless that runs off the
		// leaf, in which case it is the separator in an ancestor.
		c.settle()
		return k, v
	}
	k := c.Key()
	_, v, _ := t.Delete(k)
	c.SeekGE(k)
	return k, v
}

// Rekey moves the current entry to key k, keeping its value, and returns
// any entry that had key k and was replaced. The cursor must be valid.
// Afterwards the cursor is on the moved entry. When k still sorts between
// the entry's neighbours the move is in place and the Updater sees a
// Replacement whose key changed; otherwise it sees the Removal and
// Insertion of the fallback.
func (c *Cursor[K, V, A]) Rekey(k K) (replacedK K, replacedV V, replaced bool) {
	if !c.Valid() {
		panic("aug: Rekey on an invalid Cursor")
	}
	n := c.node
	if n.IsLeaf() {
		// In place if k still sorts between the neighbours of the entry.
		lo, hi := c.bounds()
		if c.pos > 0 {
			lo = &n.keys[c.pos-1]
		}
		if int(c.pos)+1 < len(n.keys) {
			hi = &n.keys[c.pos+1]
		}
		if c.within(k, lo, hi) {
			c.pin()
			n := c.node
			prev, v := n.keys[c.pos], n.values[c.pos]
			n.keys[c.pos] = k
			c.updatePath(UpdateInfo[K, V, A]{
				Action:        Replacement,
				RelevantKey:   k,
				RelevantValue: v,
				PrevKey:       prev,
				PrevValue:     v,
			})
			return replacedK, replacedV, false
		}
	}
	// Otherwise remove it and re-insert it near the cursor, which needs no
	// further descent when the new position is in the same leaf.
	_, v := c.Delete()
	return c.Upsert(k, v)
}

// Upsert inserts or replaces the entry with key k, using the cursor's
// position as a hint, and returns any replaced entry, as Map.Upsert does.
// Afterwards the cursor is on that entry.
func (c *Cursor[K, V, A]) Upsert(k K, v V) (replacedK K, replacedV V, replaced bool) {
	t := c.r
	n := c.node
	if n != nil && n.IsLeaf() && len(n.keys) < t.cfg.maxEntries {
		if lo, hi := c.bounds(); c.within(k, lo, hi) {
			i, found := n.find(&t.cfg, k)
			c.pin()
			n = c.node
			c.pos = int16(i)
			if found {
				// Replace the key as well as the value: it compares equal
				// but need not be identical.
				replacedK, replacedV = n.keys[i], n.values[i]
				n.keys[i], n.values[i] = k, v
				c.updatePath(UpdateInfo[K, V, A]{
					Action:        Replacement,
					RelevantKey:   k,
					RelevantValue: v,
					PrevKey:       replacedK,
					PrevValue:     replacedV,
				})
				return replacedK, replacedV, true
			}
			n.insertAt(i, k, v, nil)
			t.length++
			c.updatePath(UpdateInfo[K, V, A]{Action: Insertion, RelevantKey: k, RelevantValue: v})
			return replacedK, replacedV, false
		}
	}
	replacedK, replacedV, replaced = t.Upsert(k, v)
	c.SeekGE(k)
	return replacedK, replacedV, replaced
}
