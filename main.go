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
	access := reflex.NewNodeAccess(
		reflex.Pos{File: "interpreter"},
		node,
		ctx.Attrs.Get("result"),
	)
	result, err := reflex.Evaluate(ctx, access, reflex.NewGapStack())
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to evaluate:", err)
		os.Exit(1)
	}

	resBlock, ok := unlazify(result).(reflex.Block)
	if !ok {
		fmt.Fprintln(os.Stderr, "unexpected result type:", fmt.Sprintf("%T", result))
		os.Exit(1)
	}

	if inner, ok := resBlock.Defs()[ctx.Attrs.Get("_inner")]; ok {
		switch inner := unlazify(inner).(type) {
		case *reflex.NodeIntLit:
			fmt.Println(inner.Lit)
		case *reflex.NodeStrLit:
			fmt.Println(inner.Lit)
		default:
			fmt.Fprintln(os.Stderr, "unexpected result._inner type:", fmt.Sprintf("%T", inner))
			os.Exit(1)
		}
	} else if success, ok := resBlock.Defs()[ctx.Attrs.Get("success")]; ok {
		if grabIntLiteral(ctx, unlazify(success)) == 0 {
			if errMsg, ok := resBlock.Defs()[ctx.Attrs.Get("error")]; ok {
				if errMsgBlock, ok := unlazify(errMsg).(reflex.Block); ok {
					if inner, ok := unlazify(errMsgBlock.Defs()[ctx.Attrs.Get("_inner")]).(*reflex.NodeStrLit); ok {
						fmt.Fprintln(os.Stderr, "error: "+inner.Lit)
						os.Exit(1)
					}
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

func unlazify(obj reflex.Node) reflex.Node {
	if x, ok := obj.(*reflex.NodeLazyClone); ok {
		return x.Inner()
	}
	return obj
}

func grabIntLiteral(ctx *reflex.Context, obj reflex.Node) int64 {
	if intLit, ok := obj.(*reflex.NodeIntLit); ok {
		return intLit.Lit
	} else if block, ok := obj.(reflex.Block); ok {
		if x, ok := block.Defs()[ctx.Attrs.Get("_inner")]; ok {
			if intLit, ok := unlazify(x).(*reflex.NodeIntLit); ok {
				return intLit.Lit
			}
		}
	}
	fmt.Fprintln(os.Stderr, "expected int literal but got unexpected type", fmt.Sprintf("%T", obj))
	os.Exit(1)
	panic("unreachable")
}
