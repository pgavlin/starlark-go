package typecheck

import (
	"github.com/pgavlin/starlark-go/syntax"
)

// evalType converts an AST type expression into an internal Type.
func (c *Checker) evalType(expr syntax.Expr) Type {
	if expr == nil {
		return Any
	}

	switch e := expr.(type) {
	case *syntax.Ident:
		switch e.Name {
		case "int":
			return Int
		case "str", "string":
			return String
		case "bool":
			return Bool
		case "float":
			return Float
		case "bytes":
			return Bytes
		case "None", "NoneType":
			return None
		case "list":
			return &List{Any}
		case "dict":
			return &Dict{Any, Any}
		case "tuple":
			return &Tuple{}
		case "set":
			return &Set{Any}
		case "any":
			return Any
		}
		// Check Universe for type names.
		if t, ok := Universe[e.Name]; ok {
			return t
		}
		// Check env.Names for extension type names.
		if c.tenv != nil && c.tenv.Names != nil {
			if t, ok := c.tenv.Names[e.Name]; ok {
				return t
			}
		}
		c.errorf(e.NamePos, "unknown type: %s", e.Name)
		return Any

	case *syntax.IndexExpr:
		// Generic type: list[int], dict[str, int], set[int], tuple[int, str]
		base := c.evalType(e.X)
		switch b := base.(type) {
		case *List:
			return &List{Elem: c.evalType(e.Y)}
		case *Dict:
			// dict[K, V] - Y should be a tuple expression
			if tuple, ok := e.Y.(*syntax.TupleExpr); ok && len(tuple.List) == 2 {
				return &Dict{Key: c.evalType(tuple.List[0]), Value: c.evalType(tuple.List[1])}
			}
			return &Dict{Key: c.evalType(e.Y), Value: Any}
		case *Set:
			return &Set{Elem: c.evalType(e.Y)}
		case *Tuple:
			_ = b
			if tuple, ok := e.Y.(*syntax.TupleExpr); ok {
				elems := make([]Type, len(tuple.List))
				for i, el := range tuple.List {
					elems[i] = c.evalType(el)
				}
				return &Tuple{Elems: elems}
			}
			return &Tuple{Elems: []Type{c.evalType(e.Y)}}
		default:
			return Any
		}

	case *syntax.BinaryExpr:
		if e.Op == syntax.PIPE {
			// Union type: int | str
			left := c.evalType(e.X)
			right := c.evalType(e.Y)
			// Flatten nested unions.
			var types []Type
			if u, ok := left.(*Union); ok {
				types = append(types, u.Types...)
			} else {
				types = append(types, left)
			}
			if u, ok := right.(*Union); ok {
				types = append(types, u.Types...)
			} else {
				types = append(types, right)
			}
			return &Union{Types: types}
		}
		return Any

	default:
		return Any
	}
}
