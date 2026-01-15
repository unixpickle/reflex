package reflex

import (
	"fmt"
	"unicode"
)

type Pos struct {
	File string
	Line int
	Col  int
}

func (p Pos) String() string {
	return fmt.Sprintf("%s:%d:%d", p.File, p.Line, p.Col)
}

type Token struct {
	Typ string
	Val string
	Pos Pos
}

type LexError struct {
	Msg string
	Pos Pos
}

func (e *LexError) Error() string {
	return fmt.Sprintf("%s at %s", e.Msg, e.Pos)
}

type Tokenizer struct {
	src        []rune
	file       string
	i          int
	line, col  int
	singles    map[rune]string
	doubles    map[string]string
	whitespace map[rune]struct{}
}

func NewTokenizer(filename, src string) *Tokenizer {
	t := &Tokenizer{
		src:        []rune(src),
		i:          0,
		file:       filename,
		line:       1,
		col:        1,
		singles:    make(map[rune]string),
		doubles:    make(map[string]string),
		whitespace: make(map[rune]struct{}),
	}

	// punctuation = set("{}[]().=,+-/*:?<>%")
	punctuation := "{}[]().=,+-/*:?<>%"
	for _, ch := range punctuation {
		t.singles[ch] = string(ch)
	}
	// extra singles
	t.singles['^'] = "PARENT"
	t.singles['@'] = "SELF"
	t.singles['!'] = "UNWRAP"

	// double_punct = ["<-", ":=", "==", "<=", ">=", "!=", "||", "&&"]
	doublePunct := []string{"<-", ":=", "==", "<=", ">=", "!=", "||", "&&"}
	t.doubles["^^"] = "ANCESTOR"
	for _, s := range doublePunct {
		t.doubles[s] = s
	}

	// whitespace = set(" \t\r\n")
	for _, ch := range " \t\r\n" {
		t.whitespace[ch] = struct{}{}
	}

	return t
}

// Tokenize is the Go equivalent of the top-level `tokenize(src: str)` function.
func Tokenize(file, src string) ([]Token, error) {
	return NewTokenizer(file, src).Tokens()
}

// Pos gets the current position in the file.
func (t *Tokenizer) Pos() Pos {
	return Pos{File: t.file, Line: t.line, Col: t.col}
}

// Tokens corresponds to Tokenizer.tokens(self) in Python.
func (t *Tokenizer) Tokens() ([]Token, error) {
	var result []Token

	for t.cur() != 0 {
		x, err := t.nextToken()
		if err != nil {
			return nil, err
		}
		if x != nil {
			result = append(result, *x)
		}
	}

	result = append(result, Token{
		Typ: "EOF",
		Val: "",
		Pos: t.Pos(),
	})
	return result, nil
}

func (t *Tokenizer) peekDouble() (string, string, bool) {
	ch := t.cur()
	if t.peek() == 0 {
		return "", "", false
	}
	s := string([]rune{ch, t.peek()})
	typ, ok := t.doubles[s]
	return s, typ, ok
}

// nextToken corresponds to Tokenizer.next_token(self).
func (t *Tokenizer) nextToken() (*Token, error) {
	ch := t.cur()
	if ch == 0 {
		return nil, nil
	} else if _, ok := t.whitespace[ch]; ok {
		t.adv(1)
		return nil, nil
	} else if s, typ, ok := t.peekDouble(); ok {
		res := &Token{
			Typ: typ,
			Val: s,
			Pos: t.Pos(),
		}
		t.adv(2)
		return res, nil
	} else if unicode.IsDigit(ch) || (ch == '-' && unicode.IsDigit(t.peek())) {
		return t.parseIntLit(), nil
	} else if typ, ok := t.singles[ch]; ok {
		res := &Token{
			Typ: typ,
			Val: string(ch),
			Pos: t.Pos(),
		}
		t.adv(1)
		return res, nil
	} else if ch == '"' || ch == '\'' {
		return t.parseStringLit()
	} else if ch == '#' {
		for {
			x := t.cur()
			if x == 0 || x == '\n' {
				break
			}
			t.adv(1)
		}
		return nil, nil
	} else if unicode.IsLetter(ch) || ch == '_' {
		return t.parseIdentifier(), nil
	}
	println("is it unicode", unicode.IsLetter(ch))
	return nil, &LexError{
		Msg: fmt.Sprintf("Unexpected %q", ch), Pos: t.Pos(),
	}
}

func (t *Tokenizer) cur() rune {
	if t.i < len(t.src) {
		return t.src[t.i]
	}
	return 0 // sentinel for "no current char"
}

func (t *Tokenizer) adv(n int) {
	for step := 0; step < n; step++ {
		if t.cur() == 0 {
			break
		}
		if t.cur() == '\n' {
			t.line++
			t.col = 1
		} else {
			t.col++
		}
		t.i++
	}
}

func (t *Tokenizer) peek() rune {
	if t.i+1 >= len(t.src) {
		return 0
	}
	return t.src[t.i+1]
}

func (t *Tokenizer) parseStringLit() (*Token, error) {
	ch := t.cur()
	if ch != '"' && ch != '\'' {
		return nil, fmt.Errorf("parseStringLit called at non-quote character %q", ch)
	}

	startPos := t.Pos()

	quote := ch
	var buf []rune
	escape := false

	for {
		c := t.peek()
		if c == 0 {
			break
		}
		t.adv(1)

		if escape {
			switch c {
			case 'n':
				buf = append(buf, '\n')
			case 't':
				buf = append(buf, '\t')
			case 'r':
				buf = append(buf, '\r')
			case '\\':
				buf = append(buf, '\\')
			case '"':
				buf = append(buf, '"')
			case '\'':
				buf = append(buf, '\'')
			default:
				buf = append(buf, '\\')
				buf = append(buf, c)
			}
			escape = false
		}

		if c == '\\' {
			escape = true
		}
		if c == quote {
			// go past closing quote
			t.adv(1)
			return &Token{
				Typ: "STRING",
				Val: string(buf),
				Pos: startPos,
			}, nil
		}
		if c == '\n' {
			return nil, &LexError{
				Msg: "Unterminated string",
				Pos: startPos,
			}
		} else {
			buf = append(buf, c)
		}
	}

	return nil, &LexError{Msg: "Unterminated string", Pos: startPos}
}

func (t *Tokenizer) parseIntLit() *Token {
	startPos := t.Pos()

	ch := t.cur()
	if !(unicode.IsDigit(ch) || (ch == '-' && unicode.IsDigit(t.peek()))) {
		panic("parseIntLit called at non-int start")
	}

	var buf []rune
	buf = append(buf, ch)
	t.adv(1)

	for unicode.IsDigit(t.cur()) {
		buf = append(buf, t.cur())
		t.adv(1)
	}

	return &Token{
		Typ: "INT",
		Val: string(buf),
		Pos: startPos,
	}
}

func (t *Tokenizer) parseIdentifier() *Token {
	startPos := t.Pos()

	ch := t.cur()
	if !(unicode.IsLetter(ch) || ch == '_') {
		panic("unexpected start")
	}

	var buf []rune
	buf = append(buf, ch)
	t.adv(1)

	for {
		ch = t.cur()
		if unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_' {
			buf = append(buf, ch)
			t.adv(1)
		} else {
			break
		}
	}

	return &Token{
		Typ: "IDENT",
		Val: string(buf),
		Pos: startPos,
	}
}
