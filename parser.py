from dataclasses import dataclass
from typing import List


class Node:
    pass


@dataclass
class Definition(Node):
    name: str
    expr: Node


@dataclass
class Block(Node):
    defs: List[Definition]


@dataclass
class Identifier(Node):
    name: str


@dataclass
class IntLit(Node):
    value: int


@dataclass
class StringLit(Node):
    value: str


@dataclass
class SelfRef(Node):
    pass


@dataclass
class Parent(Node):
    depth: int


@dataclass
class Access(Node):
    base: Node
    attr: str


@dataclass
class Override(Node):
    base: Node
    defs: List[Definition]


# =========================
#         LEXER
# =========================


class Token:
    def __init__(self, typ, val, line=0, col=0):
        self.typ = typ
        self.val = val
        self.line = line
        self.col = col

    def __repr__(self):
        return f"Token({self.typ},{self.val})"


class LexError(Exception):
    pass


class ParseError(Exception):
    pass


def tokenize(src: str):
    tokens = []
    i = 0
    line = 1
    col = 1

    def adv(n=1):
        nonlocal i, line, col
        for _ in range(n):
            if i < len(src) and src[i] == "\n":
                line += 1
                col = 1
            else:
                col += 1
            i += 1

    def add(typ, val):
        tokens.append(Token(typ, val, line, col))

    def peek(k=0):
        return src[i + k] if i + k < len(src) else ""

    while i < len(src):
        ch = src[i]
        if ch in " \t\r":
            adv()
            continue
        if ch == "\n":
            add("NEWLINE", "\n")
            adv()
            continue
        # line comments: lines starting with --- (as in your sample)
        if ch == "-" and (i == 0 or src[i - 1] == "\n"):
            if src[i : i + 3] == "---":
                while i < len(src) and src[i] != "\n":
                    adv()
                continue

        # strings: "..." or '...'
        if ch == '"' or ch == "'":
            quote = ch
            j = i + 1
            buf = []
            escape = False
            start_line, start_col = line, col
            while j < len(src):
                c = src[j]
                if escape:
                    if c == "n":
                        buf.append("\n")
                    elif c == "t":
                        buf.append("\t")
                    elif c == "r":
                        buf.append("\r")
                    elif c == "\\":
                        buf.append("\\")
                    elif c == '"':
                        buf.append('"')
                    elif c == "'":
                        buf.append("'")
                    else:
                        # preserve unknown escape literally
                        buf.append("\\")
                        buf.append(c)
                    escape = False
                    j += 1
                    continue
                if c == "\\":
                    escape = True
                    j += 1
                    continue
                if c == quote:
                    add("STRING", "".join(buf))
                    # advance past closing quote
                    adv(j - i + 1)
                    break
                if c == "\n":
                    raise LexError(
                        f"Unterminated string starting at {start_line}:{start_col}"
                    )
                buf.append(c)
                j += 1
            else:
                raise LexError(
                    f"Unterminated string starting at {start_line}:{start_col}"
                )
            continue

        # explicit error for the removed ancestor operator
        if ch == "^" and peek(1) == "^":
            raise LexError(
                f"The '^^' ancestor lookup operator was removed at {line}:{col}. "
                "Remove it or replace with your intended alternative."
            )

        if ch in "{}[].=,;":
            add(ch, ch)
            adv()
            continue
        if ch == "^":
            add("PARENT", "^")
            adv()
            continue
        if ch == "@":
            add("SELF", "@")
            adv()
            continue
        if ch == "$":
            add("DOLLAR", "$")
            adv()
            continue
        if ch.isdigit() or (ch == "-" and peek(1).isdigit()):
            j = i + 1
            if ch == "-":
                j = i + 2
            while j < len(src) and src[j].isdigit():
                j += 1
            val = src[i:j]
            add("INT", val)
            adv(len(val))
            continue
        if ch.isalpha() or ch == "_":
            j = i + 1
            while j < len(src) and (src[j].isalnum() or src[j] == "_"):
                j += 1
            val = src[i:j]
            add("IDENT", val)
            adv(len(val))
            continue
        raise LexError(f"Unexpected {ch!r} at {line}:{col}")
    tokens.append(Token("EOF", "", line, col))
    return tokens


# =========================
#         PARSER
# =========================


class Parser:
    def __init__(self, toks):
        self.toks = toks
        self.k = 0

    def peek(self, t=0):
        k = self.k + t
        return self.toks[k] if k < len(self.toks) else self.toks[-1]

    def match(self, *types):
        if self.peek().typ in types:
            tok = self.peek()
            self.k += 1
            return tok
        return None

    def expect(self, typ):
        tok = self.match(typ)
        if not tok:
            raise ParseError(
                f"Expected {typ}, got {self.peek().typ} at {self.peek().line}:{self.peek().col}"
            )
        return tok

    def consume_delims(self):
        while self.peek().typ in {",", ";", "NEWLINE"}:
            self.k += 1

    def parse_module(self) -> Block:
        defs = []
        self.consume_delims()
        while self.peek().typ not in {"EOF"}:
            if self.peek().typ not in {"IDENT", "DOLLAR"}:
                break
            defs.append(self.parse_definition())
            self.consume_delims()
        self.expect("EOF")
        return Block(defs)

    def parse_definition(self) -> Definition:
        if self.peek().typ == "IDENT":
            name = self.expect("IDENT").val
        elif self.peek().typ == "DOLLAR":
            name = self.expect("DOLLAR").val
        else:
            raise ParseError("ident expected")
        self.expect("=")
        expr = self.parse_expr()
        return Definition(name, expr)

    def parse_def_list_until(self, stop):
        defs = []
        self.consume_delims()
        while self.peek().typ not in stop:
            defs.append(self.parse_definition())
            self.consume_delims()
        return defs

    def parse_expr(self) -> Node:
        node = self.parse_primary()
        while True:
            if self.match("."):
                if self.match("PARENT"):
                    if isinstance(node, Parent):
                        node = Parent(node.depth + 1)
                    else:
                        raise ParseError(
                            "expected ^ accesses to be chained with each other"
                        )
                    continue
                if self.peek().typ == "IDENT":
                    attr = self.expect("IDENT").val
                elif self.peek().typ == "DOLLAR":
                    attr = self.expect("DOLLAR").val
                else:
                    raise ParseError("property name expected after '.'")
                node = Access(node, attr)
                continue
            if self.peek().typ == "[":
                self.expect("[")
                defs = self.parse_def_list_until({"]"})
                self.expect("]")
                node = Override(node, defs)
                continue
            break
        return node

    def parse_primary(self) -> Node:
        t = self.peek()
        if t.typ == "{":
            self.expect("{")
            defs = self.parse_def_list_until({"}"})
            self.expect("}")
            return Block(defs)
        if t.typ == "INT":
            return IntLit(int(self.expect("INT").val))
        if t.typ == "STRING":
            return StringLit(self.expect("STRING").val)
        if t.typ == "SELF":
            self.expect("SELF")
            return SelfRef()
        if t.typ == "PARENT":
            self.expect("PARENT")
            return Parent(1)
        if t.typ == "IDENT":
            return Identifier(self.expect("IDENT").val)
        if t.typ == "DOLLAR":
            self.expect("DOLLAR")
            return Identifier("$")
        raise ParseError(f"Unexpected token {t.typ}")


def parse_module(code: str):
    toks = tokenize(code)
    return Parser(toks).parse_module()
