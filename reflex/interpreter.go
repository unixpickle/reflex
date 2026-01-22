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
func Evaluate(ctx *Context, node *Node, trace GapStack) (*Node, error) {
	for {
		pos := node.Pos
		newTrace := trace
		newTrace.Push(pos)

		nest := func(newNode *Node) (*Node, error) {
			return Evaluate(ctx, newNode, newTrace)
		}
		doNext := func(newNode *Node) {
			node = newNode
			trace = newTrace
		}

		switch node.Kind {
		case NodeKindAccess:
			b := node.Base
			a := node.Attr
			node = nil
			base, err := Evaluate(ctx, b, newTrace)
			if err != nil {
				return nil, err
			}
			if base.Kind != NodeKindBlock {
				return nil, &InterpreterError{Inner: fmt.Errorf("unexpected kind for base: %d", base.Kind), Trace: trace}
			}
			obj, ok := base.Defs.Get(a)
			if !ok {
				return nil, &InterpreterError{
					Inner: fmt.Errorf(
						"unable to access attribute: %#v (available: %s)",
						ctx.Attrs.Name(a),
						formatAvailable(ctx.Attrs, base),
					),
					Trace: trace,
				}
			}
			doNext(obj)
		case NodeKindOverride:
			base, err := nest(node.Base)
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
							Trace: trace,
						}
					}
				}
				newBase.Defs = NewOverrideDefMap(newBase.Defs, NewFlatDefMap(aliasMap))
			}

			if node.Eager != nil {
				newDefs := map[Attr]*Node{}

				// We will never actually use this mapping, since there are no back edges pointed
				// at nil, but this is crucial to reduce the size of ReplaceMaps.
				// If the result (newBase) is cloned again in the future, we will now create a
				// new mapping nil -> newNewBase.
				// If we iterate N times, we'll always have nil -> newNewNew...Base.
				// Without this, if the object is repeatedly cloned, then we end up with a mapping
				// like newBase->newNewBase, newNewBase -> newNewNewBase, etc, with all of these
				// redundant and unused mappings.
				var mapping *ReplaceMap[Node]
				mapping = mapping.Inserting(nil, newBase)

				for k, v := range node.Eager.Map(nil) {
					result, err := nest(v)
					if err != nil {
						return nil, err
					}
					newDefs[k] = result.Clone(mapping)
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
				nextResult, err := nest(nextExpr)
				if err != nil {
					return nil, err
				}
				op, err = op.Tell(node.Base, nextResult)
				if err != nil {
					return nil, &InterpreterError{Inner: err, Trace: trace}
				}
			}
		case NodeKindIntLit, NodeKindStrLit, NodeKindBytesLit, NodeKindBlock:
			return node, nil
		default:
			panic("unknown node type")
		}
	}
}
