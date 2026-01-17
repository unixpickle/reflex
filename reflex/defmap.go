package reflex

// A DefMap is a mapping from attributes to nodes which is potentially lazy.
type DefMap interface {
	// Indicate how many levels of wrapping this mapping is.
	// This approximates how inefficient the mapping is compared to a flat one.
	Depth() int

	// Map creates a map from all the available definitions.
	// This should only be used when flattening a map.
	// The result is owned by the caller, who may modify it.
	//
	// The callee may modify the skip argument.
	Map(skip map[Attr]struct{}) map[Attr]*Node

	// Get a single value from the map.
	Get(k Attr) (*Node, bool)
}

// A FlatDefMap is the simplest DefMap, which is based on a single Go map.
type FlatDefMap struct {
	m map[Attr]*Node
}

func NewFlatDefMap(m map[Attr]*Node) *FlatDefMap {
	return &FlatDefMap{m: m}
}

func (f *FlatDefMap) Depth() int {
	return 1
}

func (f *FlatDefMap) Map(skip map[Attr]struct{}) map[Attr]*Node {
	r := map[Attr]*Node{}
	for k, v := range f.m {
		if _, ok := skip[k]; ok {
			continue
		}
		r[k] = v
	}
	return r
}

func (f *FlatDefMap) Get(k Attr) (*Node, bool) {
	x, ok := f.m[k]
	return x, ok
}

// A CloneDefMap clones the values in the inner mapping and propagates a replacement mapping
// for back edges.
type CloneDefMap struct {
	inner      DefMap
	innerDepth int
	repl       *ReplaceMap[Node]
	cache      map[Attr]*Node
}

func NewCloneDefMap(inner DefMap, repl *ReplaceMap[Node]) *CloneDefMap {
	if c, ok := inner.(*CloneDefMap); ok {
		return &CloneDefMap{
			inner:      c.inner,
			innerDepth: c.innerDepth,
			repl:       c.repl.Updating(repl),
			cache:      map[Attr]*Node{},
		}
	}
	return &CloneDefMap{inner: inner, innerDepth: inner.Depth(), repl: repl, cache: map[Attr]*Node{}}
}

func NewCloneDefMapSingle(inner DefMap, oldNode, newNode *Node) *CloneDefMap {
	var m *ReplaceMap[Node]
	m = m.Inserting(oldNode, newNode)
	return NewCloneDefMap(inner, m)
}

func (c *CloneDefMap) Depth() int {
	return c.innerDepth + 1
}

func (c *CloneDefMap) Map(skip map[Attr]struct{}) map[Attr]*Node {
	newMapping := map[Attr]*Node{}
	for k := range c.inner.Map(skip) {
		newMapping[k], _ = c.Get(k)
	}
	return newMapping
}

func (c *CloneDefMap) Get(k Attr) (*Node, bool) {
	if result, ok := c.cache[k]; ok {
		return result, true
	}
	if v, ok := c.inner.Get(k); ok {
		newV := v.Clone(c.repl)
		c.cache[k] = newV
		return newV, true
	}
	return nil, false
}

// An OverrideDefMap exposes a new set of attributes that shadow previous definitions in
// the inner mapping.
type OverrideDefMap struct {
	inner     DefMap
	overrides DefMap
	maxDepth  int
}

// NewOverrideDefMap creates a mapping where keys from overrides take precedence over
// the corresponding keys from inner.
func NewOverrideDefMap(inner DefMap, overrides DefMap) *OverrideDefMap {
	newDepth := inner.Depth()
	if d := overrides.Depth(); d > newDepth {
		newDepth = d
	}
	return &OverrideDefMap{inner: inner, overrides: overrides, maxDepth: newDepth}
}

func (o *OverrideDefMap) Depth() int {
	return o.maxDepth + 1
}

func (o *OverrideDefMap) Map(skip map[Attr]struct{}) map[Attr]*Node {
	newSkip := map[Attr]struct{}{}
	for k, v := range skip {
		newSkip[k] = v
	}
	result := map[Attr]*Node{}
	for k, v := range o.overrides.Map(skip) {
		newSkip[k] = struct{}{}
		result[k] = v
	}
	for k, v := range o.inner.Map(newSkip) {
		result[k] = v
	}
	return result
}

func (o *OverrideDefMap) Get(k Attr) (*Node, bool) {
	if x, ok := o.overrides.Get(k); ok {
		return x, true
	}
	return o.inner.Get(k)
}

const depthAfterWhichToFlatten = 8

// MaybeFlatten precomputes the values in the DefMap if it is too deep.
func MaybeFlatten(dm DefMap) DefMap {
	if dm.Depth() > depthAfterWhichToFlatten {
		return NewFlatDefMap(dm.Map(nil))
	} else {
		return dm
	}
}
