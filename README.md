# reflex

This is a prototype language designed around defining objects and patching them.

Reflex generalizes object-oriented inheritence from two layers to infinite layers. In an OOP language, classes have methods, and you can subclass a class to override methods. In reflex, there are effectively "classes on classes on classes" all the way down&mdash;you can override variables within methods just like you would override methods on a class.

# Examples

## Blocks

The main primitive in reflex is the *block*. A block is simply a lookup table of symbol names and expressions. The entire program is itself a block. We can also create sub-blocks using curly-braces:

```
a = {
  b = 3
  d = 4
}
result = a.b # result is 3
```

## Overriding

We can override values in a block using `[` and `]` with new keys and values inside. Note that values
are actually expressions which are evaluated lazily, which you can see illustrated in this example:

```
a = {
  x = 3
  y = x # y points to a field called x
  z = 5
}
b = a.y # b is 3
c = a[x=4].y # c is 4

# d is still 3, because x is a constant and y is a pointer.
d = a[y=4].x

# e is 4, because y is no longer an expression pointing to x 
e = a[y=4].y

f = a[y=z].y # f is 5

f = a[z=2 y=z].y # f is 2
```

## Arithmetic

Integers have various sub-blocks which perform arithmetic. For example, a number has an `add` block, which by default is missing a `y` field (the second operand to the addition). If we set the `y` field, then we can read the `result` field to see the sum:

```
a = 3
b = a.add[y=4].result # b will equal 7
```

## The `^` operator

You can access parent scopes using the `^` operator, with some pretty interesting consequences.

```
a = {
  b = {
    c = ^.d
  }
  d = 4
  e = 5
  x = {
    z = 3
    y = {
        A = ^.^.d # reference to d two scopes up
        B = ^.z
    }
  }
}

f = a.b.c # equal to 4
g = a[d=e].b.c # equal to 5

h = 7
i = a.x[z=^.h].y.B # equal to 7, here ^ refers to root scope, not `a`

result = i
```

You can also use the `^^` operator to climb scopes until a scope where the given symbol is defined.

## Finding the largest factor of an integer

The following code finds the smallest factor in a number

```
factor = {
    f = 2
    next_result = @[f=^.f.add[y=1].result].result
    result = x.mod[y=^.f].result.select[false=^.f, true=^.next_result].result
}
result = factor[x=533].result
```

The above code accesses the `result` property of objects quite a lot. The `!` operator is syntactic sugar for accessing the result property, for example `x.result` can be written as `x!`:

```
factor = {
    f = 2
    next_result = @[f=^.f.add[y=1]!]!
    result = x.mod[y=^.f]!.select[false=^.f, true=^.next_result]!
}
result = factor[x=533]!
```

## Call syntax

In the above example, we used the `^` operator to access the parent scope of a given scope, e.g. `@[f=^.f.add[...]!]` creates a new object where the `f` field references the parent object's `f` field.

Using `(` and `)` instead of `[` and `]` implicitly references the parent scope within the overridden expressions. With this in mind, we can simplify the code to remove various `^` operators.

```
factor = {
  f = 2
  next_result = @(f=f.add[y=1]!)!
  result = x.mod(y=f)!.select(false=f, true=next_result)!
}
result = factor[x=533]!
```

## Binary and ternary operators

In the above, simple things seem pretty verbose, such as `f.add(y=1)!` to compute `f + 1`. Luckily, there is some syntactic sugar for these kinds of things, e.g. the `+` operator is a shorthand for calling `add` and passing `y`.

We also use the `select` call on an integer, which returns the value of `true` if the integer is nonzero, or `false` otherwise. The syntactic shortcut for this is ternary syntax, i.e. `cond ? ifTrue : ifFalse`. This is syntactic sugar for `cond.select(true=ifTrue false=ifFalse)!`.

Reworking the above example:

```
factor = {
  f = 2
  next_result = @(f=f + 1)!
  result = x % f ? next_result : f
}
result = factor[x=533]!
```

## Eager evaluation

When we set a value inside a call or override, we are actually passing a *lazy expression*&mdash;nothing is evaluated right away. This can result in excessive memory usage or redundant computation. For example, consider this code to compute the n-th Fibonacci number:

```
Fib = {
  a = 1
  b = 1
  next = @(a=b b=a + b)

  after = {
    result = n ? ^.next.after(n:=n - 1)! : ^
  }
}

result = Fib.after[n=5]!.b
```

This code seems clean, but it actually has exponential runtime. Every time we access `b` on a `Fib` object, we reevaluate the previous `Fib`'s `a` and `b`; every time we access `a`, we will reevaluate the previous `Fib`'s `b`. As a result, computing the n-th Fibonacci number requires computing the previous Fibonacci sequence **twice**, leading to exponential blowup.

When we use call syntax, we have the option to evaluate the passed expression *eagerly* rather than passing it as a lazy expression. We can do this by using `:=` instead of `=`. This can be thought of as a way to cache a result, or to force a computation to happen before the result is needed.

With this simple change, the above code becomes linear time:

```diff
- next = @(a=b b=a + b)
+ next = @(a:=b b:=a + b)
```

## Computing all the factors of a number

This example computes the factors of a number from smallest to largest and concatenates the results.

```
factors = {
  input = x # x is passed as an argument
  f = 2 # the current guess for a factor

  result_if_not_divisible = @(f:=f + 1)!
  result_if_divisible = @(x:=input / f, f=2)!
  is_prime = f == input
  is_not_divisible = input % f
  result = is_prime
    ? x.str
    : is_not_divisible
      ? result_if_not_divisible
      : f.str + " " + result_if_divisible
}

result = factors(x=246)!
```

In the bottom line, we pass 246, and get out the string `"2 3 41"`.
