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
	Node(ctx *Context, parents []Block) (Node, error)
}

type ASTParent struct {
	Pos   Pos
	Depth int
}

func (a *ASTParent) Node(ctx *Context, parents []Block) (Node, error) {
	if a.Depth+1 > len(parents) {
		return nil, &ASTError{Msg: "parent access goes beyond top scope", Pos: a.Pos}
	}
	return &NodeBackEdge{
		NodeBase: NodeBase{P: a.Pos},
		Ref:      parents[len(parents)-(a.Depth+1)],
	}, nil
}

type ASTSelfRef struct {
	Pos Pos
}

func (a *ASTSelfRef) Node(ctx *Context, parents []Block) (Node, error) {
	return &NodeBackEdge{
		NodeBase: NodeBase{P: a.Pos},
		Ref:      parents[len(parents)-1],
	}, nil
}

type ASTAccess struct {
	Pos  Pos
	Base ASTNode
	Attr string
}

func (a *ASTAccess) Node(ctx *Context, parents []Block) (Node, error) {
	baseNode, err := a.Base.Node(ctx, parents)
	if err != nil {
		return nil, err
	}
	return &NodeAccess{
		NodeBase: NodeBase{P: a.Pos},
		Base:     baseNode,
		Attr:     ctx.Attrs.Get(a.Attr),
	}, nil
}

type ASTIdentifier struct {
	Pos  Pos
	Name string
}

func (a *ASTIdentifier) Node(ctx *Context, parents []Block) (Node, error) {
	return (&ASTAccess{Pos: a.Pos, Base: &ASTSelfRef{Pos: a.Pos}, Attr: a.Name}).Node(ctx, parents)
}

type ASTAncestorLookup struct {
	Pos  Pos
	Attr string
}

func (a *ASTAncestorLookup) Node(ctx *Context, parents []Block) (Node, error) {
	attr := ctx.Attrs.Get(a.Attr)
	for i := len(parents) - 2; i >= 0; i-- {
		parent := parents[i]
		if parent.Defines(attr) {
			return &NodeAccess{
				NodeBase: NodeBase{P: a.Pos},
				Base: &NodeBackEdge{
					NodeBase: NodeBase{P: a.Pos},
					Ref:      parent,
				},
				Attr: ctx.Attrs.Get(a.Attr),
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

func (a *ASTBlock) Node(ctx *Context, parents []Block) (Node, error) {
	n := &NodeBlock{
		NodeBase: NodeBase{P: a.Pos},
		Defs:     dummyDefs(ctx, a.Defs),
	}
	if defs, err := instantiateDefs(ctx, append(parents, n), n, a.Defs); err != nil {
		return nil, err
	} else {
		n.Defs = defs
	}
	return n, nil
}

func dummyDefs[V any](ctx *Context, d map[string]V) DefMap {
	m := map[Attr]Node{}
	for k := range d {
		m[ctx.Attrs.Get(k)] = nil
	}
	return NewFlatDefMap(m)
}

func instantiateDefs(ctx *Context, parents []Block, n Node, defs map[string]ASTNode) (DefMap, error) {
	newDefs := map[Attr]Node{}
	for k, v := range defs {
		newNode, err := v.Node(ctx, parents)
		if err != nil {
			return nil, err
		}
		newDefs[ctx.Attrs.Get(k)] = newNode
	}
	return NewFlatDefMap(newDefs), nil
}

type ASTOverride struct {
	Pos     Pos
	Base    ASTNode
	Defs    map[string]ASTNode
	Aliases map[string]string
}

func (a *ASTOverride) Node(ctx *Context, parents []Block) (Node, error) {
	base, err := a.Base.Node(ctx, parents)
	if err != nil {
		return nil, err
	}
	n := &NodeOverride{
		NodeBase: NodeBase{P: a.Pos},
		Base:     base,
		Defs:     dummyDefs(ctx, a.Defs),
		Aliases:  map[Attr]Attr{},
	}
	for k, v := range a.Aliases {
		n.Aliases[ctx.Attrs.Get(k)] = ctx.Attrs.Get(v)
	}
	if defs, err := instantiateDefs(ctx, append(parents, n), n, a.Defs); err != nil {
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

func (a *ASTCall) Node(ctx *Context, parents []Block) (Node, error) {
	base, err := a.Base.Node(ctx, parents)
	if err != nil {
		return nil, err
	}
	n := &NodeOverride{
		NodeBase: NodeBase{P: a.Pos},
		Base:     base,
		Defs:     dummyDefs(ctx, a.Defs),
		Eager:    dummyDefs(ctx, a.Eager),
	}
	if defs, err := instantiateDefs(ctx, parents, n, a.Defs); err != nil {
		return nil, err
	} else {
		n.Defs = defs
	}
	if defs, err := instantiateDefs(ctx, parents, n, a.Eager); err != nil {
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

func (a *ASTIntLit) Node(ctx *Context, parents []Block) (Node, error) {
	return ctx.IntNode(a.Pos, a.Value), nil
}

type ASTFloatLit struct {
	Pos   Pos
	Value float64
}

func (a *ASTFloatLit) Node(ctx *Context, parents []Block) (Node, error) {
	return ctx.FloatNode(a.Pos, a.Value), nil
}

type ASTStrLit struct {
	Pos   Pos
	Value string
}

func (a *ASTStrLit) Node(ctx *Context, parents []Block) (Node, error) {
	return ctx.StrNode(a.Pos, a.Value), nil
}

type ASTTernary struct {
	Pos     Pos
	Cond    ASTNode
	IfTrue  ASTNode
	IfFalse ASTNode
}

func (a *ASTTernary) Node(ctx *Context, parents []Block) (Node, error) {
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
	return equiv.Node(ctx, parents)
}

type ASTBinaryOp struct {
	Pos    Pos
	OpName string
	X      ASTNode
	Y      ASTNode
}

func (a *ASTBinaryOp) Node(ctx *Context, parents []Block) (Node, error) {
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
	return equiv.Node(ctx, parents)
}
