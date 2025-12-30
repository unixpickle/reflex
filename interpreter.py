from dataclasses import dataclass
from functools import partial
from typing import Callable

from parser import (
    Access,
    AncestorLookup,
    Block,
    Call,
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
class BackEdge(Node):
    base: Block | Override


@dataclass
class BuiltInFn(Node):
    context: BackEdge

    def map(self, f: Callable[[Node], Node]) -> "BuiltInFn":
        return type(self)(context=f(self.context))


@dataclass
class IntOp(BuiltInFn):
    fn: Callable[[int, int], int]

    def map(self, f: Callable[[Node], Node]) -> "IntOp":
        return IntOp(context=f(self.context), fn=self.fn)


@dataclass
class IntStr(BuiltInFn):
    pass


@dataclass
class IntChr(BuiltInFn):
    pass


@dataclass
class Select(BuiltInFn):
    pass


def int_op_block(parent: Block, fn: Callable[[int, int], int]) -> Block:
    result = Block(defs=[])
    result.defs = dict(
        x=BackEdge(base=parent),
        result=IntOp(context=BackEdge(base=result), fn=fn),
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
            str=IntStr(context=BackEdge(base=result)),
            chr=IntChr(context=BackEdge(base=result)),
            select=select_block,
        )
    )
    return result


@dataclass
class StrCat(BuiltInFn):
    pass


@dataclass
class StrEq(BuiltInFn):
    pass


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
            eq=str_method(
                result,
                lambda method_block: dict(
                    result=StrEq(context=BackEdge(base=method_block)),
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
    return result


def block_get(block: Block | Override, key: str) -> Node:
    assert isinstance(
        block, (Block, Override)
    ), f"cannot get key {key!r} of node type {type(block)}"
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
    elif isinstance(expr, IntLit):
        return int_to_block(expr)
    elif isinstance(expr, StringLit):
        return str_to_block(expr)
    elif isinstance(expr, Identifier):
        return preprocess(parents, Access(base=SelfRef(), attr=expr.name))
    elif isinstance(expr, Eager):
        return Eager(base=preprocess(parents, expr.base))
    else:
        raise ValueError(f"type {type(expr)} cannot fill parents")


def _clone_blocks(node: Node, result: dict[int, Node]) -> Node:
    """
    Walk the tree, not following back edges, and deep-copy any blocks
    which are found.
    """
    if isinstance(node, Access):
        return Access(base=_clone_blocks(node.base, result), attr=node.attr)
    elif isinstance(node, Block):
        if id(node) not in result:
            result[id(node)] = Block(defs={})
            result[id(node)].defs = {
                k: _clone_blocks(v, result) for k, v in node.defs.items()
            }
        return result[id(node)]
    elif isinstance(node, Override):
        if id(node) not in result:
            result[id(node)] = Override(base=None, defs={})
            result[id(node)].base = _clone_blocks(node.base, result)
            result[id(node)].defs = {
                k: _clone_blocks(v, result) for k, v in node.defs.items()
            }
        return result[id(node)]
    elif isinstance(node, Call):
        return Call(
            base=_clone_blocks(node.base, result),
            defs={k: _clone_blocks(v, result) for k, v in node.defs.items()},
        )
    elif isinstance(node, BackEdge):
        # Copy the object so we can update the base later.
        return BackEdge(base=node.base)
    elif isinstance(node, Eager):
        return Eager(base=_clone_blocks(node.base, result))
    elif isinstance(node, BuiltInFn):
        return node.map(lambda x: _clone_blocks(x, result))
    elif isinstance(node, (IntLit, StringLit)):
        return node
    else:
        raise ValueError(f"cannot clone block: {type(node)}")


def _replace_back_edges(node: Node, mapping: dict[int, Node]) -> Node:
    if isinstance(node, Access):
        _replace_back_edges(node.base, mapping)
    elif isinstance(node, Block):
        for d in node.defs.values():
            _replace_back_edges(d, mapping)
    elif isinstance(node, (Call, Override)):
        _replace_back_edges(node.base, mapping)
        for d in node.defs.values():
            _replace_back_edges(d, mapping)
    elif isinstance(node, BackEdge):
        if id(node.base) in mapping:
            node.base = mapping[id(node.base)]
    elif isinstance(node, Eager):
        _replace_back_edges(node.base, mapping)
    elif isinstance(node, BuiltInFn):
        node.map(lambda x: _replace_back_edges(x, mapping))
    elif isinstance(node, (IntLit, StringLit)):
        pass
    else:
        raise ValueError(f"cannot clone block: {type(node)}")


def clone_tree(node: Node) -> Node:
    copies = dict()
    new_node = _clone_blocks(node, copies)
    _replace_back_edges(new_node, copies)
    return new_node


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
        new_block = clone_tree(base_block)
        new_block.defs = new_block.defs.copy()
        for k, v in override.defs.items():
            new_def = clone_tree(v)
            _replace_back_edges(new_def, {id(override): new_block})
            new_block.defs[k] = new_def
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

    def _streq_after_x(value: Node, *, expr: Node):
        x = value
        assert isinstance(x, StringLit), f"{x=}"
        stack.append(partial(_streq_apply, x=x))
        expr = Access(
            base=Access(base=expr.context, attr="y"),
            attr="_inner",
        )
        return expr, None

    def _streq_apply(value: Node, *, x: Node):
        y = value
        assert isinstance(y, StringLit), f"{y=}"
        value = int_to_block(IntLit(value=int(x.value == y.value)))
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

    def _eager_eval(value: Node, *, block: Node, attr: str):
        block.defs[attr] = value
        return block, None

    while True:
        assert not isinstance(
            expr, (Identifier, SelfRef)
        ), f"block type should not exist after preprocessing: {type(expr)}"

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
        elif isinstance(expr, StrEq):
            stack.append(partial(_streq_after_x, expr=expr))
            expr = Access(base=Access(base=expr.context, attr="x"), attr="_inner")
        elif isinstance(expr, StrLen):
            stack.append(_strlen_after_x)
            expr = Access(base=Access(base=expr.context, attr="x"), attr="_inner")
        elif isinstance(expr, StrSubstr):
            stack.append(partial(_strsubstr_after_x, expr=expr))
            expr = Access(base=Access(base=expr.context, attr="x"), attr="_inner")
        elif isinstance(expr, Block) and (attr := first_eager_key(expr)):
            stack.append(partial(_eager_eval, block=expr, attr=attr))
            expr = block_get(expr, attr)
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


def first_eager_key(block: Block) -> str | None:
    for k, v in block.defs.items():
        if isinstance(v, Eager):
            return k
    return None


def program_result(program: Block) -> Node:
    program = preprocess([], program)
    return evaluate_result(block_get(program, "result"))
