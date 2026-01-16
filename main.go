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
		fmt.Fprintln(os.Stderr, "error reading file: %v", err)
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
	attrs := reflex.NewAttrTable()
	node, err := parsed.Node(attrs, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to process nodes:", err)
		os.Exit(1)
	}
	access := &reflex.Node{
		Kind: reflex.NodeKindAccess,
		Pos:  reflex.Pos{File: "interpreter"},
		Base: &reflex.Node{
			Kind: reflex.NodeKindAccess,
			Pos:  reflex.Pos{File: "interpreter"},
			Base: node,
			Attr: attrs.Get("result"),
		},
		Attr: attrs.Get("_inner"),
	}
	result, err := reflex.Evaluate(attrs, access, reflex.NewGapStack())
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to evaluate:", err)
		os.Exit(1)
	}
	if result.Kind == reflex.NodeKindIntLit {
		fmt.Println(result.IntLit)
	} else if result.Kind == reflex.NodeKindStrLit {
		fmt.Println(result.StrLit)
	} else {
		fmt.Fprintln(os.Stderr, "unexpected result type:", result.Kind)
		os.Exit(1)
	}
}
