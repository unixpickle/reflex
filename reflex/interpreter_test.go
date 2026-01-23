package reflex

import (
	"bytes"
	"testing"
)

func TestInterpreterBasicModule(t *testing.T) {
	code := `
    x = 3
    y = {
      z = ^.x
    }
    result = y.z
  `
	testInterpreterOutput[int64](t, code, 3)
}

func TestInterpreterArithmetic(t *testing.T) {
	code := `
    x = 3
    y = 5
    result = x + y + 2
  `
	testInterpreterOutput[int64](t, code, 10)
}

func TestInterpreterEager(t *testing.T) {
	code := `
    x = 3
    y = {
      result = a + b
    }
    result = y(a=x, b:=4+5)!
  `
	testInterpreterOutput[int64](t, code, 12)
}

func TestInterpreterAliases(t *testing.T) {
	code := `
    x = 3
    y = {
      result = a + b
    }
    result = y(a=x)[b<-a]!
  `
	testInterpreterOutput[int64](t, code, 6)
}

func TestInterpreterStringLen(t *testing.T) {
	code := `
    x = "hi"
    result = x.len
  `
	testInterpreterOutput[int64](t, code, 2)
}

func TestInterpreterStringOps(t *testing.T) {
	code := `
    a = 7
    y = "hi"
    z = y + a.str
    result = z + " " + z.len.str + z.substr(start=1)!
  `
	testInterpreterOutput(t, code, "hi7 3i7")
}

func TestInterpreterFactor(t *testing.T) {
	code := `
    factor = {
      f = 2
      next_result = @(f:=f + 1)!
      result = x % f ? next_result : f
    }
    result = factor[x=533]!
  `
	testInterpreterOutput[int64](t, code, 13)
}

func TestInterpreterRecursion(t *testing.T) {
	code := `
    IntSum = {
      i = 0
      sum = 0
      result = i
        ? @(i := i - 1, sum := sum + i)!
        : sum
    }

    result = IntSum(i=10000)!
  `
	testInterpreterOutput[int64](t, code, 50005000)
}

func TestInterpreterAncestor(t *testing.T) {
	code := `
    a = {
      b = {
        c = {
          d = ^^.x
        }
      }
      x = ^^.y
    }
    y = 3
    result = a.b.c.d
  `
	testInterpreterOutput[int64](t, code, 3)
}

func TestInterpreterBytes(t *testing.T) {
	code := `
		result = "hi".bytes + 32.byte + "hey".bytes + ("test".bytes.at(y=1)!.str.bytes)
	`
	testInterpreterOutput(t, code, []byte("hi hey101"))
}

func TestInterpreterLogical(t *testing.T) {
	code := `
		result = 3 && 4
	`
	testInterpreterOutput[int64](t, code, 4)
	code = `
		result = 0 && 4
	`
	testInterpreterOutput[int64](t, code, 0)
	code = `
		result = 0 || 4
	`
	testInterpreterOutput[int64](t, code, 4)
	code = `
		result = 3 || 4
	`
	testInterpreterOutput[int64](t, code, 3)
	code = `
		result = 1 > 1 && 0
	`
	testInterpreterOutput[int64](t, code, 0)
}

func TestInterpreterNegative(t *testing.T) {
	code := `
		result = -3
	`
	testInterpreterOutput[int64](t, code, -3)
	code = `
		a = 1+2
		result = -a
	`
	testInterpreterOutput[int64](t, code, -3)
	code = `
		a = 1+2
		result = -(a+2)
	`
	testInterpreterOutput[int64](t, code, -5)
}

func TestInterpreterFloat(t *testing.T) {
	code := `
		result = -3.0 + 5.0
	`
	testInterpreterOutput[float64](t, code, 2.0)
	code = `
		result = -3.float + 5.0
	`
	testInterpreterOutput[float64](t, code, 2.0)
	code = `
		result = (-3.float + 5.5).int
	`
	testInterpreterOutput[int64](t, code, 2)
}

func testInterpreterOutput[T literal](t *testing.T, code string, expected T) {
	toks, err := Tokenize("file", code)
	if err != nil {
		t.Fatalf("failed to tokenize: %s", err)
	}
	parsed, err := Parse(toks)
	if err != nil {
		t.Fatalf("failed to parse: %s", err)
	}
	ctx := NewContext()
	node, err := parsed.Node(ctx, nil)
	if err != nil {
		t.Fatalf("failed to node-ify: %s", err)
	}
	access := &Node{
		Kind: NodeKindAccess,
		Pos:  Pos{File: "interpreter"},
		Base: &Node{
			Kind: NodeKindAccess,
			Pos:  Pos{File: "interpreter"},
			Base: node,
			Attr: ctx.Attrs.Get("result"),
		},
		Attr: ctx.Attrs.Get("_inner"),
	}
	var gs GapStack
	gs.Push(Pos{File: "test"})
	result, err := Evaluate(ctx, access, gs)
	if err != nil {
		t.Fatalf("failed to evaluate: %s", err)
	}
	var x any = expected
	if intval, ok := x.(int64); ok {
		if result.IntLit != intval {
			t.Fatalf("unexpected output: %d (expected %d)", result.IntLit, x)
		}
	} else if strval, ok := x.(string); ok {
		if result.StrLit != strval {
			t.Fatalf("unexpected output: %s (expected %s)", result.StrLit, x)
		}
	} else if bytesval, ok := x.([]byte); ok {
		if !bytes.Equal(result.BytesLit, bytesval) {
			t.Fatalf("unexpected output: %#v (expected %#v)", result.BytesLit, x)
		}
	} else if fval, ok := x.(float64); ok {
		if result.FloatLit != fval {
			t.Fatalf("unexpected output: %f (expected %f)", result.FloatLit, x)
		}
	} else {
		panic("unknown type")
	}
}
