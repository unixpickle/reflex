# reflex

This is a prototype language designed around defining objects and patching them.

Reflex generalizes object-oriented inheritence to infinite levels of recurrence. In an OOP language, classes have methods, and you can subclass a class to override methods. In reflex, there are effectively "classes on classes on classes" all the way down&mdash;you can override variables within methods just like you would by overriding methods on a class.

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
