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

func formatAvailable(attrs *AttrTable, node *Node) string {
	var strs []string
	for attr := range node.Defs.Map(nil) {
		strs = append(strs, fmt.Sprintf("%#v", attrs.Name(attr)))
	}
	return strings.Join(strs, ", ")
}

// Evaluate an expression until it becomes a literal or a block.
func Evaluate(ctx *Context, node *Node, trace GapStack, gc *GarbageCollector) (*Node, error) {
	if node == nil {
		panic("nil node")
	}
	if gc == nil {
		gc = NewGarbageCollector()
		defer gc.Shutdown()
	}
	for {
		gc.MaybeCollect()
		pos := node.Pos
		newTrace := trace
		newTrace.Push(pos)

		nest := func(newNode *Node, active ...*Node) (*Node, error) {
			for _, x := range active {
				gc.Retain(x)
			}
			gc.Retain(newNode)
			gc.MaybeCollect()
			res, err := Evaluate(ctx, newNode, newTrace, gc)
			gc.Retain(res)
			gc.MaybeCollect()
			gc.Release(res)
			gc.Release(newNode)
			for _, x := range active {
				gc.Release(x)
			}
			return res, err
		}
		doNext := func(newNode *Node) {
			if newNode == nil {
				panic("nil node")
			}
			node = newNode
			trace = newTrace
		}

		switch node.Kind {
		case NodeKindAccess:
			b := node.Base
			a := node.Attr
			node = nil
			base, err := Evaluate(ctx, b, newTrace, gc)
			if err != nil {
				return nil, err
			}
			if base.Kind != NodeKindBlock {
				return nil, &InterpreterError{
					Inner: fmt.Errorf("unexpected kind for base: %d", base.Kind),
					Trace: newTrace,
				}
			}
			obj, ok := base.Defs.Get(a)
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
		case NodeKindOverride:
			base, err := nest(node.Base, node)
			if err != nil {
				return nil, err
			}
			newBase := base.Clone(nil)
			newBase.Pos = node.Pos
			newBase.Defs = NewOverrideDefMap(newBase.Defs, NewCloneDefMapSingle(node.Defs, node, newBase))

			if len(node.Aliases) > 0 {
				aliasMap := map[Attr]*Node{}
				for dst, src := range node.Aliases {
					var ok bool
					aliasMap[dst], ok = newBase.Defs.Get(src)
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
				}
				newBase.Defs = NewOverrideDefMap(newBase.Defs, NewFlatDefMap(aliasMap))
			}

			if node.Eager != nil {
				newDefs := map[Attr]*Node{}
				for k, v := range node.Eager.Map(nil) {
					result, err := nest(v, node, newBase)
					if err != nil {
						return nil, err
					}
					resNode := &Node{
						Kind: NodeKindUnclonable,
						Pos:  result.Pos,
						Base: result,
					}
					newDefs[k] = resNode
					gc.Retain(resNode)
				}
				for _, v := range newDefs {
					gc.Release(v)
				}
				if len(newDefs) > 0 {
					newBase.Defs = NewOverrideDefMap(newBase.Defs, NewFlatDefMap(newDefs))
				}
			}
			newBase.Defs = MaybeFlatten(newBase.Defs)
			return newBase, nil
		case NodeKindBackEdge:
			if node.Base.Kind != NodeKindBlock {
				panic("unexpected back edge type: " + node.Base.Kind.String())
			}
			return node.Base, nil
		case NodeKindUnclonable:
			doNext(node.Base)
		case NodeKindBuiltInOp:
			op := node.BuiltInOp
			for {
				result, nextExpr, err := op.Next(node.Base)
				if err != nil {
					return nil, &InterpreterError{Inner: err, Trace: trace}
				}
				if result != nil {
					doNext(result)
					break
				}
				nextResult, err := nest(nextExpr, node)
				if err != nil {
					return nil, err
				}
				op, err = op.Tell(node.Base, nextResult)
				if err != nil {
					return nil, &InterpreterError{Inner: err, Trace: newTrace}
				}
			}
		case NodeKindIntLit, NodeKindFloatLit, NodeKindStrLit, NodeKindBytesLit, NodeKindBlock:
			return node, nil
		default:
			panic("unknown node type")
		}
	}
}
