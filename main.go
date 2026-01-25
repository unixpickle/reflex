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
	access := &reflex.Node{
		Kind: reflex.NodeKindAccess,
		Pos:  reflex.Pos{File: "interpreter"},
		Base: node,
		Attr: ctx.Attrs.Get("result"),
	}
	result, err := reflex.Evaluate(ctx, access, reflex.NewGapStack())
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to evaluate:", err)
		os.Exit(1)
	}

	if result.Kind != reflex.NodeKindBlock {
		fmt.Fprintln(os.Stderr, "unexpected result type:", result.Kind)
		os.Exit(1)
	}

	if inner, ok := result.Defs.Get(ctx.Attrs.Get("_inner")); ok {
		if inner.Kind == reflex.NodeKindIntLit {
			fmt.Println(inner.IntLit)
		} else if inner.Kind == reflex.NodeKindStrLit {
			fmt.Println(inner.StrLit)
		} else {
			fmt.Fprintln(os.Stderr, "unexpected result._inner type:", result.Kind)
			os.Exit(1)
		}
	} else if success, ok := result.Defs.Get(ctx.Attrs.Get("success")); ok {
		if grabIntLiteral(ctx, success) == 0 {
			if errMsg, ok := result.Defs.Get(ctx.Attrs.Get("error")); ok {
				if inner, ok := errMsg.Defs.Get(ctx.Attrs.Get("_inner")); ok {
					fmt.Fprintln(os.Stderr, "error: "+inner.StrLit)
					os.Exit(1)
				}
			}
			fmt.Fprintln(os.Stderr, "unsuccessful result")
			os.Exit(1)
		}
	} else {
		fmt.Fprintln(os.Stderr, "an unexpected result structure was returned")
		os.Exit(1)
	}
}

func grabIntLiteral(ctx *reflex.Context, obj *reflex.Node) int64 {
	if obj.Kind == reflex.NodeKindIntLit {
		return obj.IntLit
	} else if obj.Kind == reflex.NodeKindBlock {
		if x, ok := obj.Defs.Get(ctx.Attrs.Get("_inner")); ok {
			if x.Kind == reflex.NodeKindIntLit {
				return x.IntLit
			}
		}
	}
	fmt.Fprintln(os.Stderr, "expected int literal but got unexpected type")
	os.Exit(1)
	panic("unreachable")
}
