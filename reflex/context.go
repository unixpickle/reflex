package reflex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Context struct {
	Attrs      *AttrTable
	intProto   *NodeBlock
	floatProto *NodeBlock
	strProto   *NodeBlock
	bytesProto *NodeBlock
	builtIns   map[string]*NodeBlock

	cachedImports map[string]*NodeBackEdge
}

func NewContext() *Context {
	res := &Context{Attrs: NewAttrTable()}

	// The order matters; strProto creates an int.
	res.intProto = intNode(res)
	res.floatProto = floatNode(res)
	res.strProto = strNode(res)
	res.bytesProto = bytesNode(res)

	res.builtIns = map[string]*NodeBlock{
		"errors":      createErrors(res),
		"io":          createIO(res),
		"collections": createCollections(res),
	}
	res.cachedImports = map[string]*NodeBackEdge{}

	return res
}

// IntNode creates an integer with all of the built-in methods.
func (c *Context) IntNode(pos Pos, lit int64) Node {
	clone := c.intProto.Clone(nil).(*NodeBlock)
	clone.P = pos
	clone.Defs = NewOverrideDefMap(clone.Defs, NewFlatDefMap(map[Attr]Node{
		c.Attrs.Get("_inner"): &NodeIntLit{
			LitBase: LitBase{NodeBase: NodeBase{P: pos}},
			Lit:     lit,
		},
	}))
	clone.Defs = MaybeFlatten(clone.Defs)
	return clone
}

// FloatNode creates a floar with all of the built-in methods.
func (c *Context) FloatNode(pos Pos, lit float64) Node {
	clone := c.floatProto.Clone(nil).(*NodeBlock)
	clone.P = pos
	clone.Defs = NewOverrideDefMap(clone.Defs, NewFlatDefMap(map[Attr]Node{
		c.Attrs.Get("_inner"): &NodeFloatLit{
			LitBase: LitBase{NodeBase: NodeBase{P: pos}},
			Lit:     lit,
		},
	}))
	clone.Defs = MaybeFlatten(clone.Defs)
	return clone
}

// StrNode creates a string with all of the built-in methods.
func (c *Context) StrNode(pos Pos, lit string) Node {
	clone := c.strProto.Clone(nil).(*NodeBlock)
	clone.P = pos
	clone.Defs = NewOverrideDefMap(clone.Defs, NewFlatDefMap(map[Attr]Node{
		c.Attrs.Get("_inner"): &NodeStrLit{
			LitBase: LitBase{NodeBase: NodeBase{P: pos}},
			Lit:     lit,
		},
	}))
	clone.Defs = MaybeFlatten(clone.Defs)
	return clone
}

// BytesNode creates a byte slice with all of the built-in methods.
func (c *Context) BytesNode(pos Pos, lit []byte) Node {
	clone := c.bytesProto.Clone(nil).(*NodeBlock)
	clone.P = pos
	clone.Defs = NewOverrideDefMap(clone.Defs, NewFlatDefMap(map[Attr]Node{
		c.Attrs.Get("_inner"): &NodeBytesLit{
			LitBase: LitBase{NodeBase: NodeBase{P: pos}},
			Lit:     lit,
		},
	}))
	clone.Defs = MaybeFlatten(clone.Defs)
	return clone
}

func (c *Context) Maybe(pos Pos, result Node, err error) Node {
	maybe, ok := c.builtIns["errors"].Defs.Get(c.Attrs.Get("maybe"))
	if !ok {
		panic("errors module does not have attribute 'maybe'")
	}
	clone := maybe.Clone(nil).(*NodeBlock)
	clone.P = pos
	m := map[Attr]Node{
		c.Attrs.Get("success"): c.IntNode(pos, boolToInt(err == nil)),
	}
	if err != nil {
		m[c.Attrs.Get("error")] = c.StrNode(pos, err.Error())
	}
	if result != nil {
		m[c.Attrs.Get("result")] = result.Clone(nil)
	} else if err == nil {
		// Always return some block, even if it's empty.
		m[c.Attrs.Get("result")] = &NodeBlock{
			NodeBase: NodeBase{P: pos},
			Defs:     NewFlatDefMap(nil),
		}
	}
	clone.Defs = MaybeFlatten(NewOverrideDefMap(clone.Defs, NewFlatDefMap(m)))
	return clone
}

func (c *Context) Import(pos Pos, relPath string) (Node, error) {
	switch relPath {
	case "stdlib/io", "stdlib/collections", "stdlib/errors":
		name := strings.Split(relPath, "/")[1]
		return &NodeBackEdge{
			NodeBase: NodeBase{P: pos},
			Ref:      c.builtIns[name],
		}, nil
	default:
		absBase, err := filepath.Abs(pos.File)
		if err != nil {
			return nil, err
		}
		rp := filepath.Join(filepath.Dir(absBase), relPath)
		absPath, err := filepath.Abs(rp)
		if err != nil {
			return nil, err
		}
		if node, ok := c.cachedImports[absPath]; ok {
			return node, nil
		}
		data, err := os.ReadFile(absPath)
		if err != nil {
			return nil, err
		}
		toks, err := Tokenize(absPath, string(data))
		if err != nil {
			return nil, fmt.Errorf("failed to tokenize %#v: %w", absPath, err)
		}
		ast, err := Parse(toks)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %#v: %w", absPath, err)
		}
		node, err := ast.Node(c, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %#v: %w", absPath, err)
		}
		n := &NodeBackEdge{
			NodeBase: NodeBase{P: pos},
			Ref:      node,
		}
		c.cachedImports[absPath] = n
		return n, nil
	}
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
