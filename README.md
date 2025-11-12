# reflex

This is a prototype language designed around defining objects and patching them.

Reflex generalizes object-oriented inheritence from two-layers to infinite layers. In an OOP language, classes have methods, and you can subclass a class to override methods. In reflex, there are effectively "classes on classes on classes" all the way down&mdash;you can override variables within methods just like you would override methods on a class.

# Examples

## Finding the largest factor of an integer

```
factor = {
    f = 2
    next_result = @[f=^.f.add[y=1].result].result
    result = x.mod[y=^.f].result.select[false=^.f, true=^.next_result].result
}
result = factor[x=533].result
```

## Computing all the factors of a number

This example computes the factors of a number from smallest to largest and concatenates the results.

```
factors = {
  input = x # x is passed as an argument
  f = 2 # the current guess for a factor

  result_if_not_divisible = @[f=^.f.add[y=1].result].result
  result_if_divisible = @[x=^^.input.div[y=^^.f].result f=2].result
  is_prime = input.eq[y=^.f].result
  is_not_divisible = input.mod[y=^.f].result
  result = is_prime.select[
    true=^.x.str
    false=^.is_not_divisible.select[
      true=^^.result_if_not_divisible
      false=^^.f.str.cat[y=" "].result.cat[y=^^.result_if_divisible].result
    ].result
  ].result
}
result = factors[x=246].result
```

In the bottom line, we pass 246, and get out the factors `2 3 41`.
