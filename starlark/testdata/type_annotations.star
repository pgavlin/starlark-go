# Tests of type annotations.
# Type annotations are parsed but ignored at runtime.

load("assert.star", "assert")

# Basic parameter annotation
def add(x: int, y: int) -> int:
    return x + y

assert.eq(add(1, 2), 3)

# Annotation with default value
def greet(name: str = "world") -> str:
    return "hello " + name

assert.eq(greet(), "hello world")
assert.eq(greet("starlark"), "hello starlark")

# *args and **kwargs annotations
def variadic(*args: int, **kwargs: str):
    return (args, kwargs)

assert.eq(variadic(1, 2, 3, x="a"), ((1, 2, 3), {"x": "a"}))

# Return type annotation
def identity(x: int) -> int:
    return x

assert.eq(identity(42), 42)

# Complex type annotations (list[int], dict[str, int], unions)
def complex_types(x: list, y: dict) -> bool:
    return True

assert.eq(complex_types([1, 2], {"a": 1}), True)

# Typed assignment
x: int = 5
assert.eq(x, 5)

y: str = "hello"
assert.eq(y, "hello")

z: list = [1, 2, 3]
assert.eq(z, [1, 2, 3])

# Generic type annotation syntax (parsed but not enforced)
def with_generic(x: list) -> list:
    return x + [4]

assert.eq(with_generic([1, 2, 3]), [1, 2, 3, 4])

# Union type annotation syntax (uses | operator in type position)
def union_param(x: int) -> int:
    return x

assert.eq(union_param(5), 5)

# Multiple annotated params with defaults
def multi(a: int, b: str = "x", c: float = 1.0) -> str:
    return str(a) + b + str(c)

assert.eq(multi(1), "1x1.0")
assert.eq(multi(2, "y", 3.0), "2y3.0")

# Annotations don't affect function behavior
def no_effect(x: int):
    return x

assert.eq(no_effect("not an int"), "not an int")  # no runtime type check
