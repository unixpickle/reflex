package reflex

import (
	"github.com/unixpickle/essentials"
)

type Node interface {
	Pos() Pos
	BackEdges() *BackEdgeSet

	// Clone creates a new node, and requires that the back edges in the
	// node actually overlap with the backEdge map that is passed.
	Clone(b *BackEdges, overrides map[BackEdgeID]Scope) Node
}

type Scope interface {
	Node

	Defines(Attr) bool
	ScopeEdgeID() BackEdgeID
}

type Block interface {
	Scope
	Defs() map[Attr]Node
}

type NodeShared struct {
	P Pos
	B *BackEdgeSet
}

func (n NodeShared) Pos() Pos {
	return n.P
}

func (n NodeShared) BackEdges() *BackEdgeSet {
	return n.B
}

func (n NodeShared) Clone(b *BackEdges, overrides map[BackEdgeID]Scope) Node {
	panic("this node should never need to be cloned")
}

type NodeAccess struct {
	NodeShared
	Base Node
	Attr Attr
}

func NewNodeAccess(pos Pos, base Node, attr Attr) *NodeAccess {
	return &NodeAccess{
		NodeShared: NodeShared{
			P: pos,
			B: base.BackEdges(),
		},
		Base: base,
		Attr: attr,
	}
}

func (n *NodeAccess) Clone(b *BackEdges, overrides map[BackEdgeID]Scope) Node {
	base := CloneNode(b, n.Base, overrides)
	return &NodeAccess{
		NodeShared: NodeShared{
			P: n.P,
			B: base.BackEdges(),
		},
		Base: base,
		Attr: n.Attr,
	}
}

func sortedDefs(defs map[Attr]Node) []Node {
	attrs := make([]Attr, 0, len(defs))
	nodes := make([]Node, 0, len(defs))
	for k, v := range defs {
		attrs = append(attrs, k)
		nodes = append(nodes, v)
	}
	essentials.VoodooSort(attrs, func(i, j int) bool {
		return int(attrs[i]) < int(attrs[j])
	}, nodes)
	return nodes
}

type NodeBlock struct {
	NodeShared
	DefMap map[Attr]Node

	EdgeID BackEdgeID
}

func NewNodeBlock(
	b *BackEdges,
	pos Pos,
	defAttrs []Attr,
	defFn func(s Scope) map[Attr]Node,
) *NodeBlock {
	res := &NodeBlock{
		NodeShared: NodeShared{
			P: pos,
		},
		DefMap: dummyDefs(defAttrs),
		EdgeID: b.MakeEdgeID(),
	}

	res.DefMap = defFn(res)
	res.RecomputeBackEdges(b)
	return res
}

func (n *NodeBlock) Clone(b *BackEdges, overrides map[BackEdgeID]Scope) Node {
	res := &NodeBlock{
		NodeShared: n.NodeShared,
		DefMap:     map[Attr]Node{},
		EdgeID:     n.EdgeID,
	}

	newBackEdges := map[BackEdgeID]Scope{}
	for k, v := range overrides {
		newBackEdges[k] = v
	}
	newBackEdges[n.EdgeID] = res

	for k, v := range n.DefMap {
		res.DefMap[k] = CloneNode(b, v, newBackEdges)
	}
	res.RecomputeBackEdges(b)

	return res
}

func (n *NodeBlock) Defines(attr Attr) bool {
	_, ok := n.DefMap[attr]
	return ok
}

func (n *NodeBlock) ScopeEdgeID() BackEdgeID {
	return n.EdgeID
}

func (n *NodeBlock) Override(b *BackEdges, pos Pos, defs map[Attr]Node) *NodeBlock {
	res := &NodeBlock{
		NodeShared: NodeShared{
			P: pos,
		},
		DefMap: map[Attr]Node{},
		EdgeID: n.EdgeID,
	}
	edgeOverride := map[BackEdgeID]Scope{n.EdgeID: res}
	for k, v := range defs {
		res.DefMap[k] = CloneNode(b, v, edgeOverride)
	}
	for k, v := range n.DefMap {
		if res.DefMap[k] == nil {
			res.DefMap[k] = CloneNode(b, v, edgeOverride)
		}
	}

	res.RecomputeBackEdges(b)

	return res
}

func (n *NodeBlock) RecomputeBackEdges(b *BackEdges) {
	n.B = b.MakeSet()
	for _, x := range sortedDefs(n.DefMap) {
		n.B = b.Merge(n.B, x.BackEdges())
	}
}

func (n *NodeBlock) Defs() map[Attr]Node {
	return n.DefMap
}

type NodeFrozenBlock struct {
	Block
	Empty *BackEdgeSet
}

func NewNodeFrozenBlock(b *BackEdges, n Block) NodeFrozenBlock {
	if x, ok := n.(NodeFrozenBlock); ok {
		return x
	}
	return NodeFrozenBlock{Block: n, Empty: b.MakeSet()}
}

func (n NodeFrozenBlock) BackEdges() *BackEdgeSet {
	return n.Empty
}

func (n NodeFrozenBlock) Clone(b *BackEdges, overrides map[BackEdgeID]Scope) Node {
	panic("cannot clone NodeFrozenBlock")
}

type NodeOverride struct {
	NodeShared
	Base    Node
	Defs    map[Attr]Node
	Eager   map[Attr]Node
	Aliases map[Attr]Attr

	EdgeID BackEdgeID
}

func NewNodeOverride(
	b *BackEdges,
	pos Pos,
	base Node,
	defAttrs []Attr,
	defFn func(Scope) map[Attr]Node,
	eager map[Attr]Node,
	aliases map[Attr]Attr,
) *NodeOverride {
	res := &NodeOverride{
		NodeShared: NodeShared{
			P: pos,
			B: base.BackEdges(),
		},
		Base:    base,
		Defs:    dummyDefs(defAttrs),
		Eager:   eager,
		Aliases: aliases,
		EdgeID:  b.MakeEdgeID(),
	}

	res.Defs = defFn(res)

	for _, d := range []map[Attr]Node{res.Defs, res.Eager} {
		for _, x := range sortedDefs(d) {
			res.B = b.Merge(res.B, x.BackEdges())
		}
	}
	return res
}

func (n *NodeOverride) Clone(b *BackEdges, overrides map[BackEdgeID]Scope) Node {
	base := CloneNode(b, n.Base, overrides)

	res := &NodeOverride{
		NodeShared: NodeShared{
			P: n.P,
			B: base.BackEdges(),
		},
		Base:    base,
		Aliases: n.Aliases,
		Defs:    map[Attr]Node{},
		Eager:   map[Attr]Node{},
		EdgeID:  n.EdgeID,
	}

	newBackEdges := map[BackEdgeID]Scope{}
	for k, v := range overrides {
		newBackEdges[k] = v
	}
	newBackEdges[n.EdgeID] = res

	for k, v := range n.Defs {
		res.Defs[k] = CloneNode(b, v, newBackEdges)
	}
	for k, v := range n.Eager {
		res.Eager[k] = CloneNode(b, v, newBackEdges)
	}

	for _, d := range []map[Attr]Node{res.Defs, res.Eager} {
		for _, x := range sortedDefs(d) {
			res.B = b.Merge(res.B, x.BackEdges())
		}
	}

	return res
}

func (n *NodeOverride) Defines(a Attr) bool {
	_, ok1 := n.Defs[a]
	_, ok2 := n.Eager[a]
	_, ok3 := n.Aliases[a]
	return ok1 || ok2 || ok3
}

func (n *NodeOverride) ScopeEdgeID() BackEdgeID {
	return n.EdgeID
}

type NodeBackEdge struct {
	NodeShared

	EdgeID BackEdgeID
	Node   Scope
}

func NewNodeBackEdge(b *BackEdges, pos Pos, n Scope) *NodeBackEdge {
	return &NodeBackEdge{
		NodeShared: NodeShared{
			P: pos,
			B: b.MakeSet(n.ScopeEdgeID()),
		},
		EdgeID: n.ScopeEdgeID(),
		Node:   n,
	}
}

func (n *NodeBackEdge) Clone(b *BackEdges, overrides map[BackEdgeID]Scope) Node {
	node := overrides[n.EdgeID]
	if node == nil {
		panic("cloned NodeBackEdge when backEdges doesn't contain our back edge ID")
	}
	return &NodeBackEdge{
		NodeShared: NodeShared{
			P: n.P,
			B: b.MakeSet(node.ScopeEdgeID()),
		},
		EdgeID: node.ScopeEdgeID(),
		Node:   node,
	}
}

type NodeBuiltInOp struct {
	NodeBackEdge

	BuiltInOp BuiltInOp
}

func NewNodeBuiltInOp(b *BackEdges, pos Pos, n Scope, op BuiltInOp) *NodeBuiltInOp {
	return &NodeBuiltInOp{
		NodeBackEdge: NodeBackEdge{
			NodeShared: NodeShared{
				P: pos,
				B: b.MakeSet(n.ScopeEdgeID()),
			},
			EdgeID: n.ScopeEdgeID(),
			Node:   n,
		},
		BuiltInOp: op,
	}
}

func (n *NodeBuiltInOp) Clone(b *BackEdges, overrides map[BackEdgeID]Scope) Node {
	node := overrides[n.EdgeID]
	if node == nil {
		panic("cloned NodeBuiltInOp when backEdges doesn't contain our back edge ID")
	}
	return &NodeBuiltInOp{
		NodeBackEdge: NodeBackEdge{
			NodeShared: NodeShared{
				P: n.P,
				B: b.MakeSet(node.ScopeEdgeID()),
			},
			EdgeID: n.EdgeID,
			Node:   node,
		},
		BuiltInOp: n.BuiltInOp,
	}
}

type NodeIntLit struct {
	NodeShared

	Lit int64
}

func NewNodeIntLit(b *BackEdges, pos Pos, lit int64) *NodeIntLit {
	return &NodeIntLit{
		NodeShared: NodeShared{
			P: pos,
			B: b.MakeSet(),
		},
		Lit: lit,
	}
}

type NodeFloatLit struct {
	NodeShared

	Lit float64
}

func NewNodeFloatLit(b *BackEdges, pos Pos, lit float64) *NodeFloatLit {
	return &NodeFloatLit{
		NodeShared: NodeShared{
			P: pos,
			B: b.MakeSet(),
		},
		Lit: lit,
	}
}

type NodeStrLit struct {
	NodeShared

	Lit string
}

func NewNodeStrLit(b *BackEdges, pos Pos, lit string) *NodeStrLit {
	return &NodeStrLit{
		NodeShared: NodeShared{
			P: pos,
			B: b.MakeSet(),
		},
		Lit: lit,
	}
}

type NodeBytesLit struct {
	NodeShared

	Lit []byte
}

func NewNodeBytesLit(b *BackEdges, pos Pos, lit []byte) *NodeBytesLit {
	return &NodeBytesLit{
		NodeShared: NodeShared{
			P: pos,
			B: b.MakeSet(),
		},
		Lit: lit,
	}
}

// CloneNode creates a copy of the node if any back edges need to be
// replaced, or returns the node unchanged otherwise.
func CloneNode(b *BackEdges, n Node, overrides map[BackEdgeID]Scope) Node {
	be := n.BackEdges()
	newBackEdges := map[BackEdgeID]Scope{}
	for k, v := range overrides {
		if be.Contains(k) {
			newBackEdges[k] = v
		}
	}
	if len(newBackEdges) == 0 {
		return n
	}
	return n.Clone(b, newBackEdges)
}

func dummyDefs(attrs []Attr) map[Attr]Node {
	res := map[Attr]Node{}
	for _, x := range attrs {
		res[x] = nil
	}
	return res
}
