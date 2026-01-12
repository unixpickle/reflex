package reflex

import (
	"fmt"
	"strconv"
	"strings"
)

type BuiltInOpError struct {
	Msg string
	Pos Pos
}

func (b *BuiltInOpError) Error() string {
	return fmt.Sprintf("%s at %s", b.Msg, b.Pos)
}

// IntNode creates a node with all of the built-in int methods.
func IntNode(attrs *AttrTable, pos Pos, lit int64) *Node {
	result := &Node{
		Kind: NodeKindBlock,
		Pos:  pos,
	}

	makeBinaryOp := func(fn func(int64, int64) int64) *Node {
		result := &Node{
			Kind: NodeKindBlock,
			Pos:  pos,
		}
		result.Defs = NewFlatDefMap(map[Attr]*Node{
			attrs.Get("x"): &Node{
				Kind: NodeKindBackEdge,
				Pos:  pos,
				Base: result,
			},
			attrs.Get("result"): &Node{
				Kind: NodeKindBuiltInOp,
				Pos:  pos,
				Base: result,
				BuiltInOp: newFnBuiltInOp(attrs, []string{"x._inner", "y._inner"}, func(args map[string]*Node) (*Node, error) {
					x := args["x._inner"]
					y := args["y._inner"]
					if x.Kind != NodeKindIntLit {
						return nil, &BuiltInOpError{Msg: "x argument is not an int value", Pos: x.Pos}
					}
					if y.Kind != NodeKindIntLit {
						return nil, &BuiltInOpError{Msg: "y argument is not an int value", Pos: y.Pos}
					}
					return IntNode(attrs, pos, fn(x.IntLit, y.IntLit)), nil
				}),
			},
		})
		return result
	}

	makeUnaryOp := func(fn func(int64) any) *Node {
		result = &Node{
			Kind: NodeKindBuiltInOp,
			Pos:  pos,
			Base: result,
			BuiltInOp: newFnBuiltInOp(attrs, []string{"x._inner"}, func(args map[string]*Node) (*Node, error) {
				x := args["x._inner"]
				if x.Kind != NodeKindIntLit {
					return nil, &BuiltInOpError{Msg: "x argument is not an int value", Pos: x.Pos}
				}
				result := fn(x.IntLit)
				if str, ok := result.(string); ok {
					return StrNode(attrs, pos, str), nil
				}
				return IntNode(attrs, pos, result.(int64)), nil
			}),
		}
		return result
	}

	makeSelectOrLogic := func(selfName string, op BuiltInOp) *Node {
		result := &Node{
			Kind: NodeKindBlock,
			Pos:  pos,
		}
		result.Defs = NewFlatDefMap(map[Attr]*Node{
			attrs.Get(selfName): &Node{
				Kind: NodeKindBackEdge,
				Pos:  pos,
				Base: result,
			},
			attrs.Get("result"): &Node{
				Kind:      NodeKindBuiltInOp,
				Pos:       pos,
				Base:      result,
				BuiltInOp: op,
			},
		})
		return result
	}

	result.Defs = NewFlatDefMap(map[Attr]*Node{
		attrs.Get("_inner"): &Node{
			Kind:   NodeKindIntLit,
			Pos:    pos,
			IntLit: lit,
		},
		attrs.Get("add"): makeBinaryOp(func(x, y int64) int64 {
			return x + y
		}),
		attrs.Get("sub"): makeBinaryOp(func(x, y int64) int64 {
			return x - y
		}),
		attrs.Get("div"): makeBinaryOp(func(x, y int64) int64 {
			return x / y
		}),
		attrs.Get("mul"): makeBinaryOp(func(x, y int64) int64 {
			return x * y
		}),
		attrs.Get("mod"): makeBinaryOp(func(x, y int64) int64 {
			r := x % y
			if r < 0 {
				if y < 0 {
					r -= y
				} else {
					r += y
				}
			}
			return r
		}),
		attrs.Get("lt"): makeBinaryOp(func(x, y int64) int64 {
			if x < y {
				return 1
			} else {
				return 0
			}
		}),
		attrs.Get("gt"): makeBinaryOp(func(x, y int64) int64 {
			if x > y {
				return 1
			} else {
				return 0
			}
		}),
		attrs.Get("le"): makeBinaryOp(func(x, y int64) int64 {
			if x <= y {
				return 1
			} else {
				return 0
			}
		}),
		attrs.Get("ge"): makeBinaryOp(func(x, y int64) int64 {
			if x >= y {
				return 1
			} else {
				return 0
			}
		}),
		attrs.Get("eq"): makeBinaryOp(func(x, y int64) int64 {
			if x == y {
				return 1
			} else {
				return 0
			}
		}),
		attrs.Get("ne"): makeBinaryOp(func(x, y int64) int64 {
			if x != y {
				return 1
			} else {
				return 0
			}
		}),
		attrs.Get("chr"): makeUnaryOp(func(x int64) any {
			return string(rune(x))
		}),
		attrs.Get("str"): makeUnaryOp(func(x int64) any {
			return strconv.FormatInt(x, 10)
		}),
		attrs.Get("select"):      makeSelectOrLogic("cond", newBuiltInSelect(attrs)),
		attrs.Get("logical_and"): makeSelectOrLogic("x", newBuiltInLogic(attrs, true)),
		attrs.Get("logical_or"):  makeSelectOrLogic("x", newBuiltInLogic(attrs, false)),
	})
	return result
}

// StrNode creates a node with all of the built-in string methods.
func StrNode(attrs *AttrTable, pos Pos, lit string) *Node {
	result := &Node{
		Kind: NodeKindBlock,
		Pos:  pos,
	}

	makeBinaryOp := func(fn func(string, string) any) *Node {
		result := &Node{
			Kind: NodeKindBlock,
			Pos:  pos,
		}
		result.Defs = NewFlatDefMap(map[Attr]*Node{
			attrs.Get("x"): &Node{
				Kind: NodeKindBackEdge,
				Pos:  pos,
				Base: result,
			},
			attrs.Get("result"): &Node{
				Kind: NodeKindBuiltInOp,
				Pos:  pos,
				Base: result,
				BuiltInOp: newFnBuiltInOp(attrs, []string{"x._inner", "y._inner"}, func(args map[string]*Node) (*Node, error) {
					x := args["x._inner"]
					y := args["y._inner"]
					if x.Kind != NodeKindStrLit {
						return nil, &BuiltInOpError{Msg: "x argument is not a str value", Pos: x.Pos}
					}
					if y.Kind != NodeKindStrLit {
						return nil, &BuiltInOpError{Msg: "y argument is not a str value", Pos: y.Pos}
					}
					result := fn(x.StrLit, y.StrLit)
					if str, ok := result.(string); ok {
						return StrNode(attrs, pos, str), nil
					}
					return IntNode(attrs, pos, result.(int64)), nil
				}),
			},
		})
		return result
	}

	substr := &Node{
		Kind: NodeKindBlock,
		Pos:  pos,
	}
	substr.Defs = NewFlatDefMap(map[Attr]*Node{
		attrs.Get("x"): &Node{
			Kind: NodeKindBackEdge,
			Pos:  pos,
			Base: result,
		},
		attrs.Get("start"): IntNode(attrs, pos, 0),
		attrs.Get("end"): &Node{
			Kind: NodeKindAccess,
			Pos:  pos,
			Base: &Node{
				Kind: NodeKindAccess,
				Pos:  pos,
				Base: &Node{
					Kind: NodeKindBackEdge,
					Pos:  pos,
					Base: result,
				},
				Attr: attrs.Get("x"),
			},
			Attr: attrs.Get("len"),
		},
		attrs.Get("result"): &Node{
			Kind: NodeKindBuiltInOp,
			Pos:  pos,
			Base: substr,
			BuiltInOp: newFnBuiltInOp(attrs, []string{"x._inner", "start._inner", "end._inner"}, func(args map[string]*Node) (*Node, error) {
				x := args["x._inner"]
				startNode := args["start._inner"]
				endNode := args["end._inner"]
				if x.Kind != NodeKindStrLit {
					return nil, &BuiltInOpError{Msg: "x argument is not a str value", Pos: x.Pos}
				}
				if startNode.Kind != NodeKindIntLit {
					return nil, &BuiltInOpError{Msg: "start argument is not an int value", Pos: startNode.Pos}
				}
				if endNode.Kind != NodeKindIntLit {
					return nil, &BuiltInOpError{Msg: "end argument is not an int value", Pos: endNode.Pos}
				}
				start := int(startNode.IntLit)
				end := int(endNode.IntLit)
				str := x.StrLit
				if start < 0 {
					start += len(str)
				}
				if end < 0 {
					end += len(str)
				}
				if end > len(str) {
					end = len(str)
				} else if end < 0 {
					end = 0
				}
				if start > len(str) {
					start = len(str)
				} else if start < 0 {
					start = 0
				}
				result := ""
				if start < end {
					result = str[start:end]
				}
				return StrNode(attrs, pos, result), nil
			}),
		},
	})

	result.Defs = NewFlatDefMap(map[Attr]*Node{
		attrs.Get("_inner"): &Node{
			Kind:   NodeKindStrLit,
			Pos:    pos,
			StrLit: lit,
		},
		attrs.Get("add"): makeBinaryOp(func(x, y string) any {
			return x + y
		}),
		attrs.Get("eq"): makeBinaryOp(func(x, y string) any {
			return x == y
		}),
		attrs.Get("ne"): makeBinaryOp(func(x, y string) any {
			return x != y
		}),
		attrs.Get("len"): &Node{
			Kind: NodeKindBuiltInOp,
			Pos:  pos,
			Base: result,
			BuiltInOp: newFnBuiltInOp(attrs, []string{"_inner"}, func(args map[string]*Node) (*Node, error) {
				inner := args["_inner"]
				if inner.Kind != NodeKindStrLit {
					return nil, &BuiltInOpError{Msg: "inner argument is not a str value", Pos: inner.Pos}
				}
				return IntNode(attrs, pos, int64(len(inner.StrLit))), nil
			}),
		},
		attrs.Get("substr"): substr,
	})
	return result
}

// A BuiltInOp uses a context and optionally evaluates expressions in the context
// to perform an operation.
type BuiltInOp interface {
	// Next either tells the interpreter the final result, or indicates that
	// another expression must be evaluated first.
	Next(context *Node) (result *Node, nextExpr *Node, err error)

	// Tell gives the operation the next evaluated expression, and gets a new BuiltInOp
	// which can perform the next step.
	Tell(context *Node, result *Node) (BuiltInOp, error)
}

type fnBuiltInOp struct {
	Attrs *AttrTable
	Paths []string
	Fn    func(args map[string]*Node) (*Node, error)
	Found map[string]*Node
}

func newFnBuiltInOp(attrs *AttrTable, paths []string, fn func(args map[string]*Node) (*Node, error)) *fnBuiltInOp {
	return &fnBuiltInOp{
		Attrs: attrs,
		Paths: paths,
		Fn:    fn,
		Found: map[string]*Node{},
	}
}

func (f *fnBuiltInOp) Next(context *Node) (result *Node, nextExpr *Node, err error) {
	for _, p := range f.Paths {
		if _, ok := f.Found[p]; !ok {
			// Create an access chain and return it.
			accessChain := context
			for _, part := range strings.Split(p, ".") {
				accessChain = &Node{
					Kind: NodeKindAccess,
					Pos:  context.Pos,
					Base: accessChain,
					Attr: f.Attrs.Get(part),
				}
			}
			return nil, accessChain, nil
		}
	}
	out, err := f.Fn(f.Found)
	return out, nil, err
}

func (f *fnBuiltInOp) Tell(context, result *Node) (BuiltInOp, error) {
	newResult := map[string]*Node{}
	for _, p := range f.Paths {
		if v, ok := f.Found[p]; ok {
			newResult[p] = v
		} else {
			newResult[p] = result
			break
		}
	}
	return &fnBuiltInOp{Attrs: f.Attrs, Paths: f.Paths, Fn: f.Fn, Found: newResult}, nil
}

type builtInSelect struct {
	Attrs    *AttrTable
	SeenCond bool
	NextAttr string
}

func newBuiltInSelect(attrs *AttrTable) *builtInSelect {
	return &builtInSelect{Attrs: attrs}
}

func (b *builtInSelect) Next(context *Node) (result *Node, nextExpr *Node, err error) {
	if b.SeenCond {
		return &Node{
			Kind: NodeKindAccess,
			Pos:  context.Pos,
			Base: context,
			Attr: b.Attrs.Get(b.NextAttr),
		}, nil, nil
	}
	return nil, &Node{
		Kind: NodeKindAccess,
		Pos:  context.Pos,
		Base: &Node{
			Kind: NodeKindAccess,
			Pos:  context.Pos,
			Base: context,
			Attr: b.Attrs.Get("cond"),
		},
		Attr: b.Attrs.Get("_inner"),
	}, nil
}

func (b *builtInSelect) Tell(context, result *Node) (BuiltInOp, error) {
	if result.Kind != NodeKindIntLit {
		return nil, &BuiltInOpError{Msg: "cond argument is not an int value", Pos: result.Pos}
	}
	nextAttr := "true"
	if result.IntLit == 0 {
		nextAttr = "false"
	}
	return &builtInSelect{Attrs: b.Attrs, SeenCond: true, NextAttr: nextAttr}, nil
}

type builtInLogic struct {
	Attrs     *AttrTable
	IsAnd     bool
	XNode     *Node
	FinalNode *Node
}

func newBuiltInLogic(attrs *AttrTable, isAnd bool) *builtInLogic {
	return &builtInLogic{Attrs: attrs, IsAnd: isAnd}
}

func (b *builtInLogic) Next(context *Node) (result *Node, nextExpr *Node, err error) {
	if b.FinalNode != nil {
		return b.FinalNode, nil, nil
	} else if b.XNode != nil {
		return nil, &Node{
			Kind: NodeKindAccess,
			Pos:  context.Pos,
			Base: b.XNode,
			Attr: b.Attrs.Get("_inner"),
		}, nil
	}
	return nil, &Node{
		Kind: NodeKindAccess,
		Pos:  context.Pos,
		Base: context,
		Attr: b.Attrs.Get("x"),
	}, nil
}

func (b *builtInLogic) Tell(context, result *Node) (BuiltInOp, error) {
	if b.XNode == nil {
		return &builtInLogic{Attrs: b.Attrs, IsAnd: b.IsAnd, XNode: result}, nil
	}
	if result.Kind != NodeKindIntLit {
		return nil, &BuiltInOpError{Msg: "x argument is not an int value", Pos: result.Pos}
	}
	nextNode := b.XNode
	if (b.IsAnd && result.IntLit != 0) || (!b.IsAnd && result.IntLit == 0) {
		nextNode = &Node{
			Kind: NodeKindAccess,
			Pos:  context.Pos,
			Base: context,
			Attr: b.Attrs.Get("y"),
		}
	}
	return &builtInLogic{Attrs: b.Attrs, IsAnd: b.IsAnd, FinalNode: nextNode}, nil
}
