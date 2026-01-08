from dataclasses import dataclass

from lexer import tokenize
from nodes import (
    Access,
    AncestorLookup,
    BinaryOp,
    Block,
    Call,
    CloneAttr,
    Conditional,
    Eager,
    Identifier,
    IntLit,
    Node,
    Override,
    Parent,
    SelfRef,
    StringLit,
)


@dataclass
class Definition:
    name: str
    expr: Node


class ParseError(Exception):
    pass


# Precedence for binary arithmetic operators (higher = tighter binding)
BinaryOpPrecedence = {
    "||": 3,
    "&&": 4,
    "==": 5,
    "!=": 5,
    "<=": 7,
    ">=": 7,
    "<": 7,
    ">": 7,
    "+": 10,
    "-": 10,
    "*": 20,
    "/": 20,
    "%": 20,
}


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
        while self.peek().typ in {","}:
            self.k += 1

    def parse_module(self) -> Block:
        defs = self.parse_defs_until({"EOF"})
        self.expect("EOF")
        return Block(defs)

    def parse_definition(self) -> Definition:
        name = self.expect("IDENT").val
        eq = self.expect("=", ":=", "<-")
        if eq.typ == "<-":
            attr = self.expect("IDENT").val
            return Definition(name, CloneAttr(attr=attr))
        expr = self.parse_expr()
        return Definition(name, expr if eq.typ == "=" else Eager(base=expr))

    def parse_defs_until(self, stop, allow_eager: bool = False) -> dict[str, Node]:
        defs = {}
        self.consume_delims()
        while self.peek().typ not in stop:
            start = self.peek_position()
            d = self.parse_definition()
            if d.name in defs:
                raise ParseError(f"Redefinition of {d.name!r} at {start}")
            elif not allow_eager and isinstance(d.expr, Eager):
                raise ParseError(
                    f"Eager definition is not allowed in this context at {start}"
                )
            defs[d.name] = d.expr
            self.consume_delims()
        return defs

    def parse_expr(self) -> Node:
        return self.parse_unary_or_ternary()

    def parse_unary_or_ternary(self) -> Node:
        """
        Parse ternary conditional: cond ? a : b
        `?:` has lower precedence than arithmetic and postfix.
        """
        node = self.parse_binary(0)
        if self.match("?"):
            true_expr = self.parse_expr()
            self.expect(":")
            false_expr = self.parse_expr()
            return Conditional(cond=node, a=true_expr, b=false_expr)
        return node

    def parse_binary(self, min_prec: int) -> Node:
        """
        Precedence-climbing parser for left-associative binary ops
        using BinaryOpPrecedence.
        """
        node = self.parse_postfix()
        while True:
            tok = self.peek()
            op = tok.typ
            if op not in BinaryOpPrecedence:
                break

            prec = BinaryOpPrecedence[op]
            if prec < min_prec:
                break

            # consume operator
            self.k += 1
            # left-associative: recurse with higher min_prec
            rhs = self.parse_binary(prec + 1)
            node = BinaryOp(x=node, op=op, y=rhs)
        return node

    def parse_postfix(self) -> Node:
        """
        Parse primary expression followed by:
        - .attr / .PARENT chains
        - UNWRAP
        - [ overrides ]
        - ( call(...) )
        """
        node = self.parse_primary()
        while True:
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
                defs = self.parse_defs_until({")"}, allow_eager=True)
                self.expect(")")
                node = Call(node, defs)
            else:
                break
        return node

    def parse_primary(self) -> Node:
        start_pos = self.peek_position()
        token = self.expect(
            "{", "INT", "STRING", "SELF", "PARENT", "ANCESTOR", "IDENT", "("
        )
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
        elif token.typ == "(":
            # Parenthesized expression: (expr)
            expr = self.parse_expr()
            self.expect(")")
            return expr
        raise ParseError(f"Unexpected token {token.typ} at {start_pos}")


def parse_module(code: str):
    toks = tokenize(code)
    return Parser(toks).parse_module()
