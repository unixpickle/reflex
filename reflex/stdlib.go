package reflex

import (
	_ "embed"
	"os"
)

//go:embed stdlib/io
var ioCode string

//go:embed stdlib/errors
var errorsCode string

//go:embed stdlib/collections
var collectionsCode string

func createStdNode(ctx *Context, name, code string) *NodeBlock {
	toks, err := Tokenize("<stdlib/"+name+">", code)
	if err != nil {
		panic(err)
	}
	ast, err := Parse(toks)
	if err != nil {
		panic(err)
	}
	node, err := ast.Node(ctx, nil)
	if err != nil {
		panic(err)
	}
	return node.(*NodeBlock)
}

func createErrors(ctx *Context) *NodeBlock {
	return createStdNode(ctx, "errors", errorsCode)
}

func createCollections(ctx *Context) *NodeBlock {
	return createStdNode(ctx, "collections", collectionsCode)
}

func createIO(ctx *Context) *NodeBlock {
	node := createStdNode(ctx, "io", ioCode)
	node = node.Override(ctx.BackEdges, node.Pos(), map[Attr]Node{
		ctx.Attrs.Get("stdout"): createFile(ctx, os.Stdout),
		ctx.Attrs.Get("stdin"):  createFile(ctx, os.Stdin),
		ctx.Attrs.Get("stderr"): createFile(ctx, os.Stderr),
	})
	return node
}

func createFile(ctx *Context, f *os.File) Node {
	pos := Pos{File: "<builtin/io:file>", Line: 0, Col: 0}
	return NewNodeBlock(
		ctx.BackEdges,
		pos,
		nil,
		func(result Scope) map[Attr]Node {
			return map[Attr]Node{
				ctx.Attrs.Get("write"): NewNodeBlock(
					ctx.BackEdges,
					pos,
					nil,
					func(write Scope) map[Attr]Node {
						return map[Attr]Node{
							ctx.Attrs.Get("result"): NewNodeBuiltInOp(
								ctx.BackEdges,
								pos,
								write,
								newFnBuiltInOp(
									ctx.Attrs,
									[]string{"bytes._inner"},
									func(args map[string]Node) (Node, error) {
										x := args["bytes._inner"]
										xValue, err := literalValue[[]byte](x)
										if err != nil {
											return nil, err
										}
										n, err := f.Write(xValue)
										return ctx.Maybe(pos, ctx.IntNode(pos, int64(n)), err), nil
									},
								),
							),
						}
					},
				),
				ctx.Attrs.Get("close"): NewNodeBlock(
					ctx.BackEdges,
					pos,
					nil,
					func(close Scope) map[Attr]Node {
						return map[Attr]Node{
							ctx.Attrs.Get("result"): NewNodeBuiltInOp(
								ctx.BackEdges,
								pos,
								close,
								newFnBuiltInOp(
									ctx.Attrs,
									[]string{},
									func(args map[string]Node) (Node, error) {
										err := f.Close()
										return ctx.Maybe(pos, nil, err), nil
									},
								),
							),
						}
					},
				),
				ctx.Attrs.Get("read"): NewNodeBlock(
					ctx.BackEdges,
					pos,
					nil,
					func(read Scope) map[Attr]Node {
						return map[Attr]Node{
							ctx.Attrs.Get("result"): NewNodeBuiltInOp(
								ctx.BackEdges,
								pos,
								read,
								newFnBuiltInOp(
									ctx.Attrs,
									[]string{"n._inner"},
									func(args map[string]Node) (Node, error) {
										nNode := args["n._inner"]
										n, err := literalValue[int64](nNode)
										if err != nil {
											return nil, err
										}
										out := make([]byte, n)
										actualN, err := f.Read(out)
										return ctx.Maybe(pos, ctx.BytesNode(pos, out[:actualN]), err), nil
									},
								),
							),
						}
					},
				),
			}
		},
	)
}
