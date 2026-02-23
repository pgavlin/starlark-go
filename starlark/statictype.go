package starlark

import "github.com/pgavlin/starlark-go/typecheck"

// StaticType returns the typecheck.Type corresponding to the given Value.
//
// For basic types (NoneType, Bool, Int, Float, String, Bytes), the corresponding
// singleton type is returned. For containers (List, Dict, Tuple, Set), element types
// are inferred from the contents. For callable values (Function, Builtin), a Callable
// type is constructed from the parameter metadata. For all other values, a Named type
// using the value's Type() string is returned.
func StaticType(v Value) typecheck.Type {
	switch v := v.(type) {
	case NoneType:
		return typecheck.None
	case Bool:
		return typecheck.Bool
	case Int:
		return typecheck.Int
	case Float:
		return typecheck.Float
	case String:
		return typecheck.String
	case Bytes:
		return typecheck.Bytes
	case *List:
		elem := typecheck.Any
		if v.Len() > 0 {
			elem = StaticType(v.Index(0))
			for i := 1; i < v.Len(); i++ {
				elem = unifyTypes(elem, StaticType(v.Index(i)))
			}
		}
		return &typecheck.List{Elem: elem}
	case *Dict:
		key, val := typecheck.Any, typecheck.Any
		if v.Len() > 0 {
			items := v.Items()
			key = StaticType(items[0][0])
			val = StaticType(items[0][1])
			for _, item := range items[1:] {
				key = unifyTypes(key, StaticType(item[0]))
				val = unifyTypes(val, StaticType(item[1]))
			}
		}
		return &typecheck.Dict{Key: key, Value: val}
	case Tuple:
		elems := make([]typecheck.Type, len(v))
		for i, e := range v {
			elems[i] = StaticType(e)
		}
		return &typecheck.Tuple{Elems: elems}
	case *Set:
		elem := typecheck.Any
		iter := v.Iterate()
		defer iter.Done()
		var x Value
		first := true
		for iter.Next(&x) {
			if first {
				elem = StaticType(x)
				first = false
			} else {
				elem = unifyTypes(elem, StaticType(x))
			}
		}
		return &typecheck.Set{Elem: elem}
	case *Function:
		return functionStaticType(v)
	case *Builtin:
		return v.StaticType()
	default:
		return &typecheck.NamedType{TypeName: v.Type()}
	}
}

// functionStaticType builds a Callable type from a Function.
func functionStaticType(fn *Function) *typecheck.Callable {
	nparams := fn.NumParams()
	params := make([]typecheck.Param, 0, nparams)
	hasVarargs := fn.HasVarargs()
	hasKwargs := fn.HasKwargs()

	// Build a set of parameter names that have defaults.
	// fn.defaults is accessible directly since we're in the same package.
	nregular := nparams
	if hasKwargs {
		nregular--
	}
	if hasVarargs {
		nregular--
	}

	ndefaults := len(fn.defaults)
	firstDefault := nregular - ndefaults

	for i := 0; i < nregular; i++ {
		name, _ := fn.Param(i)
		p := typecheck.Param{Name: name, Type: typecheck.Any}
		if i >= firstDefault {
			p.Optional = true
		}
		params = append(params, p)
	}
	if hasVarargs {
		name, _ := fn.Param(nregular)
		params = append(params, typecheck.Param{Name: name, Type: typecheck.Any, Star: true})
		nregular++ // advance index for kwargs
	}
	if hasKwargs {
		name, _ := fn.Param(nregular)
		params = append(params, typecheck.Param{Name: name, Type: typecheck.Any, StarStar: true})
	}

	return &typecheck.Callable{
		Name:       fn.Name(),
		Params:     params,
		ReturnType: typecheck.Any,
	}
}

// unifyTypes returns a type that encompasses both a and b.
func unifyTypes(a, b typecheck.Type) typecheck.Type {
	if a == typecheck.Any || b == typecheck.Any {
		return typecheck.Any
	}
	if typesEqual(a, b) {
		return a
	}
	return typecheck.Any
}

// typesEqual reports whether two types are structurally equal.
func typesEqual(a, b typecheck.Type) bool {
	if a == b {
		return true
	}
	switch a := a.(type) {
	case *typecheck.List:
		if b, ok := b.(*typecheck.List); ok {
			return typesEqual(a.Elem, b.Elem)
		}
	case *typecheck.Dict:
		if b, ok := b.(*typecheck.Dict); ok {
			return typesEqual(a.Key, b.Key) && typesEqual(a.Value, b.Value)
		}
	case *typecheck.Set:
		if b, ok := b.(*typecheck.Set); ok {
			return typesEqual(a.Elem, b.Elem)
		}
	case typecheck.Named:
		if b, ok := b.(typecheck.Named); ok {
			return a.Name() == b.Name()
		}
	}
	return false
}
