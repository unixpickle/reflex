package reflex

import (
	"fmt"
	"strings"
)

// An error raised by the interpreter with a backtrace of callsites that
// led to the execution error.
type InterpreterError struct {
	Inner error
	Trace GapStack
}

func (i *InterpreterError) Error() string {
	trace := ""
	for i, x := range i.Trace.Slice() {
		if len(trace) > 0 {
			trace += "\n"
		}
		for j := 0; j < i; j++ {
			trace += "  "
		}
		if x == (Pos{}) {
			trace += "... trace is truncated ..."
		} else {
			trace += x.String()
		}
	}
	return fmt.Sprintf("%s at\n%s", i.Inner, trace)
}

func formatAvailable(attrs *AttrTable, node Node) string {
	var strs []string
	if block, ok := node.(Block); ok {
		for attr := range block.Defs() {
			strs = append(strs, fmt.Sprintf("%#v", attrs.Name(attr)))
		}
	}
	return strings.Join(strs, ", ")
}

// Evaluate an expression until it becomes a literal or a block.
func Evaluate(ctx *Context, node Node, trace GapStack) (Node, error) {
	if node == nil {
		panic("nil node")
	}
	for {
		pos := node.Pos()
		newTrace := trace
		newTrace.Push(pos)

		nest := func(newNode Node) (Node, error) {
			res, err := Evaluate(ctx, newNode, newTrace)
			return res, err
		}
		doNext := func(newNode Node) {
			if newNode == nil {
				panic("nil node")
			}
			node = newNode
			trace = newTrace
		}

		switch node := node.(type) {
		case *NodeAccess:
			b := node.Base
			a := node.Attr
			node = nil
			base, err := nest(b)
			if err != nil {
				return nil, err
			}
			baseBlock, ok := base.(Block)
			if !ok {
				return nil, &InterpreterError{
					Inner: fmt.Errorf("unexpected kind for access base: %T", base),
					Trace: newTrace,
				}
			}
			obj, ok := baseBlock.Defs()[a]
			if !ok {
				return nil, &InterpreterError{
					Inner: fmt.Errorf(
						"unable to access attribute: %#v (available: %s)",
						ctx.Attrs.Name(a),
						formatAvailable(ctx.Attrs, base),
					),
					Trace: newTrace,
				}
			}
			doNext(obj)
		case *NodeOverride:
			base, err := nest(node.Base)
			if err != nil {
				return nil, err
			}
			baseBlock, ok := base.(Block)
			if !ok {
				return nil, &InterpreterError{
					Inner: fmt.Errorf("unexpected kind for override base: %T", base),
					Trace: newTrace,
				}
			}

			newBlock := &NodeBlock{
				NodeShared: NodeShared{
					P: pos,
				},
				DefMap: map[Attr]Node{},
				EdgeID: baseBlock.ScopeEdgeID(),
			}

			preClone := map[Attr]Node{}
			for k, v := range baseBlock.Defs() {
				preClone[k] = v
			}
			for dst, src := range node.Aliases {
				srcVal, ok := preClone[src]
				if !ok {
					return nil, &InterpreterError{
						Inner: fmt.Errorf(
							"could not create alias from %s to %s because source attribute does not exist",
							ctx.Attrs.Name(src),
							ctx.Attrs.Name(dst),
						),
						Trace: newTrace,
					}
				}
				preClone[dst] = srcVal
			}
			for k, v := range preClone {
				baseMap := map[BackEdgeID]Scope{baseBlock.ScopeEdgeID(): newBlock}
				if node.Defs[k] == nil && node.Eager[k] == nil {
					newBlock.DefMap[k] = NewNodeLazyClone(ctx.BackEdges, v, baseMap)
				}
			}

			overrideMap := map[BackEdgeID]Scope{node.EdgeID: newBlock}
			for k, v := range node.Defs {
				// Freeze back edges to the base block since they must refer to some other
				// object that is not being overridden.
				newBlock.DefMap[k] = CloneNode(
					ctx.BackEdges,
					v,
					overrideMap,
					ctx.BackEdges.MakeSet(baseBlock.ScopeEdgeID()),
				)
			}

			for k, v := range node.Eager {
				result, err := nest(v)
				if err != nil {
					return nil, err
				}
				block, ok := result.(Block)
				if !ok {
					return nil, &InterpreterError{
						Inner: fmt.Errorf(
							"eager assignment to %s produced a non-block type %T",
							ctx.Attrs.Name(k),
							result,
						),
						Trace: newTrace,
					}
				}
				newBlock.DefMap[k] = NewNodeFrozenBlock(ctx.BackEdges, block)
			}

			newBlock.RecomputeBackEdges(ctx.BackEdges)

			return newBlock, nil
		case *NodeBackEdge:
			return node.Node, nil
		case NodeFrozenBackEdge:
			return node.NodeBackEdge.Node, nil
		case *NodeLazyClone:
			doNext(node.Inner())
		case *NodeBuiltInOp:
			op := node.BuiltInOp
			for {
				result, nextExpr, err := op.Next(node.Node)
				if err != nil {
					return nil, &InterpreterError{Inner: err, Trace: trace}
				}
				if result != nil {
					doNext(result)
					break
				}
				nextResult, err := nest(nextExpr)
				if err != nil {
					return nil, err
				}
				op, err = op.Tell(node.Node, nextResult)
				if err != nil {
					return nil, &InterpreterError{Inner: err, Trace: newTrace}
				}
			}
		case *NodeIntLit, *NodeFloatLit, *NodeStrLit, *NodeBytesLit, *NodeBlock, NodeFrozenBlock:
			return node, nil
		default:
			panic("unknown node type")
		}
	}
}
