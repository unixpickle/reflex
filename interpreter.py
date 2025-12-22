from dataclasses import dataclass
from typing import Callable

from parser import (
    Access,
    AncestorLookup,
    Block,
    Call,
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
class IntChr(BuiltInFn):
    pass


@dataclass
class Select(BuiltInFn):
    pass


def int_op_block(parent: Block, fn: Callable[[int, int], int]) -> Block:
    return Block(
        defs=dict(
            x=BackEdge(base=parent),
            result=IntOp(fn=fn),
        )
    )


def int_to_block(node: IntLit) -> Block:
    assert isinstance(node, IntLit)
    result = Block(defs=dict(_inner=node))
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
            str=IntStr(),
            chr=IntChr(),
            select=Block(
                defs=dict(
                    cond=BackEdge(base=result),
                    result=Select(),
                )
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


@dataclass
class StrSubstr(BuiltInFn):
    pass


def str_to_block(node: StringLit) -> Block:
    assert isinstance(node, StringLit)
    result = Block(defs=dict(_inner=node))
    result.defs.update(
        dict(
            cat=Block(
                defs=dict(
                    x=BackEdge(base=result),
                    result=StrCat(),
                )
            ),
            eq=Block(
                defs=dict(
                    x=BackEdge(base=result),
                    result=StrEq(),
                )
            ),
            len=Block(
                defs=dict(
                    x=BackEdge(base=result),
                    result=StrLen(),
                )
            ),
            substr=Block(
                defs=dict(
                    x=BackEdge(base=result),
                    start=int_to_block(IntLit(value=0)),
                    end=StrLen(),
                    result=StrSubstr(),
                )
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
        return expr
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
        assert id(node) not in result, "somehow found a circular reference"
        result[id(node)] = Block(defs={})
        result[id(node)].defs = {
            k: _clone_blocks(v, result) for k, v in node.defs.items()
        }
        return result[id(node)]
    elif isinstance(node, Override):
        assert id(node) not in result, "somehow found a circular reference"
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
    elif isinstance(node, (IntLit, StringLit, Identifier, BuiltInFn)):
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
    elif isinstance(node, (IntLit, StringLit, Identifier, BuiltInFn)):
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

        if isinstance(expr, (Call, Override)):
            stack.append(("override_after_base", expr))
            expr = expr.base
            continue

        if isinstance(expr, Identifier):
            # --- CACHE: bare identifier lookup in the current Block context ---
            if isinstance(context, Block) and expr.name in context.cache:
                # Treat cached value as the next expression (which will be a leaf).
                expr = context.cache[expr.name]
                continue

            # Not cached: evaluate the definition and then cache the *evaluated* value.
            if isinstance(context, Block):
                stack.append(("cache_identifier", context, expr.name))
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

        if isinstance(expr, IntChr):
            # Evaluate _inner, then convert to string block.
            stack.append(("intchr_after_inner",))
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

        if isinstance(expr, StrSubstr):
            # Evaluate x._inner, then start._inner, then end._inner; slice s[start:end].
            stack.append(("strsubstr_after_x", context))
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

                # --- CACHE fast path for attribute access on a Block owner ---
                if isinstance(owner, Block) and attr in owner.cache:
                    value = owner.cache[attr]
                    context = saved_ctx
                    # do not push restore frame; we already restored
                    continue

                # Evaluate attribute in the owner's context, and cache result afterwards.
                expr = block_get(owner, attr)
                stack.append(("cache_attr_then_restore", owner, attr, saved_ctx))
                context = owner
                break  # go back to descend with new expr

            if tag == "cache_attr_then_restore":
                owner, attr, saved_ctx = rest
                # Store the fully evaluated result into the owner's cache.
                if isinstance(owner, Block):
                    owner.cache[attr] = value
                context = saved_ctx
                # Keep 'value' and continue applying higher frames.
                continue

            if tag == "restore_ctx":
                (saved_ctx,) = rest
                context = saved_ctx
                continue

            if tag == "override_after_base":
                (override,) = rest
                base_block = value
                new_block = clone_tree(base_block)
                new_block.defs = new_block.defs.copy()
                for k, v in override.defs.items():
                    new_def = clone_tree(v)
                    _replace_back_edges(new_def, {id(override): new_block})
                    new_block.defs[k] = new_def
                value = new_block
                continue

            if tag == "cache_identifier":
                owner_block, name = rest
                if isinstance(owner_block, Block):
                    owner_block.cache[name] = value
                # keep 'value'
                continue

            if tag == "intop_after_x":
                fn, saved_ctx = rest
                x = value
                assert isinstance(x, IntLit), f"{x=}"
                stack.append(("intop_apply", fn, x, saved_ctx))
                expr = Access(base=Identifier(name="y"), attr="_inner")
                break

            if tag == "intop_apply":
                fn, x, saved_ctx = rest
                y = value
                assert isinstance(y, IntLit), f"{y=}"
                value = int_to_block(IntLit(value=fn(x.value, y.value)))
                continue

            if tag == "select_after_cond":
                (saved_ctx,) = rest
                cond = value
                assert isinstance(cond, IntLit), f"{cond=}"
                expr = (
                    Identifier(name="true") if cond.value else Identifier(name="false")
                )
                context = saved_ctx
                break

            if tag == "intstr_after_inner":
                x = value
                assert isinstance(x, IntLit), f"{x=}"
                value = str_to_block(StringLit(value=str(x.value)))
                continue

            if tag == "intchr_after_inner":
                x = value
                assert isinstance(x, IntLit), f"{x=}"
                value = str_to_block(StringLit(value=chr(x.value)))
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

            if tag == "strsubstr_after_x":
                (saved_ctx,) = rest
                x = value
                assert isinstance(x, StringLit), f"{x=}"
                # Next, evaluate start._inner
                stack.append(("strsubstr_after_start", x, saved_ctx))
                expr = Access(base=Identifier(name="start"), attr="_inner")
                break  # descend to evaluate start

            if tag == "strsubstr_after_start":
                x, saved_ctx = rest
                start = value
                assert isinstance(start, IntLit), f"{start=}"
                # Next, evaluate end._inner
                stack.append(("strsubstr_apply", x, start, saved_ctx))
                expr = Access(base=Identifier(name="end"), attr="_inner")
                break  # descend to evaluate end

            if tag == "strsubstr_apply":
                x, start, saved_ctx = rest
                end = value
                assert isinstance(x, StringLit), f"{x=}"
                assert isinstance(start, IntLit), f"{start=}"
                assert isinstance(end, IntLit), f"{end=}"
                # Inclusive start, exclusive end
                substr = x.value[start.value : end.value]
                value = str_to_block(StringLit(value=substr))
                context = saved_ctx
                continue

            raise RuntimeError(f"unknown continuation frame: {tag}")


def program_result(program: Block) -> Node:
    program = preprocess([], program)
    return evaluate_result(program, block_get(program, "result"))
