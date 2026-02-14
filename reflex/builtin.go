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

func literalValue[T literal](x Node) (T, error) {
	var zero T
	switch any(zero).(type) {
	case int64:
		if lit, ok := x.(*NodeIntLit); !ok {
			return zero, &BuiltInOpError{Msg: "value is not an int", Pos: x.Pos()}
		} else {
			return any(lit.Lit).(T), nil
		}
	case float64:
		if lit, ok := x.(*NodeFloatLit); !ok {
			return zero, &BuiltInOpError{Msg: "value is not a float", Pos: x.Pos()}
		} else {
			return any(lit.Lit).(T), nil
		}
	case string:
		if lit, ok := x.(*NodeStrLit); !ok {
			return zero, &BuiltInOpError{Msg: "value is not a string", Pos: x.Pos()}
		} else {
			return any(lit.Lit).(T), nil
		}
	case []byte:
		if lit, ok := x.(*NodeBytesLit); !ok {
			return zero, &BuiltInOpError{Msg: "value is not bytes", Pos: x.Pos()}
		} else {
			return any(lit.Lit).(T), nil
		}
	default:
		panic("unsupported type")
	}
}

func literalNode[T literal](ctx *Context, pos Pos, x T) Node {
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

func makeFallibleUnaryOp[T, R literal](
	ctx *Context,
	pos Pos,
	parent Scope,
	fn func(T) (R, error),
) Node {
	return NewNodeBuiltInOp(
		ctx.BackEdges,
		pos,
		parent,
		newFnBuiltInOp(
			ctx.Attrs,
			[]string{"_inner"},
			func(args map[string]Node) (Node, error) {
				x := args["_inner"]
				argValue, err := literalValue[T](x)
				if err != nil {
					return nil, err
				}
				if result, err := fn(argValue); err != nil {
					return nil, err
				} else {
					return literalNode(ctx, x.Pos(), result), nil
				}
			},
		),
	)
}

func makeUnaryOp[T, R literal](ctx *Context, pos Pos, parent Scope, fn func(T) R) Node {
	return makeFallibleUnaryOp(ctx, pos, parent, func(x T) (R, error) {
		return fn(x), nil
	})
}

func makeFallibleBinaryOp[T1, T2, R literal](
	ctx *Context,
	pos Pos,
	parent Scope,
	fn func(T1, T2) (R, error),
) Node {
	return NewNodeBlock(
		ctx.BackEdges,
		pos,
		nil,
		func(op Scope) map[Attr]Node {
			return map[Attr]Node{
				ctx.Attrs.Get("x"): NewNodeBackEdge(ctx.BackEdges, pos, parent),
				ctx.Attrs.Get("result"): NewNodeBuiltInOp(
					ctx.BackEdges,
					pos,
					op,
					newFnBuiltInOp(
						ctx.Attrs,
						[]string{"x._inner", "y._inner"},
						func(args map[string]Node) (Node, error) {
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
							return literalNode(ctx, x.Pos(), out), nil
						},
					),
				),
			}
		},
	)
}

func makeBinaryOp[T1, T2, R literal](ctx *Context, pos Pos, parent Scope, fn func(T1, T2) R) Node {
	return makeFallibleBinaryOp(ctx, pos, parent, func(x T1, y T2) (R, error) {
		return fn(x, y), nil
	})
}

// intNode creates a node with all of the built-in int methods,
// but without an _inner for the node itself.
func intNode(ctx *Context) *NodeBlock {
	pos := Pos{File: "<builtin/int>", Line: 0, Col: 0}

	return NewNodeBlock(
		ctx.BackEdges,
		pos,
		nil,
		func(result Scope) map[Attr]Node {
			makeSelectOrLogic := func(selfName string, op BuiltInOp) Node {
				return NewNodeBlock(
					ctx.BackEdges,
					pos,
					nil,
					func(opNode Scope) map[Attr]Node {
						return map[Attr]Node{
							ctx.Attrs.Get(selfName): NewNodeBackEdge(
								ctx.BackEdges,
								pos,
								result,
							),
							ctx.Attrs.Get("result"): NewNodeBuiltInOp(
								ctx.BackEdges,
								pos,
								opNode,
								op,
							),
						}
					},
				)
			}

			return map[Attr]Node{
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
			}
		},
	)
}

func floatNode(ctx *Context) *NodeBlock {
	pos := Pos{File: "<builtin/float>", Line: 0, Col: 0}

	return NewNodeBlock(
		ctx.BackEdges,
		pos,
		nil,
		func(result Scope) map[Attr]Node {
			return map[Attr]Node{
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
			}
		},
	)
}

func createSubstrOrSlice(ctx *Context, pos Pos, parent Scope, isStr bool) *NodeBlock {
	return NewNodeBlock(
		ctx.BackEdges,
		pos,
		nil,
		func(substr Scope) map[Attr]Node {
			return map[Attr]Node{
				ctx.Attrs.Get("x"):     NewNodeBackEdge(ctx.BackEdges, pos, parent),
				ctx.Attrs.Get("start"): ctx.IntNode(pos, 0),
				ctx.Attrs.Get("end"): NewNodeAccess(
					pos,
					NewNodeAccess(
						pos,
						NewNodeBackEdge(ctx.BackEdges, pos, substr),
						ctx.Attrs.Get("x"),
					),
					ctx.Attrs.Get("len"),
				),
				ctx.Attrs.Get("result"): NewNodeBuiltInOp(
					ctx.BackEdges,
					pos,
					substr,
					newFnBuiltInOp(
						ctx.Attrs,
						[]string{"x._inner", "start._inner", "end._inner"},
						func(args map[string]Node) (Node, error) {
							x := args["x._inner"]
							startNode, ok := args["start._inner"].(*NodeIntLit)
							if !ok {
								return nil, &BuiltInOpError{
									Msg: "start argument is not an int value",
									Pos: startNode.Pos(),
								}
							}
							endNode, ok := args["end._inner"].(*NodeIntLit)
							if !ok {
								return nil, &BuiltInOpError{
									Msg: "end argument is not an int value",
									Pos: endNode.Pos(),
								}
							}
							if isStr {
								if _, ok := x.(*NodeStrLit); !ok {
									return nil, &BuiltInOpError{
										Msg: "x argument is not a str value",
										Pos: x.Pos(),
									}
								}
							} else {
								if _, ok := x.(*NodeBytesLit); !ok {
									return nil, &BuiltInOpError{
										Msg: "x argument is not a bytes value",
										Pos: x.Pos(),
									}
								}
							}

							start := int(startNode.Lit)
							end := int(endNode.Lit)

							objLen := 0
							if isStr {
								objLen = len(x.(*NodeStrLit).Lit)
							} else {
								objLen = len(x.(*NodeBytesLit).Lit)
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
								result := ""
								if start < end {
									result = x.(*NodeStrLit).Lit[start:end]
								}
								return ctx.StrNode(x.Pos(), result), nil
							} else {
								var res []byte
								if start < end {
									res = x.(*NodeBytesLit).Lit[start:end]
								}
								return ctx.BytesNode(x.Pos(), res), nil
							}
						},
					),
				),
			}
		},
	)
}

// strNode creates a node with all of the built-in string methods, but
// without an _inner field for the actual value.
func strNode(ctx *Context) *NodeBlock {
	pos := Pos{File: "<builtin/str>", Line: 0, Col: 0}
	return NewNodeBlock(
		ctx.BackEdges,
		pos,
		nil,
		func(result Scope) map[Attr]Node {
			return map[Attr]Node{
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
				ctx.Attrs.Get("substr"): createSubstrOrSlice(ctx, pos, result, true),
				ctx.Attrs.Get("import"): NewNodeBlock(
					ctx.BackEdges,
					pos,
					nil,
					func(importNode Scope) map[Attr]Node {
						return map[Attr]Node{
							ctx.Attrs.Get("x"): NewNodeBackEdge(ctx.BackEdges, pos, result),
							ctx.Attrs.Get("result"): NewNodeBuiltInOp(
								ctx.BackEdges,
								pos,
								importNode,
								newFnBuiltInOp(
									ctx.Attrs,
									[]string{"x._inner"},
									func(args map[string]Node) (Node, error) {
										x := args["x._inner"]
										if x, ok := x.(*NodeStrLit); !ok {
											return nil, &BuiltInOpError{
												Msg: "x argument is not a str value",
												Pos: x.Pos(),
											}
										} else {
											return ctx.Import(x.Pos(), x.Lit)
										}
									},
								),
							),
						}
					},
				),
				ctx.Attrs.Get("panic"): NewNodeBuiltInOp(
					ctx.BackEdges,
					pos,
					result,
					newFnBuiltInOp(
						ctx.Attrs,
						[]string{"_inner"},
						func(args map[string]Node) (Node, error) {
							x := args["_inner"]
							argValue, err := literalValue[string](x)
							if err != nil {
								return nil, err
							}
							return nil, &BuiltInOpError{Msg: argValue, Pos: x.Pos()}
						},
					),
				),
			}
		},
	)
}

// bytesNode creates a node with all of the built-in bytes methods, but
// without an _inner field for the actual value.
func bytesNode(ctx *Context) *NodeBlock {
	pos := Pos{File: "<builtin/bytes>", Line: 0, Col: 0}
	return NewNodeBlock(
		ctx.BackEdges,
		pos,
		nil,
		func(result Scope) map[Attr]Node {
			return map[Attr]Node{
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
								Msg: fmt.Sprintf(
									"%d is out of range [%d, %d)",
									origIdx,
									-len(x),
									len(x),
								),
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
				ctx.Attrs.Get("slice"): createSubstrOrSlice(ctx, pos, result, false),
			}
		},
	)
}

// A BuiltInOp uses a context and optionally evaluates expressions in the context
// to perform an operation.
type BuiltInOp interface {
	// Next either tells the interpreter the final result, or indicates that
	// another expression must be evaluated first.
	Next(context Node) (result Node, nextExpr Node, err error)

	// Tell gives the operation the next evaluated expression, and gets a new BuiltInOp
	// which can perform the next step.
	Tell(context Node, result Node) (BuiltInOp, error)
}

type fnBuiltInOp struct {
	Attrs *AttrTable
	Paths []string
	Fn    func(args map[string]Node) (Node, error)
	Found map[string]Node
}

func newFnBuiltInOp(
	attrs *AttrTable,
	paths []string,
	fn func(args map[string]Node) (Node, error),
) *fnBuiltInOp {
	return &fnBuiltInOp{
		Attrs: attrs,
		Paths: paths,
		Fn:    fn,
		Found: map[string]Node{},
	}
}

func (f *fnBuiltInOp) Next(context Node) (result Node, nextExpr Node, err error) {
	for _, p := range f.Paths {
		if _, ok := f.Found[p]; !ok {
			// Create an access chain and return it.
			accessChain := context
			for _, part := range strings.Split(p, ".") {
				accessChain = NewNodeAccess(context.Pos(), accessChain, f.Attrs.Get(part))
			}
			return nil, accessChain, nil
		}
	}
	out, err := f.Fn(f.Found)
	return out, nil, err
}

func (f *fnBuiltInOp) Tell(context, result Node) (BuiltInOp, error) {
	newResult := map[string]Node{}
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

func (b *builtInSelect) Next(context Node) (result Node, nextExpr Node, err error) {
	if b.SeenCond {
		return NewNodeAccess(context.Pos(), context, b.Attrs.Get(b.NextAttr)), nil, nil
	}
	return nil, NewNodeAccess(
		context.Pos(),
		NewNodeAccess(
			context.Pos(),
			context,
			b.Attrs.Get("cond"),
		),
		b.Attrs.Get("_inner"),
	), nil
}

func (b *builtInSelect) Tell(context, result Node) (BuiltInOp, error) {
	intLit, ok := result.(*NodeIntLit)
	if !ok {
		return nil, &BuiltInOpError{Msg: "cond argument is not an int value", Pos: result.Pos()}
	}
	nextAttr := "true"
	if intLit.Lit == 0 {
		nextAttr = "false"
	}
	return &builtInSelect{Attrs: b.Attrs, SeenCond: true, NextAttr: nextAttr}, nil
}

type builtInLogic struct {
	Attrs     *AttrTable
	IsAnd     bool
	XNode     Node
	FinalNode Node
}

func newBuiltInLogic(attrs *AttrTable, isAnd bool) *builtInLogic {
	return &builtInLogic{Attrs: attrs, IsAnd: isAnd}
}

func (b *builtInLogic) Next(context Node) (result Node, nextExpr Node, err error) {
	if b.FinalNode != nil {
		return b.FinalNode, nil, nil
	} else if b.XNode != nil {
		return nil, NewNodeAccess(context.Pos(), b.XNode, b.Attrs.Get("_inner")), nil
	}
	return nil, NewNodeAccess(context.Pos(), context, b.Attrs.Get("x")), nil
}

func (b *builtInLogic) Tell(context, result Node) (BuiltInOp, error) {
	if b.XNode == nil {
		return &builtInLogic{Attrs: b.Attrs, IsAnd: b.IsAnd, XNode: result}, nil
	}
	intLit, ok := result.(*NodeIntLit)
	if !ok {
		return nil, &BuiltInOpError{Msg: "x argument is not an int value", Pos: result.Pos()}
	}
	nextNode := b.XNode
	if (b.IsAnd && intLit.Lit != 0) || (!b.IsAnd && intLit.Lit == 0) {
		nextNode = NewNodeAccess(context.Pos(), context, b.Attrs.Get("y"))
	}
	return &builtInLogic{Attrs: b.Attrs, IsAnd: b.IsAnd, FinalNode: nextNode}, nil
}
