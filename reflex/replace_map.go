package reflex

// A mapping from old pointers to new pointers, which follows replacements
// upon insertion.
// The nil value is an empty map, and you can still call methods on it.
type ReplaceMap struct {
	m map[Node]Node
}

// Get checks if the node is in the map, and if so, returns its value.
func (r *ReplaceMap) Get(k Node) (Node, bool) {
	if r == nil {
		return nil, false
	}
	x, ok := r.m[k]
	return x, ok
}

// Updating adds the updates with higher precedence than w, and
// follows replacement chains to simplify the mapping.
func (w *ReplaceMap) Updating(update *ReplaceMap) *ReplaceMap {
	if w == nil {
		return update
	} else if update == nil {
		return w
	}

	inverse := make(map[Node][]Node, len(w.m)+len(update.m))
	mapping := make(map[Node]Node, len(w.m)+len(update.m))
	for _, d := range []*ReplaceMap{w, update} {
		for k, v := range d.m {
			if oldNodes := inverse[k]; d == update && len(oldNodes) > 0 {
				delete(inverse, k)
				for _, oldNode := range oldNodes {
					mapping[oldNode] = v
					inverse[v] = append(inverse[v], oldNode)
				}
			} else {
				mapping[k] = v
				inverse[v] = append(inverse[v], k)
			}
		}
	}
	return &ReplaceMap{m: mapping}
}

// Inserting adds a single replacement to get a new replacement map.
func (w *ReplaceMap) Inserting(newNode Node, newV Node) *ReplaceMap {
	newCount := 1
	if w != nil {
		newCount += len(w.m)
	}
	result := &ReplaceMap{m: make(map[Node]Node, newCount)}
	if w != nil {
		for k, v := range w.m {
			if k == newNode {
				panic("overwriting key")
			}
			if v == newNode {
				panic("insertion out of order")
			}
			result.m[k] = v
		}
	}
	result.m[newNode] = newV
	return result
}
