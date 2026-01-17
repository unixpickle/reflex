package reflex

type Context struct {
	Attrs    *AttrTable
	intProto *Node
	strProto *Node
}

func NewContext() *Context {
	res := &Context{Attrs: NewAttrTable()}

	// The order matters; strProto creates an int.
	res.intProto = intNode(res)
	res.strProto = strNode(res)

	return res
}

// IntNode creates an integer with all of the built-in methods.
func (c *Context) IntNode(pos Pos, lit int64) *Node {
	clone := c.intProto.Clone(nil)
	clone.Pos = pos
	clone.Defs = NewOverrideDefMap(clone.Defs, NewFlatDefMap(map[Attr]*Node{
		c.Attrs.Get("_inner"): &Node{
			Kind:   NodeKindIntLit,
			Pos:    pos,
			IntLit: lit,
		},
	}))
	clone.Defs = MaybeFlatten(clone.Defs)
	return clone
}

// StrNode creates a string with all of the built-in methods.
func (c *Context) StrNode(pos Pos, lit string) *Node {
	clone := c.strProto.Clone(nil)
	clone.Pos = pos
	clone.Defs = NewOverrideDefMap(clone.Defs, NewFlatDefMap(map[Attr]*Node{
		c.Attrs.Get("_inner"): &Node{
			Kind:   NodeKindStrLit,
			Pos:    pos,
			StrLit: lit,
		},
	}))
	clone.Defs = MaybeFlatten(clone.Defs)
	return clone
}

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
