from dataclasses import dataclass
from functools import partial
from typing import Callable, Literal, Type
from weakref import WeakKeyDictionary

from nodes import (
    Access,
    AncestorLookup,
    BackEdge,
    BinaryOp,
    Block,
    BuiltInFn,
    Call,
    CloneAttr,
    Conditional,
    Eager,
    Identifier,
    IntLit,
    IntOp,
    Node,
    Override,
    Parent,
    SelfRef,
    StringLit,
)


@dataclass
class IntStr(BuiltInFn):
    pass


@dataclass
class IntChr(BuiltInFn):
    pass


@dataclass
class Select(BuiltInFn):
    pass


@dataclass
class IntLogicalAnd(BuiltInFn):
    pass


@dataclass
class IntLogicalOr(BuiltInFn):
    pass


def int_op_block(parent: Block, fn: Callable[[int, int], int]) -> Block:
    result = Block(defs=[])
    result.defs = dict(
        x=BackEdge(base=parent),
        result=IntOp(context=BackEdge(base=result), fn=fn),
    )
    return result


def int_logical_op_block(
    parent: Block, op: Type[IntLogicalAnd] | Type[IntLogicalOr]
) -> Block:
    result = Block(defs=[])
    result.defs = dict(
        x=BackEdge(base=parent),
        result=op(context=BackEdge(base=result)),
    )
    return result


def int_to_block(node: IntLit) -> Block:
    assert isinstance(node, IntLit)
    result = Block(defs=dict(_inner=node))
    select_block = Block(defs=[])
    select_block.defs = dict(
        cond=BackEdge(base=result),
        result=Select(context=BackEdge(base=select_block)),
    )
    result.defs.update(
        dict(
            add=int_op_block(result, lambda x, y: x + y),
            sub=int_op_block(result, lambda x, y: x - y),
            eq=int_op_block(result, lambda x, y: int(x == y)),
            ne=int_op_block(result, lambda x, y: int(x != y)),
            lt=int_op_block(result, lambda x, y: int(x < y)),
            gt=int_op_block(result, lambda x, y: int(x > y)),
            le=int_op_block(result, lambda x, y: int(x <= y)),
            ge=int_op_block(result, lambda x, y: int(x >= y)),
            mul=int_op_block(result, lambda x, y: x * y),
            div=int_op_block(result, lambda x, y: x // y),
            mod=int_op_block(result, lambda x, y: x % y),
            bit_and=int_op_block(result, lambda x, y: x & y),
            bit_or=int_op_block(result, lambda x, y: x | y),
            bit_xor=int_op_block(result, lambda x, y: x ^ y),
            logical_and=int_logical_op_block(result, IntLogicalAnd),
            logical_or=int_logical_op_block(result, IntLogicalOr),
            str=IntStr(context=BackEdge(base=result)),
            chr=IntChr(context=BackEdge(base=result)),
            select=select_block,
        )
    )
    return result


@dataclass
class StrCat(BuiltInFn):
    pass


ComparisonOp = Literal["==", "!=", "<", ">", "<=", ">="]


@dataclass
class StrComparison(BuiltInFn):
    op: ComparisonOp

    def lazy_clone(
        self, overrides: WeakKeyDictionary[Node, Node] | None = None
    ) -> Node:
        result = type(self)(context=self.context, op=self.op)
        result.clone_overrides = (overrides or WeakKeyDictionary()) | (
            self.clone_overrides or WeakKeyDictionary()
        )
        return result


@dataclass
class StrLen(BuiltInFn):
    pass


@dataclass
class StrSubstr(BuiltInFn):
    pass


def str_method(str_block: Block, fn: Callable[[Block], dict[str, Block]]) -> Block:
    block = Block(defs=dict(x=BackEdge(base=str_block)))
    block.defs.update(fn(block))
    return block


def str_to_block(node: StringLit) -> Block:
    assert isinstance(node, StringLit)
    result = Block(defs=dict(_inner=node))
    result.defs.update(
        dict(
            cat=str_method(
                result,
                lambda method_block: dict(
                    result=StrCat(context=BackEdge(base=method_block)),
                ),
            ),
            add=str_method(
                result,
                lambda method_block: dict(
                    result=StrCat(context=BackEdge(base=method_block)),
                ),
            ),
            len=StrLen(context=BackEdge(base=result)),
            substr=str_method(
                result,
                lambda method_block: dict(
                    start=int_to_block(IntLit(value=0)),
                    end=StrLen(context=BackEdge(base=result)),
                    result=StrSubstr(context=BackEdge(base=method_block)),
                ),
            ),
        )
    )
    for op, name in [
        ("<=", "le"),
        (">=", "ge"),
        ("==", "eq"),
        ("!=", "ne"),
        ("<", "lt"),
        (">", "gt"),
    ]:
        result.defs[name] = str_method(
            result,
            lambda method_block: dict(
                result=StrComparison(context=BackEdge(base=method_block), op=op),
            ),
        )
    return result


def block_get(block: Block | Override, key: str) -> Node:
    assert block.clone_overrides is None, (
        "cannot access a block before its clone is propagated"
    )
    assert isinstance(block, (Block, Override)), (
        f"cannot get key {key!r} of node type {type(block)}"
    )
    if key in block.defs:
        return block.defs[key]
    raise KeyError(f"object has no key {key!r}, keys are {list(block.defs.keys())!r}")


def preprocess(parents: list[Block | Override], expr: Node) -> Node:
    """
    Replace Parent with BackEdge and wrap literals in blocks.
    """
    if isinstance(expr, Parent):
        assert expr.depth + 1 <= len(parents)
        return BackEdge(base=parents[len(parents) - (expr.depth + 1)])
    elif isinstance(expr, SelfRef):
        return BackEdge(base=parents[-1])
    elif isinstance(expr, Access):
        return Access(base=preprocess(parents, expr.base), attr=expr.attr)
    elif isinstance(expr, AncestorLookup):
        found_parent = None
        for parent in parents[:-1][::-1]:
            assert isinstance(parent, (Override, Block))
            try:
                block_get(parent, expr.name)
                found_parent = parent
            except KeyError:
                continue
        if found_parent is None:
            raise KeyError(f"ancestor lookup failed for key {expr.name!r}")
        return Access(base=BackEdge(base=found_parent), attr=expr.name)
    elif isinstance(expr, Block):
        b = Block(defs={})
        b.defs = {k: None for k in expr.defs.keys()}  # temporary for ancestor lookup
        b.defs = {k: preprocess(parents + [b], v) for k, v in expr.defs.items()}
        return b
    elif isinstance(expr, Override):
        o = Override(base=None, defs={})
        o.base = preprocess(parents, expr.base)
        o.defs = {k: None for k in expr.defs.keys()}  # temporary for ancestor lookup
        o.defs = {k: preprocess(parents + [o], v) for k, v in expr.defs.items()}
        return o
    elif isinstance(expr, Call):
        return Call(
            base=preprocess(parents, expr.base),
            defs={k: preprocess(parents, v) for k, v in expr.defs.items()},
        )
    elif isinstance(expr, BinaryOp):
        fn_name = {
            "==": "eq",
            "!=": "ne",
            "<": "lt",
            ">": "gt",
            ">=": "ge",
            "<=": "le",
            "+": "add",
            "-": "sub",
            "/": "div",
            "*": "mul",
            "%": "mod",
            "&&": "logical_and",
            "||": "logical_or",
        }
        new_expr = Access(
            base=Call(
                base=Access(base=expr.x, attr=fn_name[expr.op]), defs=dict(y=expr.y)
            ),
            attr="result",
        )
        return preprocess(parents, new_expr)
    elif isinstance(expr, Conditional):
        new_expr = Access(
            base=Call(
                base=Access(base=expr.cond, attr="select"),
                defs=dict(true=expr.a, false=expr.b),
            ),
            attr="result",
        )
        return preprocess(parents, new_expr)
    elif isinstance(expr, IntLit):
        return int_to_block(expr)
    elif isinstance(expr, StringLit):
        return str_to_block(expr)
    elif isinstance(expr, Identifier):
        return preprocess(parents, Access(base=SelfRef(), attr=expr.name))
    elif isinstance(expr, Eager):
        return Eager(base=preprocess(parents, expr.base))
    elif isinstance(expr, CloneAttr):
        return expr
    else:
        raise ValueError(f"type {type(expr)} cannot fill parents")


def evaluate_result(expr: Node) -> Node:
    """
    Non-recursive evaluator using an explicit continuation stack.
    """

    # This is a stack of continuations.
    # Each continuation can either yield a value for the next continuation on
    # the stack, or yield an expression for the next iteration of the outer loop.
    #
    # In particular, the call returns (expr, value) where exactly one is None.
    # If the return is value, then we continue the inner loop; for expr, we break
    # and start a new outer loop with the new expr.
    stack: list[Callable[[Node], tuple[Node | None, Node | None]]] = []

    def _access_after_base(value: Node, *, attr: str):
        owner = value
        expr = block_get(owner, attr)
        return expr, None

    def _override_after_base(value: Node, *, override: Call | Override):
        base_block = value
        new_block = base_block.lazy_clone()

        # We want to replace the base_block with new_block in all of the
        # existing definitions, but not the new definitions.
        new_block.propagate_clone()

        for k, v in override.defs.items():
            new_block.defs[k] = v.lazy_clone(
                overrides=WeakKeyDictionary({override: new_block})
            )

        # Return the block as an expr to evaluate any eager fields before
        # continuing.
        return new_block, None

    def _intop_after_x(value: Node, *, expr: Node):
        x = value
        assert isinstance(x, IntLit), f"{x=}"
        stack.append(partial(_intop_apply, expr=expr, x=x))
        expr = Access(
            base=Access(base=expr.context, attr="y"),
            attr="_inner",
        )
        return expr, None

    def _intop_apply(value: Node, *, expr: Node, x: Node):
        y = value
        assert isinstance(y, IntLit), f"{y=}"
        value = int_to_block(IntLit(value=expr.fn(x.value, y.value)))
        return None, value

    def _logical_int_op_after_x(value: Node, *, expr: Node, is_or: bool):
        x = value
        assert isinstance(x, IntLit), f"{x=}"
        if (is_or and x.value) or (not is_or and not x.value):
            return None, int_to_block(x)
        expr = Access(base=expr.context, attr="y")
        return expr, None

    def _select_after_cond(value: Node, *, expr: Node):
        cond = value
        assert isinstance(cond, IntLit), f"{cond=}"
        expr = Access(
            base=expr.context,
            attr=("true" if cond.value else "false"),
        )
        return expr, None

    def _intstr_apply(value: Node):
        x = value
        assert isinstance(x, IntLit), f"{x=}"
        value = str_to_block(StringLit(value=str(x.value)))
        return None, value

    def _intchr_apply(value: Node):
        x = value
        assert isinstance(x, IntLit), f"{x=}"
        value = str_to_block(StringLit(value=chr(x.value)))
        return None, value

    def _strcat_after_x(value: Node, *, expr: Node):
        x = value
        assert isinstance(x, StringLit), f"{x=}"
        stack.append(partial(_strcat_apply, x=x))
        expr = Access(
            base=Access(base=expr.context, attr="y"),
            attr="_inner",
        )
        return expr, None

    def _strcat_apply(value: Node, *, x: Node):
        y = value
        assert isinstance(y, StringLit), f"{y=}"
        value = str_to_block(StringLit(value=x.value + y.value))
        return None, value

    def _strcmp_after_x(value: Node, *, expr: Node, op: ComparisonOp):
        x = value
        assert isinstance(x, StringLit), f"{x=}"
        stack.append(partial(_strcmp_apply, x=x, op=op))
        expr = Access(
            base=Access(base=expr.context, attr="y"),
            attr="_inner",
        )
        return expr, None

    def _strcmp_apply(value: Node, *, x: Node, op: ComparisonOp):
        y = value
        assert isinstance(y, StringLit), f"{y=}"
        if op == "==":
            result = x.value == y.value
        elif op == "<":
            result = x.value < y.value
        elif op == "<=":
            result = x.value <= y.value
        elif op == ">":
            result = x.value > y.value
        elif op == ">=":
            result = x.value >= y.value
        elif op == "!=":
            result = x.value != y.value
        else:
            raise ValueError(f"Unsupported comparison operator: {op}")
        value = int_to_block(IntLit(value=int(result)))
        return None, value

    def _strlen_after_x(value: Node):
        x = value
        assert isinstance(x, StringLit), f"{x=}"
        value = int_to_block(IntLit(value=len(x.value)))
        return None, value

    def _strsubstr_after_x(value: Node, *, expr: Node):
        x = value
        assert isinstance(x, StringLit), f"{x=}"
        stack.append(partial(_strsubstr_after_start, x=x, expr=expr))
        expr = Access(
            base=Access(base=expr.context, attr="start"),
            attr="_inner",
        )
        return expr, None

    def _strsubstr_after_start(value: Node, *, x: Node, expr: Node):
        start = value
        assert isinstance(start, IntLit), f"{start=}"
        stack.append(partial(_strsubstr_apply, x=x, start=start))
        expr = Access(
            base=Access(base=expr.context, attr="end"),
            attr="_inner",
        )
        return expr, None

    def _strsubstr_apply(value: Node, *, x: Node, start: Node):
        end = value
        assert isinstance(x, StringLit), f"{x=}"
        assert isinstance(start, IntLit), f"{start=}"
        assert isinstance(end, IntLit), f"{end=}"
        substr = x.value[start.value : end.value]
        value = str_to_block(StringLit(value=substr))
        return None, value

    def _eager_eval(value: Node, *, block: Node, attrs: list[str]):
        block.defs[attrs[0]] = value.lazy_clone()
        if len(attrs) == 1:
            return block, None
        stack.append(partial(_eager_eval, block=block, attrs=attrs[1:]))
        return None, block_get(block, attrs[1])

    while True:
        assert not isinstance(expr, (Identifier, SelfRef)), (
            f"block type should not exist after preprocessing: {type(expr)}"
        )
        expr.propagate_clone()

        if isinstance(expr, Access):
            stack.append(partial(_access_after_base, attr=expr.attr))
            expr = expr.base
        elif isinstance(expr, Eager):
            expr = expr.base
        elif isinstance(expr, (Call, Override)):
            stack.append(partial(_override_after_base, override=expr))
            expr = expr.base
        elif isinstance(expr, BackEdge):
            expr = expr.base
        elif isinstance(expr, IntOp):
            stack.append(partial(_intop_after_x, expr=expr))
            expr = Access(base=Access(base=expr.context, attr="x"), attr="_inner")
        elif isinstance(expr, (IntLogicalOr, IntLogicalAnd)):
            stack.append(
                partial(
                    _logical_int_op_after_x,
                    expr=expr,
                    is_or=isinstance(expr, IntLogicalOr),
                )
            )
            expr = Access(base=Access(base=expr.context, attr="x"), attr="_inner")
        elif isinstance(expr, Select):
            stack.append(partial(_select_after_cond, expr=expr))
            expr = Access(base=Access(base=expr.context, attr="cond"), attr="_inner")
        elif isinstance(expr, IntStr):
            stack.append(_intstr_apply)
            expr = Access(base=expr.context, attr="_inner")
        elif isinstance(expr, IntChr):
            stack.append(_intchr_apply)
            expr = Access(base=expr.context, attr="_inner")
        elif isinstance(expr, StrCat):
            stack.append(partial(_strcat_after_x, expr=expr))
            expr = Access(base=Access(base=expr.context, attr="x"), attr="_inner")
        elif isinstance(expr, StrComparison):
            stack.append(partial(_strcmp_after_x, expr=expr, op=expr.op))
            expr = Access(base=Access(base=expr.context, attr="x"), attr="_inner")
        elif isinstance(expr, StrLen):
            stack.append(_strlen_after_x)
            expr = Access(base=Access(base=expr.context, attr="x"), attr="_inner")
        elif isinstance(expr, StrSubstr):
            stack.append(partial(_strsubstr_after_x, expr=expr))
            expr = Access(base=Access(base=expr.context, attr="x"), attr="_inner")
        elif isinstance(expr, Block) and (attrs := attr_clones(expr)):
            expr = expr.lazy_clone()
            for target, source in attrs:
                expr.defs[target] = expr.defs[source]
        elif isinstance(expr, Block) and (attrs := eager_keys(expr)):
            expr = expr.lazy_clone()
            expr.propagate_clone()
            stack.append(partial(_eager_eval, block=expr, attrs=attrs))
            expr = block_get(expr, attrs[0])
        else:
            # Apply continuation(s) because we have reduced to a leaf value
            # like IntLit that evaluates to itself.
            value = expr
            while True:
                if not stack:
                    return value
                fn = stack.pop()
                result = fn(value)
                if result[0] is not None:
                    expr = result[0]
                    break
                else:
                    value = result[1]
                    value.propagate_clone()


def eager_keys(block: Block) -> str | None:
    result = []
    for k, v in block.defs.items():
        if isinstance(v, Eager):
            result.append(k)
    return result


def attr_clones(block: Block) -> list[(str, str)]:
    result = []
    for k, v in block.defs.items():
        if isinstance(v, CloneAttr):
            result.append((k, v.attr))
    return result


def program_result(program: Block) -> Node:
    program = preprocess([], program)
    return evaluate_result(block_get(program, "result"))
