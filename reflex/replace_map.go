package reflex

import (
	"unsafe"
	"weak"
)

type replaceMapEntry[K any] struct {
	key   weak.Pointer[K]
	value *K
}

// A mapping from old pointers to new pointers, which follows replacements
// upon insertion.
// The nil value represents an empty map, and you have call methods on it.
type ReplaceMap[K any] struct {
	m map[unsafe.Pointer]replaceMapEntry[K]
}

// Get checks if the node is in the map, and if so, returns its value.
func (w *ReplaceMap[K]) Get(k *K) (*K, bool) {
	if value, ok := w.m[unsafe.Pointer(k)]; ok {
		if value.key.Value() == k {
			return value.value, true
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

	inverse := make(map[*K][]*K, len(w.m)+len(update.m))
	mapping := make(map[unsafe.Pointer]replaceMapEntry[K], len(w.m)+len(update.m))
	for _, d := range []*ReplaceMap[K]{w, update} {
		for _, item := range d.m {
			if k := item.key.Value(); k != nil {
				v := item.value
				if oldKs := inverse[k]; d == update && len(oldKs) > 0 {
					delete(inverse, k)
					for _, oldK := range oldKs {
						item.key = mapping[unsafe.Pointer(oldK)].key
						delete(mapping, unsafe.Pointer(oldK))
						k = oldK
						mapping[unsafe.Pointer(k)] = item
						inverse[v] = append(inverse[v], k)
					}
				} else {
					mapping[unsafe.Pointer(k)] = item
					inverse[v] = append(inverse[v], k)
				}
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
	result := &ReplaceMap[K]{
		m: make(map[unsafe.Pointer]replaceMapEntry[K], newCount),
	}
	if w != nil {
		for _, item := range w.m {
			if actualKey := item.key.Value(); actualKey != nil {
				if actualKey == newK {
					panic("overwriting key")
				}
				oldV := item.value
				if oldV == newK {
					panic("insertion out of order")
				}
				result.m[unsafe.Pointer(actualKey)] = item
			}
		}
	}
	result.m[unsafe.Pointer(newK)] = replaceMapEntry[K]{
		key:   weak.Make(newK),
		value: newV,
	}
	return result
}
