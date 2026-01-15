package reflex

import "testing"

func TestParseBasicModule(t *testing.T) {
	code := `
    x = 3
    y = {
      z = ^.x
    }
    result = y.z
  `
	toks, err := Tokenize("file", code)
	if err != nil {
		t.Fatalf("failed to tokenize: %s", err)
	}
	parsed, err := Parse(toks)
	if err != nil {
		t.Fatalf("failed to parse: %s", err)
	}
	block, ok := parsed.(*ASTBlock)
	if !ok {
		t.Fatal("unexpected kind")
	}
	if _, ok := block.Defs["result"]; !ok {
		t.Fatal("no result")
	}
}
