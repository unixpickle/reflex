from dataclasses import dataclass, field
from typing import Callable
from weakref import WeakKeyDictionary

_Empty = WeakKeyDictionary()


class Node:
    def __eq__(self, other):
        return id(self) == id(other)

    def __hash__(self):
        return id(self)

    def lazy_clone(
        self, overrides: WeakKeyDictionary["Node", "Node"] | None = None
    ) -> "Node":
        raise NotImplementedError

    def propagate_clone(self) -> "Node":
        raise NotImplementedError

    @property
    def clone_overrides(self) -> WeakKeyDictionary["Node", "Node"] | None:
        return getattr(self, "_clone_overrides", None)

    @clone_overrides.setter
    def clone_overrides(self, d: WeakKeyDictionary["Node", "Node"] | None):
        self._clone_overrides = _walk_redundant_paths(d)


def _walk_redundant_paths(
    overrides: WeakKeyDictionary[Node, Node] | None,
) -> WeakKeyDictionary[Node, Node]:
    if overrides is None:
        return None
    while True:
        new_result = WeakKeyDictionary()
        to_remove = set()
        for k, v in overrides.items():
            if k in to_remove:
                continue
            if (x := overrides.get(v, None)) is not None:
                to_remove.add(v)
                new_result[k] = x
            else:
                new_result[k] = v
        for x in to_remove:
            if x in new_result:
                del new_result[x]
        if len(new_result) == len(overrides):
            return overrides
        overrides = new_result


@dataclass(eq=False)
class Block(Node):
    defs: dict[str, Node]

    def lazy_clone(
        self, overrides: WeakKeyDictionary[Node, Node] | None = None
    ) -> Node:
        result = Block(defs=self.defs.copy())
        result.clone_overrides = (
            (overrides or _Empty)
            | WeakKeyDictionary({self: result})
            | (self.clone_overrides or _Empty)
        )
        return result

    def propagate_clone(self):
        if overrides := self.clone_overrides:
            self.defs = {
                k: v.lazy_clone(overrides=overrides) for k, v in self.defs.items()
            }
            self.clone_overrides = None


class AtomicNode(Node):
    def lazy_clone(
        self, overrides: WeakKeyDictionary[Node, Node] | None = None
    ) -> Node:
        _ = overrides
        return self

    def propagate_clone(self):
        pass


@dataclass(eq=False)
class Identifier(AtomicNode):
    name: str


@dataclass(eq=False)
class IntLit(AtomicNode):
    value: int


@dataclass(eq=False)
class StringLit(AtomicNode):
    value: str


@dataclass(eq=False)
class CloneAttr(AtomicNode):
    attr: str


@dataclass(eq=False)
class SelfRef(Node):
    pass


@dataclass(eq=False)
class Parent(Node):
    depth: int


@dataclass(eq=False)
class AncestorLookup(Node):
    name: str


@dataclass(eq=False)
class Access(Node):
    base: Node
    attr: str

    def lazy_clone(
        self, overrides: WeakKeyDictionary[Node, Node] | None = None
    ) -> Node:
        result = Access(base=self.base, attr=self.attr)
        result.clone_overrides = (overrides or _Empty) | (
            self.clone_overrides or _Empty
        )
        return result

    def propagate_clone(self):
        if overrides := self.clone_overrides:
            self.base = self.base.lazy_clone(overrides=overrides)
            self.clone_overrides = None


@dataclass(eq=False)
class BaseAndDefs(Node):
    base: Node
    defs: dict[str, Node]

    def propagate_clone(self):
        if overrides := self.clone_overrides:
            self.base = self.base.lazy_clone(overrides=overrides)
            self.defs = {
                k: v.lazy_clone(overrides=overrides) for k, v in self.defs.items()
            }
            self.clone_overrides = None


@dataclass(eq=False)
class Override(BaseAndDefs):
    def lazy_clone(
        self, overrides: WeakKeyDictionary[Node, Node] | None = None
    ) -> Node:
        result = Override(base=self.base, defs=self.defs.copy())
        result.clone_overrides = (
            (overrides or _Empty)
            | WeakKeyDictionary({self: result})
            | (self.clone_overrides or _Empty)
        )
        return result


@dataclass(eq=False)
class Call(BaseAndDefs):
    def lazy_clone(
        self, overrides: WeakKeyDictionary[Node, Node] | None = None
    ) -> Node:
        # There will be no back-edges pointing to a Call, so we don't have to add an override.
        result = Call(base=self.base, defs=self.defs.copy())
        result.clone_overrides = (overrides or _Empty) | (
            self.clone_overrides or _Empty
        )
        return result


@dataclass(eq=False)
class Eager(Node):
    base: Node

    def lazy_clone(
        self, overrides: WeakKeyDictionary[Node, Node] | None = None
    ) -> Node:
        result = Eager(base=self.base)
        result.clone_overrides = (overrides or _Empty) | (
            self.clone_overrides or _Empty
        )
        return result

    def propagate_clone(self):
        if overrides := self.clone_overrides:
            self.base = self.base.lazy_clone(overrides=overrides)
            self.clone_overrides = None


@dataclass(eq=False)
class BackEdge(Node):
    base: Block | Override

    def lazy_clone(
        self, overrides: WeakKeyDictionary[Node, Node] | None = None
    ) -> Node:
        result = BackEdge(base=self.base)
        result.clone_overrides = (overrides or _Empty) | (
            self.clone_overrides or _Empty
        )
        return result

    def propagate_clone(self):
        if overrides := self.clone_overrides:
            # We might have to trace down multiple steps of overrides
            while new_base := overrides.get(self.base, None):
                self.base = new_base
            self.clone_overrides = None


@dataclass(eq=False)
class BuiltInFn(Node):
    context: BackEdge

    def lazy_clone(
        self, overrides: WeakKeyDictionary[Node, Node] | None = None
    ) -> Node:
        result = type(self)(context=self.context)
        result.clone_overrides = (overrides or _Empty) | (
            self.clone_overrides or _Empty
        )
        return result

    def propagate_clone(self):
        if overrides := self.clone_overrides:
            self.context = self.context.lazy_clone(overrides=overrides)
            self.clone_overrides = None


@dataclass(eq=False)
class IntOp(BuiltInFn):
    fn: Callable[[int, int], int]

    def lazy_clone(
        self, overrides: WeakKeyDictionary[Node, Node] | None = None
    ) -> Node:
        result = IntOp(context=self.context, fn=self.fn)
        result.clone_overrides = (overrides or _Empty) | (
            self.clone_overrides or _Empty
        )
        return result

    def propagate_clone(self):
        if overrides := self.clone_overrides:
            self.context = self.context.lazy_clone(overrides=overrides)
            self.clone_overrides = None
