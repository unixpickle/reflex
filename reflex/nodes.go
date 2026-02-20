package reflex

type Node interface {
	Clone(*ReplaceMap) Node
	Flatten()
	Pos() Pos
}

type Block interface {
	Node
	Defines(Attr) bool
}

type NodeBase struct {
	P          Pos
	DidFlatten bool
}

func (n *NodeBase) Pos() Pos {
	return n.P
}

type NodeAccess struct {
	NodeBase
	Base Node
	Attr Attr
}

func (n *NodeAccess) Clone(r *ReplaceMap) Node {
	return &NodeAccess{
		NodeBase: n.NodeBase,
		Base:     n.Base.Clone(r),
		Attr:     n.Attr,
	}
}

func (n *NodeAccess) Flatten() {
	if !n.DidFlatten {
		n.DidFlatten = true
		n.Base.Flatten()
	}
}

type NodeBlock struct {
	NodeBase
	Defs DefMap
}

func (n *NodeBlock) Clone(r *ReplaceMap) Node {
	newNode := &NodeBlock{NodeBase: n.NodeBase}
	newMap := r.Inserting(n, newNode)
	newNode.Defs = MaybeFlatten(NewCloneDefMap(n.Defs, newMap))
	return newNode
}

func (n *NodeBlock) Defines(attr Attr) bool {
	_, ok := n.Defs.Get(attr)
	return ok
}

func (n *NodeBlock) Flatten() {
	if !n.DidFlatten {
		n.DidFlatten = true
		newMap := n.Defs.Map(nil)
		for _, newN := range newMap {
			newN.Flatten()
		}
		n.Defs = NewFlatDefMap(newMap)
	}
}

type NodeOverride struct {
	NodeBase
	Base    Node
	Defs    DefMap
	Eager   DefMap
	Aliases map[Attr]Attr
}

func (n *NodeOverride) Clone(r *ReplaceMap) Node {
	newNode := &NodeOverride{
		NodeBase: n.NodeBase,
		Base:     n.Base.Clone(r),
		Aliases:  n.Aliases,
	}
	newMap := r.Inserting(n, newNode)
	newNode.Defs = MaybeFlatten(NewCloneDefMap(n.Defs, newMap))
	if n.Eager != nil {
		newNode.Eager = MaybeFlatten(NewCloneDefMap(n.Eager, r))
	}
	return newNode
}

func (n *NodeOverride) Defines(attr Attr) bool {
	if _, ok := n.Defs.Get(attr); ok {
		return true
	}
	if _, ok := n.Eager.Get(attr); ok {
		return true
	}
	if _, ok := n.Aliases[attr]; ok {
		return true
	}
	return false
}

func (n *NodeOverride) Flatten() {
	if !n.DidFlatten {
		n.DidFlatten = true
		for _, mPtr := range []*DefMap{&n.Defs, &n.Eager} {
			if *mPtr == nil {
				continue
			}
			newMap := (*mPtr).Map(nil)
			for _, newN := range newMap {
				newN.Flatten()
			}
			*mPtr = NewFlatDefMap(newMap)
		}
		n.Base.Flatten()
	}
}

type NodeBackEdge struct {
	NodeBase
	Ref Node
}

func (n *NodeBackEdge) Clone(r *ReplaceMap) Node {
	if repl, ok := r.Get(n.Ref); ok {
		return &NodeBackEdge{NodeBase: n.NodeBase, Ref: repl}
	} else {
		return n
	}
}

func (n *NodeBackEdge) Flatten() {
	if !n.DidFlatten {
		n.DidFlatten = true
		n.Ref.Flatten()
	}
}

type LitBase struct {
	NodeBase
}

func (n *LitBase) Flatten() {
	n.DidFlatten = true
}

type NodeIntLit struct {
	LitBase
	Lit int64
}

func (n *NodeIntLit) Clone(r *ReplaceMap) Node {
	return n
}

type NodeFloatLit struct {
	LitBase
	Lit float64
}

func (n *NodeFloatLit) Clone(r *ReplaceMap) Node {
	return n
}

type NodeStrLit struct {
	LitBase
	Lit string
}

func (n *NodeStrLit) Clone(r *ReplaceMap) Node {
	return n
}

type NodeBytesLit struct {
	LitBase
	Lit []byte
}

func (n *NodeBytesLit) Clone(r *ReplaceMap) Node {
	return n
}

type NodeBuiltInOp struct {
	NodeBase
	Context Node
	Op      BuiltInOp
}

func (n *NodeBuiltInOp) Clone(r *ReplaceMap) Node {
	if repl, ok := r.Get(n.Context); ok {
		return &NodeBuiltInOp{NodeBase: n.NodeBase, Context: repl, Op: n.Op}
	} else {
		return n
	}
}

func (n *NodeBuiltInOp) Flatten() {
	if !n.DidFlatten {
		n.DidFlatten = true
		n.Context.Flatten()
	}
}

type NodeUnclonable struct {
	NodeBase
	Wrapped Node
}

func (n *NodeUnclonable) Clone(r *ReplaceMap) Node {
	return n
}

func (n *NodeUnclonable) Flatten() {
	if !n.DidFlatten {
		n.DidFlatten = true
		n.Wrapped.Flatten()
	}
}
