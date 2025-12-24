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

## Computing all the factors of a number

This example computes the factors of a number from smallest to largest and concatenates the results.

```
factors = {
  input = x # x is passed as an argument
  f = 2 # the current guess for a factor

  result_if_not_divisible = @(f=f.add(y=1)!)!
  result_if_divisible = @(x=input.div(y=f)! f=2)!
  is_prime = input.eq(y=f)!
  is_not_divisible = input.mod(y=f)!
  result = is_prime.select(
    true=x.str
    false=is_not_divisible.select(
      true=result_if_not_divisible
      false=f.str.cat(y=" ")!.cat(y=result_if_divisible)!
    )!
  )!
}
result = factors(x=246)!
```

In the bottom line, we pass 246, and get out the string `"2 3 41"`.
