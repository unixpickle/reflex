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

// Insert a single replacement to get a new replacement map.
func (w *ReplaceMap[K]) Inserting(newK *K, newV *K) *ReplaceMap[K] {
	var newKeys []weak.Pointer[K]
	var newValues []*K
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
