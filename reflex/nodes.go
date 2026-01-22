package reflex

type NodeKind int

const (
	NodeKindAccess NodeKind = iota
	NodeKindBlock
	NodeKindOverride
	NodeKindBackEdge
	NodeKindIntLit
	NodeKindStrLit
	NodeKindBytesLit
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
	case NodeKindBytesLit:
		return "NodeKindBytesLit"
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
	StrLit   string
	BytesLit []byte
	IntLit   int64
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
			newNode.Eager = MaybeFlatten(NewCloneDefMap(n.Eager, r))
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
	case NodeKindBytesLit:
		newNode.BytesLit = n.BytesLit
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
