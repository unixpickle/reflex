# reflex

This is a prototype language designed around defining objects and patching them.

Reflex generalizes object-oriented inheritence from two layers to infinite layers. In an OOP language, classes have methods, and you can subclass a class to override methods. In reflex, there are effectively "classes on classes on classes" all the way down&mdash;you can override variables within methods just like you would override methods on a class.

# Examples

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

The above code accesses the `result` property of objects quite a lot. The `!` operator is syntactic sugar for this:

```
factor = {
    f = 2
    next_result = @[f=^.f.add[y=1]!]!
    result = x.mod[y=^.f]!.select[false=^.f, true=^.next_result]!
}
result = factor[x=533]!
```

## Computing all the factors of a number

This example computes the factors of a number from smallest to largest and concatenates the results.

```
factors = {
  input = x # x is passed as an argument
  f = 2 # the current guess for a factor

  result_if_not_divisible = @[f=^.f.add[y=1]!]!
  result_if_divisible = @[x=^^.input.div[y=^^.f]! f=2]!
  is_prime = input.eq[y=^.f]!
  is_not_divisible = input.mod[y=^.f]!
  result = is_prime.select[
    true=^.x.str
    false=^.is_not_divisible.select[
      true=^^.result_if_not_divisible
      false=^^.f.str.cat[y=" "]!.cat[y=^^.result_if_divisible]!
    ]!
  ]!
}
result = factors[x=246]!
```

In the bottom line, we pass 246, and get out the string `"2 3 41"`.

## Call syntax with parentheses

Sometimes, we might end up needing to do a few nested overrides, and we end up chaining `^` operators:

```
result = x.select[false=3 true=^.y[z=^.^.w]]!
```

Every time we override in brackets, we create a new scope, and we have to add another `^`.
We can instead use parentheses, in which case the scope doesn't change, and we can use `@` to access members of the calling scope.

```
result = x.select(false=3 true=@.y(z=@.w))!
```

In this example, we can't use `true=y(...)`, because without the `@` operator, we don't create a back edge pointer
and would instead attempt to lookup the key 'y' directly in the select block.
