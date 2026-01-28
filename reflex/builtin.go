package reflex

import (
	"bytes"
	"fmt"
	"math"
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

func boolToInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

type literal interface {
	int64 | float64 | string | []byte
}

func literalValue[T literal](x *Node) (T, error) {
	var zero T
	switch any(zero).(type) {
	case int64:
		if x.Kind != NodeKindIntLit {
			return zero, &BuiltInOpError{Msg: "value is not an int", Pos: x.Pos}
		}
		return any(x.IntLit).(T), nil
	case float64:
		if x.Kind != NodeKindFloatLit {
			return zero, &BuiltInOpError{Msg: "value is not a float", Pos: x.Pos}
		}
		return any(x.FloatLit).(T), nil
	case string:
		if x.Kind != NodeKindStrLit {
			return zero, &BuiltInOpError{Msg: "value is not a string", Pos: x.Pos}
		}
		return any(x.StrLit).(T), nil
	case []byte:
		if x.Kind != NodeKindBytesLit {
			return zero, &BuiltInOpError{Msg: "value is not bytes", Pos: x.Pos}
		}
		return any(x.BytesLit).(T), nil
	default:
		panic("unsupported type")
	}
}

func literalNode[T literal](ctx *Context, pos Pos, x T) *Node {
	switch x := any(x).(type) {
	case int64:
		return ctx.IntNode(pos, x)
	case float64:
		return ctx.FloatNode(pos, x)
	case string:
		return ctx.StrNode(pos, x)
	case []byte:
		return ctx.BytesNode(pos, x)
	default:
		panic("unsupported type")
	}
}

func makeFallibleUnaryOp[T, R literal](ctx *Context, pos Pos, parent *Node, fn func(T) (R, error)) *Node {
	return &Node{
		Kind: NodeKindBuiltInOp,
		Pos:  pos,
		Base: parent,
		BuiltInOp: newFnBuiltInOp(
			ctx.Attrs,
			[]string{"_inner"},
			func(args map[string]*Node) (*Node, error) {
				x := args["_inner"]
				argValue, err := literalValue[T](x)
				if err != nil {
					return nil, err
				}
				if result, err := fn(argValue); err != nil {
					return nil, err
				} else {
					return literalNode(ctx, x.Pos, result), nil
				}
			},
		),
	}
}

func makeUnaryOp[T, R literal](ctx *Context, pos Pos, parent *Node, fn func(T) R) *Node {
	return makeFallibleUnaryOp(ctx, pos, parent, func(x T) (R, error) {
		return fn(x), nil
	})
}

func makeFallibleBinaryOp[T1, T2, R literal](ctx *Context, pos Pos, parent *Node, fn func(T1, T2) (R, error)) *Node {
	op := &Node{
		Kind: NodeKindBlock,
		Pos:  pos,
	}
	op.Defs = NewFlatDefMap(map[Attr]*Node{
		ctx.Attrs.Get("x"): &Node{
			Kind: NodeKindBackEdge,
			Pos:  pos,
			Base: parent,
		},
		ctx.Attrs.Get("result"): &Node{
			Kind: NodeKindBuiltInOp,
			Pos:  pos,
			Base: op,
			BuiltInOp: newFnBuiltInOp(
				ctx.Attrs,
				[]string{"x._inner", "y._inner"},
				func(args map[string]*Node) (*Node, error) {
					x := args["x._inner"]
					y := args["y._inner"]
					xValue, err := literalValue[T1](x)
					if err != nil {
						return nil, err
					}
					yValue, err := literalValue[T2](y)
					if err != nil {
						return nil, err
					}
					out, err := fn(xValue, yValue)
					if err != nil {
						return nil, err
					}
					return literalNode(ctx, x.Pos, out), nil
				},
			),
		},
	})
	return op
}

func makeBinaryOp[T1, T2, R literal](ctx *Context, pos Pos, parent *Node, fn func(T1, T2) R) *Node {
	return makeFallibleBinaryOp(ctx, pos, parent, func(x T1, y T2) (R, error) {
		return fn(x, y), nil
	})
}

// intNode creates a node with all of the built-in int methods,
// but without an _inner for the node itself.
func intNode(ctx *Context) *Node {
	pos := Pos{File: "<builtin/int>", Line: 0, Col: 0}

	result := &Node{
		Kind: NodeKindBlock,
		Pos:  pos,
	}

	makeSelectOrLogic := func(selfName string, op BuiltInOp) *Node {
		opNode := &Node{
			Kind: NodeKindBlock,
			Pos:  pos,
		}
		opNode.Defs = NewFlatDefMap(map[Attr]*Node{
			ctx.Attrs.Get(selfName): &Node{
				Kind: NodeKindBackEdge,
				Pos:  pos,
				Base: result,
			},
			ctx.Attrs.Get("result"): &Node{
				Kind:      NodeKindBuiltInOp,
				Pos:       pos,
				Base:      opNode,
				BuiltInOp: op,
			},
		})
		return opNode
	}

	result.Defs = NewFlatDefMap(map[Attr]*Node{
		ctx.Attrs.Get("add"): makeBinaryOp(ctx, pos, result, func(x, y int64) int64 {
			return x + y
		}),
		ctx.Attrs.Get("sub"): makeBinaryOp(ctx, pos, result, func(x, y int64) int64 {
			return x - y
		}),
		ctx.Attrs.Get("div"): makeBinaryOp(ctx, pos, result, func(x, y int64) int64 {
			return x / y
		}),
		ctx.Attrs.Get("mul"): makeBinaryOp(ctx, pos, result, func(x, y int64) int64 {
			return x * y
		}),
		ctx.Attrs.Get("mod"): makeBinaryOp(ctx, pos, result, func(x, y int64) int64 {
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
		ctx.Attrs.Get("lt"): makeBinaryOp(ctx, pos, result, func(x, y int64) int64 {
			return boolToInt(x < y)
		}),
		ctx.Attrs.Get("gt"): makeBinaryOp(ctx, pos, result, func(x, y int64) int64 {
			return boolToInt(x > y)
		}),
		ctx.Attrs.Get("le"): makeBinaryOp(ctx, pos, result, func(x, y int64) int64 {
			return boolToInt(x <= y)
		}),
		ctx.Attrs.Get("ge"): makeBinaryOp(ctx, pos, result, func(x, y int64) int64 {
			return boolToInt(x >= y)
		}),
		ctx.Attrs.Get("eq"): makeBinaryOp(ctx, pos, result, func(x, y int64) int64 {
			return boolToInt(x == y)
		}),
		ctx.Attrs.Get("ne"): makeBinaryOp(ctx, pos, result, func(x, y int64) int64 {
			return boolToInt(x != y)
		}),
		ctx.Attrs.Get("chr"): makeUnaryOp(ctx, pos, result, func(x int64) string {
			return string(rune(x))
		}),
		ctx.Attrs.Get("str"): makeUnaryOp(ctx, pos, result, func(x int64) string {
			return strconv.FormatInt(x, 10)
		}),
		ctx.Attrs.Get("byte"): makeUnaryOp(ctx, pos, result, func(x int64) []byte {
			return []byte{byte(x)}
		}),
		ctx.Attrs.Get("neg"): makeUnaryOp(ctx, pos, result, func(x int64) int64 {
			return -x
		}),
		ctx.Attrs.Get("float"): makeUnaryOp(ctx, pos, result, func(x int64) float64 {
			return float64(x)
		}),
		ctx.Attrs.Get("select"):      makeSelectOrLogic("cond", newBuiltInSelect(ctx.Attrs)),
		ctx.Attrs.Get("logical_and"): makeSelectOrLogic("x", newBuiltInLogic(ctx.Attrs, true)),
		ctx.Attrs.Get("logical_or"):  makeSelectOrLogic("x", newBuiltInLogic(ctx.Attrs, false)),
	})
	return result
}

func floatNode(ctx *Context) *Node {
	pos := Pos{File: "<builtin/float>", Line: 0, Col: 0}

	result := &Node{
		Kind: NodeKindBlock,
		Pos:  pos,
	}

	result.Defs = NewFlatDefMap(map[Attr]*Node{
		ctx.Attrs.Get("add"): makeBinaryOp(ctx, pos, result, func(x, y float64) float64 {
			return x + y
		}),
		ctx.Attrs.Get("sub"): makeBinaryOp(ctx, pos, result, func(x, y float64) float64 {
			return x - y
		}),
		ctx.Attrs.Get("div"): makeBinaryOp(ctx, pos, result, func(x, y float64) float64 {
			return x / y
		}),
		ctx.Attrs.Get("mul"): makeBinaryOp(ctx, pos, result, func(x, y float64) float64 {
			return x * y
		}),
		// optional: float mod, similar sign behavior to int mod above
		ctx.Attrs.Get("mod"): makeBinaryOp(ctx, pos, result, func(x, y float64) float64 {
			r := math.Mod(x, y)
			if r < 0 {
				if y < 0 {
					r -= y
				} else {
					r += y
				}
			}
			return r
		}),

		ctx.Attrs.Get("lt"): makeBinaryOp(ctx, pos, result, func(x, y float64) int64 {
			return boolToInt(x < y)
		}),
		ctx.Attrs.Get("gt"): makeBinaryOp(ctx, pos, result, func(x, y float64) int64 {
			return boolToInt(x > y)
		}),
		ctx.Attrs.Get("le"): makeBinaryOp(ctx, pos, result, func(x, y float64) int64 {
			return boolToInt(x <= y)
		}),
		ctx.Attrs.Get("ge"): makeBinaryOp(ctx, pos, result, func(x, y float64) int64 {
			return boolToInt(x >= y)
		}),
		ctx.Attrs.Get("eq"): makeBinaryOp(ctx, pos, result, func(x, y float64) int64 {
			return boolToInt(x == y)
		}),
		ctx.Attrs.Get("ne"): makeBinaryOp(ctx, pos, result, func(x, y float64) int64 {
			return boolToInt(x != y)
		}),

		ctx.Attrs.Get("neg"): makeUnaryOp(ctx, pos, result, func(x float64) float64 {
			return -x
		}),
		ctx.Attrs.Get("str"): makeUnaryOp(ctx, pos, result, func(x float64) string {
			return strconv.FormatFloat(x, 'f', -1, 64)
		}),
		ctx.Attrs.Get("int"): makeUnaryOp(ctx, pos, result, func(x float64) int64 {
			return int64(x)
		}),
	})

	return result
}

func createSubstrOrSlice(ctx *Context, pos Pos, parent *Node, isStr bool) *Node {
	substr := &Node{
		Kind: NodeKindBlock,
		Pos:  pos,
	}
	substr.Defs = NewFlatDefMap(map[Attr]*Node{
		ctx.Attrs.Get("x"): &Node{
			Kind: NodeKindBackEdge,
			Pos:  pos,
			Base: parent,
		},
		ctx.Attrs.Get("start"): ctx.IntNode(pos, 0),
		ctx.Attrs.Get("end"): &Node{
			Kind: NodeKindAccess,
			Pos:  pos,
			Base: &Node{
				Kind: NodeKindAccess,
				Pos:  pos,
				Base: &Node{
					Kind: NodeKindBackEdge,
					Pos:  pos,
					Base: substr,
				},
				Attr: ctx.Attrs.Get("x"),
			},
			Attr: ctx.Attrs.Get("len"),
		},
		ctx.Attrs.Get("result"): &Node{
			Kind: NodeKindBuiltInOp,
			Pos:  pos,
			Base: substr,
			BuiltInOp: newFnBuiltInOp(
				ctx.Attrs,
				[]string{"x._inner", "start._inner", "end._inner"},
				func(args map[string]*Node) (*Node, error) {
					x := args["x._inner"]
					startNode := args["start._inner"]
					endNode := args["end._inner"]
					if startNode.Kind != NodeKindIntLit {
						return nil, &BuiltInOpError{
							Msg: "start argument is not an int value",
							Pos: startNode.Pos,
						}
					}
					if endNode.Kind != NodeKindIntLit {
						return nil, &BuiltInOpError{
							Msg: "end argument is not an int value",
							Pos: endNode.Pos,
						}
					}
					if isStr {
						if x.Kind != NodeKindStrLit {
							return nil, &BuiltInOpError{
								Msg: "x argument is not a str value",
								Pos: x.Pos,
							}
						}
					} else {
						if x.Kind != NodeKindBytesLit {
							return nil, &BuiltInOpError{
								Msg: "x argument is not a bytes value",
								Pos: x.Pos,
							}
						}
					}

					start := int(startNode.IntLit)
					end := int(endNode.IntLit)

					objLen := 0
					if isStr {
						objLen = len(x.StrLit)
					} else {
						objLen = len(x.BytesLit)
					}

					if start < 0 {
						start += objLen
					}
					if end < 0 {
						end += objLen
					}
					if end > objLen {
						end = objLen
					} else if end < 0 {
						end = 0
					}
					if start > objLen {
						start = objLen
					} else if start < 0 {
						start = 0
					}

					if isStr {
						str := x.StrLit
						result := ""
						if start < end {
							result = str[start:end]
						}
						return ctx.StrNode(x.Pos, result), nil
					} else {
						var res []byte
						if start < end {
							res = x.BytesLit[start:end]
						}
						return ctx.BytesNode(x.Pos, res), nil
					}
				},
			),
		},
	})
	return substr
}

// strNode creates a node with all of the built-in string methods, but
// without an _inner field for the actual value.
func strNode(ctx *Context) *Node {
	pos := Pos{File: "<builtin/str>", Line: 0, Col: 0}
	result := &Node{
		Kind: NodeKindBlock,
		Pos:  pos,
	}
	substr := createSubstrOrSlice(ctx, pos, result, true)

	importNode := &Node{
		Kind: NodeKindBlock,
		Pos:  pos,
	}
	importNode.Defs = NewFlatDefMap(map[Attr]*Node{
		ctx.Attrs.Get("x"): &Node{
			Kind: NodeKindBackEdge,
			Pos:  pos,
			Base: result,
		},
		ctx.Attrs.Get("result"): &Node{
			Kind: NodeKindBuiltInOp,
			Pos:  pos,
			Base: importNode,
			BuiltInOp: newFnBuiltInOp(
				ctx.Attrs,
				[]string{"x._inner"},
				func(args map[string]*Node) (*Node, error) {
					x := args["x._inner"]
					if x.Kind != NodeKindStrLit {
						return nil, &BuiltInOpError{
							Msg: "x argument is not a str value",
							Pos: x.Pos,
						}
					}
					return ctx.Import(x.Pos, x.StrLit)
				},
			),
		},
	})

	result.Defs = NewFlatDefMap(map[Attr]*Node{
		ctx.Attrs.Get("add"): makeBinaryOp(ctx, pos, result, func(x, y string) string {
			return x + y
		}),
		ctx.Attrs.Get("eq"): makeBinaryOp(ctx, pos, result, func(x, y string) int64 {
			return boolToInt(x == y)
		}),
		ctx.Attrs.Get("ne"): makeBinaryOp(ctx, pos, result, func(x, y string) int64 {
			return boolToInt(x != y)
		}),
		ctx.Attrs.Get("len"): makeUnaryOp(ctx, pos, result, func(s string) int64 {
			return int64(len(s))
		}),
		ctx.Attrs.Get("bytes"): makeUnaryOp(ctx, pos, result, func(x string) []byte {
			return []byte(x)
		}),
		ctx.Attrs.Get("substr"): substr,
		ctx.Attrs.Get("import"): importNode,
		ctx.Attrs.Get("panic"): &Node{
			Kind: NodeKindBuiltInOp,
			Pos:  pos,
			Base: result,
			BuiltInOp: newFnBuiltInOp(
				ctx.Attrs,
				[]string{"_inner"},
				func(args map[string]*Node) (*Node, error) {
					x := args["_inner"]
					argValue, err := literalValue[string](x)
					if err != nil {
						return nil, err
					}
					return nil, &BuiltInOpError{Msg: argValue, Pos: x.Pos}
				},
			),
		},
	})
	return result
}

// bytesNode creates a node with all of the built-in bytes methods, but
// without an _inner field for the actual value.
func bytesNode(ctx *Context) *Node {
	pos := Pos{File: "<builtin/bytes>", Line: 0, Col: 0}
	result := &Node{
		Kind: NodeKindBlock,
		Pos:  pos,
	}
	slice := createSubstrOrSlice(ctx, pos, result, false)
	result.Defs = NewFlatDefMap(map[Attr]*Node{
		ctx.Attrs.Get("add"): makeBinaryOp(ctx, pos, result, func(x, y []byte) []byte {
			return append(append([]byte{}, x...), y...)
		}),
		ctx.Attrs.Get("at"): makeFallibleBinaryOp(
			ctx,
			pos,
			result,
			func(x []byte, y int64) (int64, error) {
				origIdx := y
				if y < 0 {
					y += int64(len(x))
				}
				if y < 0 || y >= int64(len(x)) {
					return 0, &BuiltInOpError{
						Msg: fmt.Sprintf("%d is out of range [%d, %d)", origIdx, -len(x), len(x)),
						Pos: pos,
					}
				}
				return int64(x[y]), nil
			},
		),
		ctx.Attrs.Get("eq"): makeBinaryOp(ctx, pos, result, func(x, y []byte) int64 {
			return boolToInt(bytes.Equal(x, y))
		}),
		ctx.Attrs.Get("ne"): makeBinaryOp(ctx, pos, result, func(x, y []byte) int64 {
			return boolToInt(!bytes.Equal(x, y))
		}),
		ctx.Attrs.Get("len"): makeUnaryOp(ctx, pos, result, func(x []byte) int64 {
			return int64(len(x))
		}),
		ctx.Attrs.Get("str"): makeUnaryOp(ctx, pos, result, func(x []byte) string {
			return string(x)
		}),
		ctx.Attrs.Get("slice"): slice,
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
