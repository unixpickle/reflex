package reflex

import "fmt"

// An error raised by the interpreter with a backtrace of callsites that
// led to the execution error.
type InterpreterError struct {
	Inner error
	Trace []Pos
}

func (i *InterpreterError) Error() string {
	trace := ""
	for i, x := range i.Trace {
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

func truncateTrace(trace []Pos) []Pos {
	if len(trace) > 20 {
		return append(append(append([]Pos{}, trace[:10]...), Pos{}), trace[len(trace)-10:]...)
	}
	return trace
}

func addTrace(trace []Pos, newPos Pos) []Pos {
	result := append(append([]Pos{}, trace...), newPos)
	return truncateTrace(result)
}

// Evaluate an expression until it becomes a literal or a block.
func Evaluate(attrs *AttrTable, node *Node, trace []Pos) (*Node, error) {
	nest := func(newNode *Node) (*Node, error) {
		return Evaluate(attrs, node, addTrace(trace, node.Pos))
	}

	switch node.Kind {
	case NodeKindAccess:
		obj, ok := node.Base.Defs.Get(node.Attr)
		if !ok {
			return nil, &InterpreterError{
				Inner: fmt.Errorf("unable to access attribute: %#v", attrs.Name(node.Attr)),
				Trace: trace,
			}
		}
		return nest(obj)
	case NodeKindOverride:
		base, err := nest(node.Base)
		if err != nil {
			return nil, err
		}
		newBase := base.Clone(nil)
		newBase.Defs = NewOverrideDefMap(newBase.Defs, node.Defs)
		return newBase, nil
	case NodeKindBackEdge:
		if node.Base.Kind != NodeKindBlock {
			panic("unexpected back edge type")
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
				return nest(result)
			}
			nextResult, err := nest(nextExpr)
			if err != nil {
				return nil, &InterpreterError{Inner: err, Trace: trace}
			}
			op, err = op.Tell(node.Base, nextResult)
			if err != nil {
				return nil, &InterpreterError{Inner: err, Trace: trace}
			}
		}
	case NodeKindIntLit, NodeKindStrLit, NodeKindBlock:
		return node, nil
	default:
		panic("unknown node type")
	}
}
