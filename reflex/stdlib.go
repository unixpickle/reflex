package reflex

import (
	_ "embed"
	"os"
)

//go:embed stdlib/io
var ioCode string

//go:embed stdlib/maybe
var maybeCode string

//go:embed stdlib/collections
var collectionsCode string

func createStdNode(ctx *Context, name, code string) *Node {
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
	return node
}

func createMaybe(ctx *Context) *Node {
	return createStdNode(ctx, "maybe", maybeCode)
}

func createCollections(ctx *Context) *Node {
	return createStdNode(ctx, "collections", collectionsCode)
}

func createIO(ctx *Context) *Node {
	node := createStdNode(ctx, "io", ioCode)
	node.Defs = NewOverrideDefMap(node.Defs, NewFlatDefMap(map[Attr]*Node{
		ctx.Attrs.Get("stdout"): createFile(ctx, os.Stdout),
		ctx.Attrs.Get("stdin"):  createFile(ctx, os.Stdin),
		ctx.Attrs.Get("stderr"): createFile(ctx, os.Stderr),
	}))
	return node
}

func createFile(ctx *Context, f *os.File) *Node {
	pos := Pos{File: "<builtin/io:file>", Line: 0, Col: 0}

	result := &Node{
		Kind: NodeKindBlock,
		Pos:  pos,
	}

	write := &Node{
		Kind: NodeKindBlock,
		Pos:  pos,
	}
	write.Defs = NewFlatDefMap(map[Attr]*Node{
		ctx.Attrs.Get("result"): &Node{
			Kind: NodeKindBuiltInOp,
			Pos:  pos,
			Base: write,
			BuiltInOp: newFnBuiltInOp(
				ctx.Attrs,
				[]string{"bytes._inner"},
				func(args map[string]*Node) (*Node, error) {
					x := args["bytes._inner"]
					xValue, err := literalValue[[]byte](x)
					if err != nil {
						return nil, err
					}
					n, err := f.Write(xValue)
					return ctx.Maybe(pos, ctx.IntNode(pos, int64(n)), err), nil
				},
			),
		},
	})

	close := &Node{
		Kind: NodeKindBlock,
		Pos:  pos,
	}
	close.Defs = NewFlatDefMap(map[Attr]*Node{
		ctx.Attrs.Get("result"): &Node{
			Kind: NodeKindBuiltInOp,
			Pos:  pos,
			Base: close,
			BuiltInOp: newFnBuiltInOp(
				ctx.Attrs,
				[]string{},
				func(args map[string]*Node) (*Node, error) {
					err := f.Close()
					return ctx.Maybe(pos, nil, err), nil
				},
			),
		},
	})

	read := &Node{
		Kind: NodeKindBlock,
		Pos:  pos,
	}
	read.Defs = NewFlatDefMap(map[Attr]*Node{
		ctx.Attrs.Get("result"): &Node{
			Kind: NodeKindBuiltInOp,
			Pos:  pos,
			Base: read,
			BuiltInOp: newFnBuiltInOp(
				ctx.Attrs,
				[]string{"n._inner"},
				func(args map[string]*Node) (*Node, error) {
					nNode := args["bytes._inner"]
					n, err := literalValue[int64](nNode)
					if err != nil {
						return nil, err
					}
					out := make([]byte, n)
					actualN, err := f.Read(out)
					return ctx.Maybe(pos, ctx.BytesNode(pos, out[:actualN]), err), nil
				},
			),
		},
	})

	result.Defs = NewFlatDefMap(map[Attr]*Node{
		ctx.Attrs.Get("read"):  read,
		ctx.Attrs.Get("write"): write,
		ctx.Attrs.Get("close"): close,
	})

	return result
}
