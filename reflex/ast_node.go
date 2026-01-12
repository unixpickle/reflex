package reflex

import "fmt"

type ASTError struct {
	Msg string
	Pos Pos
}

func (p *ASTError) Error() string {
	return fmt.Sprintf("%s at %s", p.Msg, p.Pos)
}

type ASTNode interface {
	Node(attrs *AttrTable, parents []*Node) (*Node, error)
}

type ASTParent struct {
	Pos   Pos
	Depth int
}

func (a *ASTParent) Node(attrs *AttrTable, parents []*Node) (*Node, error) {
	if a.Depth+1 > len(parents) {
		return nil, &ASTError{Msg: "parent access goes beyond top scope", Pos: a.Pos}
	}
	return &Node{
		Kind: NodeKindBackEdge,
		Pos:  a.Pos,
		Base: parents[len(parents)-(a.Depth+1)],
	}, nil
}

type ASTSelfRef struct {
	Pos Pos
}

func (a *ASTSelfRef) Node(attrs *AttrTable, parents []*Node) (*Node, error) {
	return &Node{
		Kind: NodeKindBackEdge,
		Pos:  a.Pos,
		Base: parents[len(parents)-1],
	}, nil
}

type ASTAccess struct {
	Pos  Pos
	Base ASTNode
	Attr string
}

func (a *ASTAccess) Node(attrs *AttrTable, parents []*Node) (*Node, error) {
	baseNode, err := a.Base.Node(attrs, parents)
	if err != nil {
		return nil, err
	}
	return &Node{
		Kind: NodeKindAccess,
		Pos:  a.Pos,
		Base: baseNode,
		Attr: attrs.Get(a.Attr),
	}, nil
}

type ASTIdentifier struct {
	Pos  Pos
	Name string
}

func (a *ASTIdentifier) Node(attrs *AttrTable, parents []*Node) (*Node, error) {
	return (&ASTAccess{Pos: a.Pos, Base: &ASTSelfRef{Pos: a.Pos}, Attr: a.Name}).Node(attrs, parents)
}

type ASTAncestorLookup struct {
	Pos  Pos
	Attr string
}

func (a *ASTAncestorLookup) Node(attrs *AttrTable, parents []*Node) (*Node, error) {
	attr := attrs.Get(a.Attr)
	for i := len(parents) - 2; i >= 0; i-- {
		parent := parents[i]
		if parent.Defines(attr) {
			return &Node{
				Kind: NodeKindBackEdge,
				Pos:  a.Pos,
				Base: parent,
			}, nil
		}
	}
	return nil, &ASTError{
		Msg: fmt.Sprintf("no ancestor with attribute %#v found", a.Attr),
		Pos: a.Pos,
	}
}

type ASTBlock struct {
	Pos  Pos
	Defs map[string]ASTNode
}

func (a *ASTBlock) Node(attrs *AttrTable, parents []*Node) (*Node, error) {
	n := &Node{
		Kind: NodeKindBlock,
		Pos:  a.Pos,
		Defs: dummyDefs(attrs, a.Defs),
	}
	if defs, err := instantiateDefs(attrs, parents, n, a.Defs); err != nil {
		return nil, err
	} else {
		n.Defs = defs
	}
	return n, nil
}

func dummyDefs[V any](attrs *AttrTable, d map[string]V) DefMap {
	m := map[Attr]*Node{}
	for k := range d {
		m[attrs.Get(k)] = nil
	}
	return NewFlatDefMap(m)
}

func instantiateDefs(attrs *AttrTable, parents []*Node, n *Node, defs map[string]ASTNode) (DefMap, error) {
	newDefs := map[Attr]*Node{}
	newParents := append(parents, n)
	for k, v := range defs {
		newNode, err := v.Node(attrs, newParents)
		if err != nil {
			return nil, err
		}
		newDefs[attrs.Get(k)] = newNode
	}
	return NewFlatDefMap(newDefs), nil
}

type ASTOverride struct {
	Pos     Pos
	Base    ASTNode
	Defs    map[string]ASTNode
	Aliases map[string]string
}

func (a *ASTOverride) Node(attrs *AttrTable, parents []*Node) (*Node, error) {
	base, err := a.Base.Node(attrs, parents)
	if err != nil {
		return nil, err
	}
	n := &Node{
		Kind:    NodeKindOverride,
		Pos:     a.Pos,
		Base:    base,
		Defs:    dummyDefs(attrs, a.Defs),
		Aliases: map[Attr]Attr{},
	}
	for k, v := range a.Aliases {
		n.Aliases[attrs.Get(k)] = attrs.Get(v)
	}
	if defs, err := instantiateDefs(attrs, parents, n, a.Defs); err != nil {
		return nil, err
	} else {
		n.Defs = defs
	}
	return n, nil
}

type ASTCall struct {
	Pos   Pos
	Base  ASTNode
	Defs  map[string]ASTNode
	Eager map[string]ASTNode
}

func (a *ASTCall) Node(attrs *AttrTable, parents []*Node) (*Node, error) {
	base, err := a.Base.Node(attrs, parents)
	if err != nil {
		return nil, err
	}
	n := &Node{
		Kind:  NodeKindOverride,
		Pos:   a.Pos,
		Base:  base,
		Defs:  dummyDefs(attrs, a.Defs),
		Eager: dummyDefs(attrs, a.Eager),
	}
	if defs, err := instantiateDefs(attrs, parents, n, a.Defs); err != nil {
		return nil, err
	} else {
		n.Defs = defs
	}
	if defs, err := instantiateDefs(attrs, parents, n, a.Eager); err != nil {
		return nil, err
	} else {
		n.Eager = defs
	}
	return n, nil
}

type ASTIntLit struct {
	Pos   Pos
	Value int64
}

func (a *ASTIntLit) Node(attrs *AttrTable, parents []*Node) (*Node, error) {
	return IntNode(attrs, a.Pos, a.Value), nil
}

type ASTStrLit struct {
	Pos   Pos
	Value string
}

func (a *ASTStrLit) Node(attrs *AttrTable, parents []*Node) (*Node, error) {
	return StrNode(attrs, a.Pos, a.Value), nil
}

type ASTTernary struct {
	Pos     Pos
	Cond    ASTNode
	IfTrue  ASTNode
	IfFalse ASTNode
}

func (a *ASTTernary) Node(attrs *AttrTable, parents []*Node) (*Node, error) {
	equiv := &ASTAccess{
		Pos: a.Pos,
		Base: &ASTCall{
			Pos: a.Pos,
			Base: &ASTAccess{
				Pos:  a.Pos,
				Base: a.Cond,
				Attr: "select",
			},
			Defs: map[string]ASTNode{
				"true":  a.IfTrue,
				"false": a.IfFalse,
			},
		},
		Attr: "result",
	}
	return equiv.Node(attrs, parents)
}

type ASTBinaryOp struct {
	Pos    Pos
	OpName string
	X      ASTNode
	Y      ASTNode
}

func (a *ASTBinaryOp) Node(attrs *AttrTable, parents []*Node) (*Node, error) {
	equiv := &ASTAccess{
		Pos: a.Pos,
		Base: &ASTCall{
			Pos: a.Pos,
			Base: &ASTAccess{
				Pos:  a.Pos,
				Base: a.X,
				Attr: a.OpName,
			},
			Defs: map[string]ASTNode{"y": a.Y},
		},
		Attr: "result",
	}
	return equiv.Node(attrs, parents)
}
