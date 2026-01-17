package reflex

// A mapping from old pointers to new pointers, which follows replacements
// upon insertion.
// The nil value is an empty map, and you can still call methods on it.
type ReplaceMap[K any] struct {
	m map[*K]*K
}

// Get checks if the node is in the map, and if so, returns its value.
func (r *ReplaceMap[K]) Get(k *K) (*K, bool) {
	if r == nil {
		return nil, false
	}
	x, ok := r.m[k]
	return x, ok
}

// Updating adds the updates with higher precedence than w, and
// follows replacement chains to simplify the mapping.
func (w *ReplaceMap[K]) Updating(update *ReplaceMap[K]) *ReplaceMap[K] {
	if w == nil {
		return update
	} else if update == nil {
		return w
	}

	inverse := make(map[*K][]*K, len(w.m)+len(update.m))
	mapping := make(map[*K]*K, len(w.m)+len(update.m))
	for _, d := range []*ReplaceMap[K]{w, update} {
		for k, v := range d.m {
			if oldKs := inverse[k]; d == update && len(oldKs) > 0 {
				delete(inverse, k)
				for _, oldK := range oldKs {
					mapping[oldK] = v
					inverse[v] = append(inverse[v], oldK)
				}
			} else {
				mapping[k] = v
				inverse[v] = append(inverse[v], k)
			}
		}
	}
	return &ReplaceMap[K]{m: mapping}
}

// Inserting adds a single replacement to get a new replacement map.
func (w *ReplaceMap[K]) Inserting(newK *K, newV *K) *ReplaceMap[K] {
	newCount := 1
	if w != nil {
		newCount += len(w.m)
	}
	result := &ReplaceMap[K]{m: make(map[*K]*K, newCount)}
	if w != nil {
		for k, v := range w.m {
			if k == newK {
				panic("overwriting key")
			}
			if v == newK {
				panic("insertion out of order")
			}
			result.m[k] = v
		}
	}
	result.m[newK] = newV
	return result
}
