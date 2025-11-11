from dataclasses import dataclass
from typing import Callable

from parser import (
    Access,
    Block,
    Definition,
    Identifier,
    IntLit,
    Node,
    Override,
    Parent,
    SelfRef,
)


@dataclass
class BackEdge(Node):
    base: Block | Override


@dataclass
class IntOp(Node):
    fn: Callable[[int, int], int]


@dataclass
class Select(Node):
    pass


def int_op_block(parent: Block, fn: Callable[[int, int], int]) -> Block:
    return Block(
        defs=[
            Definition(name="x", expr=BackEdge(base=parent)),
            Definition(name="result", expr=IntOp(fn=fn)),
        ]
    )


def int_to_block(node: IntLit) -> Block:
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


def block_get(block: Block | Override, key: str) -> Node:
    """
    Get a key, prioritizing earlier definitions over later ones.
    """
    assert isinstance(block, (Block, Override)), (
        f"cannot get key {key!r} of node type {type(block)}"
    )
    for d in block.defs:
        if key == d.name:
            return d.expr
    if isinstance(block, Override):
        return block_get(block.base, key)
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
    elif isinstance(expr, SelfRef):
        return parents[-1]
    elif isinstance(expr, Block):
        b = Block(defs=[])
        b.defs = [preprocess(parents + [b], d) for d in expr.defs]
        return b
    elif isinstance(expr, Override):
        o = Override(base=None, defs=[])
        o.base = preprocess(parents, expr.base)
        o.defs = [preprocess(parents + [o], d) for d in expr.defs]
        return o
    elif isinstance(expr, Definition):
        return Definition(name=expr.name, expr=preprocess(parents, expr.expr))
    elif isinstance(expr, IntLit):
        return int_to_block(expr)
    elif isinstance(expr, Identifier):
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
    elif isinstance(node, (SelfRef, IntLit, Identifier, Select, IntOp)):
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
    elif isinstance(node, (SelfRef, IntLit, Identifier, Select, IntOp)):
        pass
    else:
        raise ValueError(f"cannot clone block: {type(node)}")


def clone_tree(node: Node) -> Node:
    copies = dict()
    new_node = _clone_blocks(node, copies)
    _replace_back_edges(new_node, copies)
    return new_node


def evaluate_result(context: Block | Override, expr: Node) -> Node:
    if isinstance(expr, Access):
        owner = evaluate_result(context, expr.base)
        lookup = block_get(owner, expr.attr)
        return evaluate_result(owner, lookup)
    elif isinstance(expr, Override):
        base_block = evaluate_result(context, expr.base)
        new_block = clone_tree(base_block)
        new_block.defs = expr.defs + new_block.defs
        return new_block
    elif isinstance(expr, Identifier):
        return evaluate_result(context, block_get(context, expr.name))
    elif isinstance(expr, BackEdge):
        return evaluate_result(context, expr.base)
    elif isinstance(expr, IntOp):
        x = evaluate_result(context, Access(base=Identifier(name="x"), attr="_inner"))
        y = evaluate_result(context, Access(base=Identifier(name="y"), attr="_inner"))
        assert isinstance(x, IntLit)
        assert isinstance(y, IntLit)
        result = expr.fn(x.value, y.value)
        return int_to_block(IntLit(value=result))
    elif isinstance(expr, Select):
        cond = evaluate_result(
            context, Access(base=Identifier(name="cond"), attr="_inner")
        )
        if cond.value:
            return evaluate_result(context, Identifier(name="true"))
        else:
            return evaluate_result(context, Identifier(name="false"))
    else:
        return expr


def program_result(program: Block) -> Node:
    program = preprocess([], program)
    return evaluate_result(program, block_get(program, "result"))
