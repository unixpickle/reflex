package reflex

import "weak"

// A mapping from old pointers to new pointers, which follows replacements
// upon insertion.
// The nil value represents an empty map, and you have call methods on it.
type ReplaceMap[K any] struct {
	keys   []weak.Pointer[K]
	values []*K
}

// Get checks if the node is in the map, and if so, returns its value.
func (w *ReplaceMap[K]) Get(k *K) (*K, bool) {
	for i, x := range w.keys {
		if x.Value() == k {
			return w.values[i], true
		}
	}
	return nil, false
}

// Updating adds the updates with higher precedence than w, and
// follows replacement chains to simplify the mapping.
func (w *ReplaceMap[K]) Updating(update *ReplaceMap[K]) *ReplaceMap[K] {
	if w == nil {
		return update
	} else if update == nil {
		return w
	}

	inverse := make(map[*K]*K, len(w.keys)+len(update.keys))
	mapping := make(map[*K]*K, len(w.keys)+len(update.keys))
	for _, d := range []*ReplaceMap[K]{w, update} {
		for i, weakK := range d.keys {
			if k := weakK.Value(); k != nil {
				v := d.values[i]
				if d == update {
					if oldK, ok := inverse[k]; ok {
						delete(mapping, oldK)
						delete(inverse, k)
						k = oldK
					}
				}
				mapping[k] = v
				inverse[v] = k
			}
		}
	}
	result := new(ReplaceMap[K])
	for k, v := range mapping {
		result.keys = append(result.keys, weak.Make(k))
		result.values = append(result.values, v)
	}
	return result
}

// Inserting adds a single replacement to get a new replacement map.
func (w *ReplaceMap[K]) Inserting(newK *K, newV *K) *ReplaceMap[K] {
	newCount := 1
	if w != nil {
		newCount += len(w.keys)
	}
	newKeys := make([]weak.Pointer[K], 0, newCount)
	newValues := make([]*K, 0, newCount)
	var found bool
	if w != nil {
		for i, k := range w.keys {
			if actualKey := k.Value(); actualKey != nil {
				if actualKey == newK {
					continue
				}
				oldV := w.values[i]
				if oldV == newK {
					found = true
					newKeys = append(newKeys, k)
					newValues = append(newValues, newV)
				} else {
					newKeys = append(newKeys, k)
					newValues = append(newValues, w.values[i])
				}
			}
		}
	}
	if !found {
		newKeys = append(newKeys, weak.Make(newK))
		newValues = append(newValues, newV)
	}
	return &ReplaceMap[K]{keys: newKeys, values: newValues}
}
