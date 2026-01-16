package reflex

type Attr int

type AttrTable struct {
	m map[string]Attr
}

func NewAttrTable() *AttrTable {
	return &AttrTable{m: map[string]Attr{}}
}

func (a *AttrTable) Get(name string) Attr {
	if x, ok := a.m[name]; ok {
		return x
	} else {
		a.m[name] = Attr(len(a.m))
		return Attr(len(a.m) - 1)
	}
}

func (a *AttrTable) Name(attr Attr) string {
	for k, v := range a.m {
		if v == attr {
			return k
		}
	}
	panic("unknown attr")
}

type NodeKind int

const (
	NodeKindAccess NodeKind = iota
	NodeKindBlock
	NodeKindOverride
	NodeKindBackEdge
	NodeKindIntLit
	NodeKindStrLit
	NodeKindBuiltInOp
)

func (n NodeKind) String() string {
	switch n {
	case NodeKindAccess:
		return "NodeKindAccess"
	case NodeKindBlock:
		return "NodeKindBlock"
	case NodeKindOverride:
		return "NodeKindOverride"
	case NodeKindBackEdge:
		return "NodeKindBackEdge"
	case NodeKindIntLit:
		return "NodeKindIntLit"
	case NodeKindStrLit:
		return "NodeKindStrLit"
	case NodeKindBuiltInOp:
		return "NodeKindBuiltInOp"
	default:
		panic("no node kind")
	}
}

// A unit value or container inside the interpreter.
type Node struct {
	Kind NodeKind
	Pos  Pos

	BuiltInOp BuiltInOp

	// Access, back edge, or built in op
	Base *Node

	// Access or Identifier
	Attr Attr

	// Block / override
	Defs    DefMap
	Eager   DefMap
	Aliases map[Attr]Attr

	// Literals
	StrLit string
	IntLit int64
}

// Clone creates a copy of the node and applies the replacement map,
// updating it to include the new node for any subnodes.
func (n *Node) Clone(r *ReplaceMap[Node]) *Node {
	newNode := &Node{Kind: n.Kind, Pos: n.Pos}
	switch n.Kind {
	case NodeKindAccess:
		newNode.Base = n.Base.Clone(r)
		newNode.Attr = n.Attr
	case NodeKindBlock:
		newMap := r.Inserting(n, newNode)
		newNode.Defs = MaybeFlatten(NewCloneDefMap(n.Defs, newMap))
	case NodeKindOverride:
		newMap := r.Inserting(n, newNode)
		newNode.Base = n.Base.Clone(r)
		newNode.Defs = MaybeFlatten(NewCloneDefMap(n.Defs, newMap))
		if n.Eager != nil {
			newNode.Eager = MaybeFlatten(NewCloneDefMap(n.Eager, newMap))
		}
		newNode.Aliases = n.Aliases
	case NodeKindBuiltInOp:
		newNode.BuiltInOp = n.BuiltInOp
		fallthrough
	case NodeKindBackEdge:
		if repl, ok := r.Get(n.Base); ok {
			newNode.Base = repl
		} else {
			newNode.Base = n.Base
		}
	case NodeKindStrLit:
		newNode.StrLit = n.StrLit
	case NodeKindIntLit:
		newNode.IntLit = n.IntLit
	}
	return newNode
}

// Defines checks if an attribute is defined in the node.
func (n *Node) Defines(attr Attr) bool {
	for _, defs := range []DefMap{n.Defs, n.Eager} {
		if defs == nil {
			continue
		}
		if _, ok := defs.Get(attr); ok {
			return true
		}
	}
	if n.Aliases != nil {
		if _, ok := n.Aliases[attr]; ok {
			return true
		}
	}
	return false
}

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
