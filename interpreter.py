from dataclasses import dataclass
from typing import Callable

from parser import (
    Access,
    AncestorLookup,
    Block,
    Definition,
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


class BuiltInFn(Node):
    pass


@dataclass
class IntOp(BuiltInFn):
    fn: Callable[[int, int], int]


@dataclass
class IntStr(BuiltInFn):
    pass


@dataclass
class Select(BuiltInFn):
    pass


def int_op_block(parent: Block, fn: Callable[[int, int], int]) -> Block:
    return Block(
        defs=[
            Definition(name="x", expr=BackEdge(base=parent)),
            Definition(name="result", expr=IntOp(fn=fn)),
        ]
    )


def int_to_block(node: IntLit) -> Block:
    assert isinstance(node, IntLit)
    result = Block(
        defs=[
            Definition(name="_inner", expr=node),
        ]
    )
    result.defs.append(
        Definition(name="add", expr=int_op_block(result, lambda x, y: x + y))
    )
    result.defs.append(
        Definition(name="eq", expr=int_op_block(result, lambda x, y: int(x == y)))
    )
    result.defs.append(
        Definition(name="mul", expr=int_op_block(result, lambda x, y: x * y))
    )
    result.defs.append(
        Definition(name="div", expr=int_op_block(result, lambda x, y: x // y))
    )
    result.defs.append(
        Definition(name="mod", expr=int_op_block(result, lambda x, y: x % y))
    )
    result.defs.append(Definition(name="str", expr=IntStr()))
    result.defs.append(
        Definition(
            name="select",
            expr=Block(
                defs=[
                    Definition(name="cond", expr=BackEdge(base=result)),
                    Definition(name="result", expr=Select()),
                ]
            ),
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


def str_to_block(node: StringLit) -> Block:
    assert isinstance(node, StringLit)
    result = Block(
        defs=[
            Definition(name="_inner", expr=node),
        ]
    )
    result.defs.extend(
        [
            Definition(
                name="cat",
                expr=Block(
                    defs=[
                        Definition(name="x", expr=BackEdge(base=result)),
                        Definition(name="result", expr=StrCat()),
                    ]
                ),
            ),
            Definition(
                name="eq",
                expr=Block(
                    defs=[
                        Definition(name="x", expr=BackEdge(base=result)),
                        Definition(name="result", expr=StrEq()),
                    ]
                ),
            ),
            Definition(
                name="len",
                expr=Block(
                    defs=[
                        Definition(name="x", expr=BackEdge(base=result)),
                        Definition(name="result", expr=StrLen()),
                    ]
                ),
            ),
        ]
    )
    return result


def block_get(block: Block | Override, key: str) -> Node:
    """
    Get a key, prioritizing earlier definitions over later ones.
    Iterative (non-recursive), so deep override chains don't hit recursion limits.
    """
    assert isinstance(block, (Block, Override)), (
        f"cannot get key {key!r} of node type {type(block)}"
    )

    for d in block.defs:
        if key == d.name:
            return d.expr

    raise KeyError(
        f"object has no key {key!r}, keys are {[d.name for d in block.defs]}"
    )


def preprocess(parents: list[Block | Override], expr: Node) -> Node:
    """
    Replace Parent with BackEdge and wrap int literals in blocks.
    """
    if isinstance(expr, Parent):
        assert expr.depth + 1 <= len(parents)
        return BackEdge(base=parents[len(parents) - (expr.depth + 1)])
    elif isinstance(expr, Access):
        return Access(base=preprocess(parents, expr.base), attr=expr.attr)
    elif isinstance(expr, AncestorLookup):
        found_parent = None
        for parent in parents[::-1]:
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
        b = Block(defs=[])
        b.defs = [
            Definition(name=d.name, expr=None) for d in expr.defs
        ]  # temporary, so that ancestor lookup works.
        b.defs = [preprocess(parents + [b], d) for d in expr.defs]
        return b
    elif isinstance(expr, Override):
        o = Override(base=None, defs=[])
        o.base = preprocess(parents, expr.base)
        o.defs = [
            Definition(name=d.name, expr=None) for d in expr.defs
        ]  # temporary, so that ancestor lookup works.
        o.defs = [preprocess(parents + [o], d) for d in expr.defs]
        return o
    elif isinstance(expr, Definition):
        return Definition(name=expr.name, expr=preprocess(parents, expr.expr))
    elif isinstance(expr, IntLit):
        return int_to_block(expr)
    elif isinstance(expr, StringLit):
        return str_to_block(expr)
    elif isinstance(expr, (Identifier, SelfRef)):
        return expr
    else:
        raise ValueError(f"type {type(expr)} cannot fill parents")


def _clone_blocks(node: Node, result: dict[int, Node]) -> Node:
    """
    Walk the tree, not following back edges, and deep-copy any blocks
    which are found.
    """
    if isinstance(node, Definition):
        return Definition(name=node.name, expr=_clone_blocks(node.expr, result))
    elif isinstance(node, Access):
        return Access(base=_clone_blocks(node.base, result), attr=node.attr)
    elif isinstance(node, Block):
        assert id(node) not in result, "somehow found a circular reference"
        result[id(node)] = Block(defs=[])
        result[id(node)].defs = [_clone_blocks(block, result) for block in node.defs]
        return result[id(node)]
    elif isinstance(node, Override):
        assert id(node) not in result, "somehow found a circular reference"
        result[id(node)] = Override(base=None, defs=[])
        result[id(node)].base = _clone_blocks(node.base, result)
        result[id(node)].defs = [_clone_blocks(d, result) for d in node.defs]
        return result[id(node)]
    elif isinstance(node, BackEdge):
        # Copy the object so we can update the base later.
        return BackEdge(base=node.base)
    elif isinstance(node, (SelfRef, IntLit, StringLit, Identifier, BuiltInFn)):
        return node
    else:
        raise ValueError(f"cannot clone block: {type(node)}")


def _replace_back_edges(node: Node, mapping: dict[int, Node]) -> Node:
    if isinstance(node, Definition):
        _replace_back_edges(node.expr, mapping)
    elif isinstance(node, Access):
        _replace_back_edges(node.base, mapping)
    elif isinstance(node, Block):
        for d in node.defs:
            _replace_back_edges(d, mapping)
    elif isinstance(node, Override):
        _replace_back_edges(node.base, mapping)
        for d in node.defs:
            _replace_back_edges(d, mapping)
    elif isinstance(node, BackEdge):
        if id(node.base) in mapping:
            node.base = mapping[id(node.base)]
    elif isinstance(node, (SelfRef, IntLit, StringLit, Identifier, BuiltInFn)):
        pass
    else:
        raise ValueError(f"cannot clone block: {type(node)}")


def clone_tree(node: Node) -> Node:
    copies = dict()
    new_node = _clone_blocks(node, copies)
    _replace_back_edges(new_node, copies)
    return new_node


def evaluate_result(context: Block | Override, expr: Node) -> Node:
    """
    Non-recursive evaluator using an explicit continuation stack.

    The stack holds small 'frames' (tagged tuples) that describe what to do
    with the result of the current sub-evaluation. This mirrors the call stack
    of the original recursive implementation while avoiding Python recursion.
    """
    stack: list[tuple] = []

    while True:
        # ---- Descend (choose what to evaluate next) ----
        if isinstance(expr, Access):
            # Evaluate base first; after that, look up attr in the owner,
            # evaluate it in the owner context, then restore the previous context.
            stack.append(("access_after_base", expr.attr, context))
            expr = expr.base
            continue

        if isinstance(expr, Override):
            # Evaluate base; then build a new block with defs prepended.
            stack.append(("override_after_base", expr.defs))
            expr = expr.base
            continue

        if isinstance(expr, Identifier):
            expr = block_get(context, expr.name)
            continue

        if isinstance(expr, BackEdge):
            expr = expr.base
            continue

        if isinstance(expr, IntOp):
            # Evaluate x._inner then y._inner, then apply fn.
            stack.append(("intop_after_x", expr.fn, context))
            expr = Access(base=Identifier(name="x"), attr="_inner")
            continue

        if isinstance(expr, Select):
            # Evaluate cond._inner, then pick 'true' or 'false' in the *current* context.
            stack.append(("select_after_cond", context))
            expr = Access(base=Identifier(name="cond"), attr="_inner")
            continue

        if isinstance(expr, IntStr):
            # Evaluate _inner, then convert to string block.
            stack.append(("intstr_after_inner",))
            expr = Identifier(name="_inner")
            continue

        if isinstance(expr, StrCat):
            # Evaluate x._inner then y._inner, then concatenate.
            stack.append(("strcat_after_x", context))
            expr = Access(base=Identifier(name="x"), attr="_inner")
            continue

        if isinstance(expr, StrEq):
            # Evaluate x._inner then y._inner, then compare.
            stack.append(("streq_after_x", context))
            expr = Access(base=Identifier(name="x"), attr="_inner")
            continue

        if isinstance(expr, StrLen):
            # Evaluate x._inner, then take its length.
            stack.append(("strlen_after_x",))
            expr = Access(base=Identifier(name="x"), attr="_inner")
            continue

        if isinstance(expr, SelfRef):
            value: Node = context
        else:
            # Leaf (e.g., IntLit, StringLit, Block, BuiltInFn that evaluates to itself, etc.)
            value = expr

        # ---- Return / apply continuations ----
        while True:
            if not stack:
                return value

            tag, *rest = stack.pop()

            if tag == "access_after_base":
                attr, saved_ctx = rest
                owner = value
                # Evaluate attribute in the owner's context.
                expr = block_get(owner, attr)
                # Restore the caller's context after finishing attr evaluation.
                stack.append(("restore_ctx", saved_ctx))
                context = owner
                break  # go back to descend with new expr

            if tag == "restore_ctx":
                (saved_ctx,) = rest
                context = saved_ctx
                # Keep 'value' and continue applying higher frames.
                continue

            if tag == "override_after_base":
                (defs,) = rest
                base_block = value
                new_block = clone_tree(base_block)
                new_block.defs = list(defs) + new_block.defs
                value = new_block
                # Continue applying frames with this value.
                continue

            if tag == "intop_after_x":
                fn, saved_ctx = rest
                x = value
                assert isinstance(x, IntLit), f"{x=}"
                # After y is evaluated (and context restored), apply fn(x, y).
                stack.append(("intop_apply", fn, x, saved_ctx))
                expr = Access(base=Identifier(name="y"), attr="_inner")
                break  # descend to evaluate y

            if tag == "intop_apply":
                fn, x, saved_ctx = rest
                y = value
                assert isinstance(y, IntLit), f"{y=}"
                value = int_to_block(IntLit(value=fn(x.value, y.value)))
                # (context has already been restored by the access frame)
                continue

            if tag == "select_after_cond":
                (saved_ctx,) = rest
                cond = value
                # cond should be an int-like (0/1). If your IntLit differs, adjust here.
                assert isinstance(cond, IntLit), f"{cond=}"
                expr = (
                    Identifier(name="true") if cond.value else Identifier(name="false")
                )
                context = saved_ctx
                break  # descend to evaluate chosen branch

            if tag == "intstr_after_inner":
                x = value
                assert isinstance(x, IntLit), f"{x=}"
                value = str_to_block(StringLit(value=str(x.value)))
                continue

            if tag == "strcat_after_x":
                (saved_ctx,) = rest
                x = value
                assert isinstance(x, StringLit), f"{x=}"
                stack.append(("strcat_apply", x, saved_ctx))
                expr = Access(base=Identifier(name="y"), attr="_inner")
                break  # descend to evaluate y

            if tag == "strcat_apply":
                x, saved_ctx = rest
                y = value
                assert isinstance(y, StringLit), f"{y=}"
                value = str_to_block(StringLit(value=x.value + y.value))
                context = saved_ctx
                continue

            if tag == "streq_after_x":
                (saved_ctx,) = rest
                x = value
                assert isinstance(x, StringLit), f"{x=}"
                stack.append(("streq_apply", x, saved_ctx))
                expr = Access(base=Identifier(name="y"), attr="_inner")
                break  # descend to evaluate y

            if tag == "streq_apply":
                x, saved_ctx = rest
                y = value
                assert isinstance(y, StringLit), f"{y=}"
                value = int_to_block(IntLit(value=int(x.value == y.value)))
                context = saved_ctx
                continue

            if tag == "strlen_after_x":
                x = value
                assert isinstance(x, StringLit), f"{x=}"
                # If your StringLit stores the raw string elsewhere, adjust accordingly.
                value = int_to_block(IntLit(value=len(x.value)))
                continue

            raise RuntimeError(f"unknown continuation frame: {tag}")


def program_result(program: Block) -> Node:
    program = preprocess([], program)
    return evaluate_result(program, block_get(program, "result"))
