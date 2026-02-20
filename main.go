package main

import (
	"fmt"
	"os"

	"github.com/unixpickle/reflex/reflex"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "Usage: go run . <file>")
		os.Exit(1)
	}
	path := os.Args[1]

	content, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error reading file:", err)
		os.Exit(1)
	}

	contentStr := string(content)

	toks, err := reflex.Tokenize(path, contentStr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to tokenize:", err)
		os.Exit(1)
	}
	parsed, err := reflex.Parse(toks)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to parse:", err)
		os.Exit(1)
	}
	ctx := reflex.NewContext()
	node, err := parsed.Node(ctx, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to process nodes:", err)
		os.Exit(1)
	}
	pos := reflex.Pos{File: "interpreter"}
	base := reflex.NodeBase{P: pos}
	access := &reflex.NodeAccess{
		NodeBase: base,
		Base:     node,
		Attr:     ctx.Attrs.Get("result"),
	}
	rawResult, err := reflex.Evaluate(ctx, access, reflex.NewGapStack(), nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to evaluate:", err)
		os.Exit(1)
	}
	result := mustCast[*reflex.NodeBlock]("result", rawResult)

	if inner, ok := result.Defs.Get(ctx.Attrs.Get("_inner")); ok {
		switch inner := inner.(type) {
		case *reflex.NodeIntLit:
			fmt.Println(inner.Lit)
		case *reflex.NodeStrLit:
			fmt.Println(inner.Lit)
		default:
			fmt.Fprintln(os.Stderr, "unexpected type for result._inner:", fmt.Sprintf("%T", inner))
			os.Exit(1)
		}
	} else if success, ok := result.Defs.Get(ctx.Attrs.Get("success")); ok {
		if mustCastInner[*reflex.NodeIntLit](ctx, "result.success", success).Lit == 0 {
			if errObj, ok := result.Defs.Get(ctx.Attrs.Get("error")); ok {
				errLit := mustCastInner[*reflex.NodeStrLit](ctx, "result.error", errObj)
				fmt.Fprintln(os.Stderr, "error: "+errLit.Lit)
				os.Exit(1)
			}
			fmt.Fprintln(os.Stderr, "unsuccessful result with no error")
			os.Exit(1)
		}
	} else {
		fmt.Fprintln(os.Stderr, "the program's result is not a literal or a `maybe` structure")
		os.Exit(1)
	}
}

func mustCast[T any](name string, obj any) T {
	if x, ok := obj.(T); ok {
		return x
	}
	fmt.Fprintln(os.Stderr, fmt.Sprintf("unexpected type for %s: %T", name, obj))
	os.Exit(1)
	panic("")
}

func mustCastInner[T any](ctx *reflex.Context, name string, obj any) T {
	block := mustCast[*reflex.NodeBlock](name, obj)
	inner, ok := block.Defs.Get(ctx.Attrs.Get("_inner"))
	if !ok {
		fmt.Fprintln(os.Stderr, fmt.Sprintf("missing _inner on %s", name))
		os.Exit(1)
	}
	return mustCast[T](name+"._inner", inner)
}
