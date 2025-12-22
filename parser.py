from dataclasses import dataclass, field
from lexer import tokenize


class Node:
    pass


@dataclass
class Block(Node):
    defs: dict[str, Node]
    cache: dict = field(default_factory=dict)


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
class AncestorLookup(Node):
    name: str


@dataclass
class Access(Node):
    base: Node
    attr: str


@dataclass
class Override(Node):
    base: Node
    defs: dict[str, Node]


@dataclass
class Call(Node):
    base: Node
    defs: dict[str, Node]


@dataclass
class Definition:
    name: str
    expr: Node


class ParseError(Exception):
    pass


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

    def expect(self, *types):
        tok = self.match(*types)
        if not tok:
            raise ParseError(
                f"Expected {types}, got {self.peek().typ} at {self.peek_position()}"
            )
        return tok

    def peek_position(self):
        return f"{self.peek().line}:{self.peek().col}"

    def consume_delims(self):
        while self.peek().typ in {",", "NEWLINE"}:
            self.k += 1

    def parse_module(self) -> Block:
        defs = self.parse_defs_until({"EOF"})
        self.expect("EOF")
        return Block(defs)

    def parse_definition(self) -> Definition:
        name = self.expect("IDENT").val
        self.expect("=")
        expr = self.parse_expr()
        return Definition(name, expr)

    def parse_defs_until(self, stop) -> dict[str, Node]:
        defs = {}
        self.consume_delims()
        while self.peek().typ not in stop:
            start = self.peek_position()
            d = self.parse_definition()
            if d.name in defs:
                raise ParseError(f"Redefinition of {d.name!r} at {start}")
            defs[d.name] = d.expr
            self.consume_delims()
        return defs

    def parse_expr(self) -> Node:
        node = self.parse_primary()
        while True:
            start = self.peek_position()
            if self.match("."):
                if self.match("PARENT"):
                    if isinstance(node, Parent):
                        node = Parent(node.depth + 1)
                    else:
                        raise ParseError("Unexpected ^ operator at {start}")
                    continue
                attr = self.expect("IDENT").val
                node = Access(node, attr)
            elif self.match("UNWRAP"):
                node = Access(node, "result")
            elif self.peek().typ == "[":
                self.expect("[")
                defs = self.parse_defs_until({"]"})
                self.expect("]")
                node = Override(node, defs)
            elif self.peek().typ == "(":
                self.expect("(")
                defs = self.parse_defs_until({")"})
                self.expect(")")
                node = Call(node, defs)
            else:
                break
        return node

    def parse_primary(self) -> Node:
        start_pos = self.peek_position()
        token = self.expect("{", "INT", "STRING", "SELF", "PARENT", "ANCESTOR", "IDENT")
        if token.typ == "{":
            defs = self.parse_defs_until({"}"})
            self.expect("}")
            return Block(defs)
        elif token.typ == "INT":
            return IntLit(int(token.val))
        elif token.typ == "STRING":
            return StringLit(token.val)
        elif token.typ == "SELF":
            return SelfRef()
        elif token.typ == "PARENT":
            return Parent(1)
        elif token.typ == "ANCESTOR":
            self.expect(".")
            name = self.expect("IDENT").val
            return AncestorLookup(name)
        elif token.typ == "IDENT":
            return Identifier(token.val)
        raise ParseError(f"Unexpected token {token.typ} at {start_pos}")


def parse_module(code: str):
    toks = tokenize(code)
    return Parser(toks).parse_module()
