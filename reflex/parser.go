package reflex

import (
	"fmt"
	"strconv"
)

type ParseError struct {
	Msg string
	Pos Pos
}

func (p *ParseError) Error() string {
	return fmt.Sprintf("%s at %s", p.Msg, p.Pos)
}

var binaryOpPrecedence = map[string]int{
	"||": 3,
	"&&": 4,
	"==": 5,
	"!=": 5,
	"<=": 7,
	">=": 7,
	"<":  7,
	">":  7,
	"+":  10,
	"-":  10,
	"*":  20,
	"/":  20,
	"%":  20,
}

type Parser struct {
	toks []*Token
	k    int
}

func NewParser(toks []*Token) *Parser {
	return &Parser{toks: toks}
}

func (p *Parser) peek() *Token {
	if p.k >= len(p.toks) {
		return p.toks[len(p.toks)-1]
	}
	return p.toks[p.k]
}

func (p *Parser) match(types ...string) *Token {
	t := p.peek()
	for _, x := range types {
		if t.Typ == x {
			p.k += 1
			return t
		}
	}
	return nil
}

func (p *Parser) expect(types ...string) (*Token, error) {
	if t := p.match(types...); t != nil {
		return t, nil
	}
	return nil, &ParseError{
		Msg: fmt.Sprintf("expected type in %#v but got %#v", types, p.peek().Typ),
		Pos: p.peek().Pos,
	}
}

func (p *Parser) consumeDelims() {
	for p.peek().Typ == "," {
		p.k += 1
	}
}

func (p *Parser) parseModule() (ASTNode, error) {
	startPos := p.peek().Pos
	defs, _, _, err := p.parseDefsUntil("EOF", false, false)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect("EOF"); err != nil {
		return nil, err
	}
	return &ASTBlock{Pos: startPos, Defs: defs}, nil
}

func (p *Parser) parseDefsUntil(
	terminator string,
	allowAliases,
	allowEager bool,
) (defs map[string]ASTNode, aliases map[string]string, eager map[string]ASTNode, err error) {
	defs = map[string]ASTNode{}
	if allowAliases {
		aliases = map[string]string{}
	}
	if allowEager {
		eager = map[string]ASTNode{}
	}
	for p.peek().Typ != terminator {
		name, err := p.expect("IDENT")
		if err != nil {
			return nil, nil, nil, err
		}
		t := p.peek()
		if t.Typ == "=" {
			val, err := p.parseExpr()
			if err != nil {
				return nil, nil, nil, err
			}
			defs[name.Val] = val
		} else if allowAliases && t.Typ == "<-" {
			ident, err := p.expect("IDENT")
			if err != nil {
				return nil, nil, nil, err
			}
			aliases[name.Val] = ident.Val
		} else if allowEager && t.Typ == ":=" {
			val, err := p.parseExpr()
			if err != nil {
				return nil, nil, nil, err
			}
			eager[name.Val] = val
		} else {
			return nil, nil, nil, &ParseError{Msg: fmt.Sprintf("unexpected token %#v inside definition", t.Typ), Pos: t.Pos}
		}
		p.consumeDelims()
	}
	return
}

func (p *Parser) parseExpr() (ASTNode, error) {
	node, err := p.parseBinary(0)
	if err != nil {
		return nil, err
	}
	if t := p.match("?"); t != nil {
		trueExpr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(":"); err != nil {
			return nil, err
		}
		falseExpr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		return &ASTTernary{Pos: t.Pos, Cond: node, IfTrue: trueExpr, IfFalse: falseExpr}, nil
	}
	return node, nil
}

func (p *Parser) parseBinary(minPrec int) (ASTNode, error) {
	node, err := p.parsePostfix()
	if err != nil {
		return nil, err
	}
	for {
		t := p.peek()
		op := t.Typ
		prec, ok := binaryOpPrecedence[op]
		if !ok || prec < minPrec {
			break
		}
		p.k += 1
		rhs, err := p.parseBinary(minPrec + 1)
		if err != nil {
			return nil, err
		}
		opName := map[string]string{
			"==": "eq",
			"!=": "ne",
			"<":  "lt",
			">":  "gt",
			">=": "ge",
			"<=": "le",
			"+":  "add",
			"-":  "sub",
			"/":  "div",
			"*":  "mul",
			"%":  "mod",
			"&&": "logical_and",
			"||": "logical_or",
		}[op]
		node = &ASTBinaryOp{
			Pos:    t.Pos,
			OpName: opName,
			X:      node,
			Y:      rhs,
		}
	}
	return node, nil
}

func (p *Parser) parsePostfix() (ASTNode, error) {
	node, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for {
		if p.match(".") != nil {
			startPos := p.peek().Pos
			if p.match("PARENT") != nil {
				if oldParent, ok := node.(*ASTParent); ok {
					node = &ASTParent{Pos: oldParent.Pos, Depth: oldParent.Depth + 1}
				} else {
					return nil, &ParseError{Msg: "invalid parent chaining", Pos: startPos}
				}
			} else {
				attr, err := p.expect("IDENT")
				if err != nil {
					return nil, err
				}
				node = &ASTAccess{Pos: attr.Pos, Base: node, Attr: attr.Val}
			}
		} else if uw := p.match("UNWRAP"); uw != nil {
			node = &ASTAccess{Pos: uw.Pos, Base: node, Attr: "result"}
		} else if open := p.match("["); open != nil {
			defs, aliases, _, err := p.parseDefsUntil("]", true, false)
			if err != nil {
				return nil, err
			}
			if _, err := p.expect("]"); err != nil {
				return nil, err
			}
			node = &ASTOverride{Pos: open.Pos, Base: node, Defs: defs, Aliases: aliases}
		} else if open := p.match("("); open != nil {
			defs, _, eager, err := p.parseDefsUntil("]", false, true)
			if err != nil {
				return nil, err
			}
			if _, err := p.expect("("); err != nil {
				return nil, err
			}
			node = &ASTCall{Pos: open.Pos, Base: node, Defs: defs, Eager: eager}
		} else {
			break
		}
	}
	return node, nil
}

func (p *Parser) parsePrimary() (ASTNode, error) {
	startPos := p.peek().Pos
	tok, err := p.expect("{", "INT", "STRING", "SELF", "PARENT", "ANCESTOR", "IDENT", "(")
	if err != nil {
		return nil, err
	}
	switch tok.Typ {
	case "{":
		defs, _, _, err := p.parseDefsUntil("}", false, false)
		if err != nil {
			return nil, err
		}
		return &ASTBlock{Pos: startPos, Defs: defs}, nil
	case "INT":
		value, err := strconv.ParseInt(tok.Val, 10, 64)
		if err != nil {
			return nil, &ParseError{Msg: err.Error(), Pos: startPos}
		}
		return &ASTIntLit{Pos: startPos, Value: value}, nil
	case "STRING":
		return &ASTStrLit{Pos: startPos, Value: tok.Val}, nil
	case "SELF":
		return &ASTSelfRef{Pos: startPos}, nil
	case "PARENT":
		return &ASTParent{Pos: startPos, Depth: 1}, nil
	case "ANCESTOR":
		if _, err := p.expect("."); err != nil {
			return nil, err
		}
		name, err := p.expect("IDENT")
		if err != nil {
			return nil, err
		}
		return &ASTAncestorLookup{Pos: startPos, Attr: name.Val}, nil
	case "IDENT":
		return &ASTIdentifier{Pos: startPos, Name: tok.Val}, nil
	case "(":
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(")"); err != nil {
			return nil, err
		}
		return expr, nil
	default:
		panic("unreachable")
	}
}
