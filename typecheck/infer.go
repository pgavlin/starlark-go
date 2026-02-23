package typecheck

import (
	"github.com/pgavlin/starlark-go/syntax"
)

// exprType infers the type of an expression and records it.
func (c *Checker) exprType(expr syntax.Expr) Type {
	typ := c.exprTypeInner(expr)
	if expr != nil {
		c.record(expr, typ)
	}
	return typ
}

// exprTypeInner performs the actual type inference without recording.
func (c *Checker) exprTypeInner(expr syntax.Expr) Type {
	if expr == nil {
		return Any
	}

	switch e := expr.(type) {
	case *syntax.Ident:
		if e.Name == "None" {
			return None
		}
		if e.Name == "True" || e.Name == "False" {
			return Bool
		}
		// Look up via resolve.Binding.
		if b := c.lookupBinding(e); b != nil {
			// Record in Uses map.
			if c.info != nil && c.info.Uses != nil {
				c.info.Uses[e] = b
			}
			return b.Type
		}
		return Any

	case *syntax.Literal:
		switch e.Token {
		case syntax.INT:
			return Int
		case syntax.FLOAT:
			return Float
		case syntax.STRING:
			return String
		case syntax.BYTES:
			return Bytes
		}
		return Any

	case *syntax.ListExpr:
		if len(e.List) == 0 {
			return &List{Any}
		}
		elemType := c.exprType(e.List[0])
		for _, el := range e.List[1:] {
			t := c.exprType(el)
			elemType = c.unify(elemType, t)
		}
		return &List{elemType}

	case *syntax.DictExpr:
		if len(e.List) == 0 {
			return &Dict{Any, Any}
		}
		var keyType, valType Type
		for _, entry := range e.List {
			de := entry.(*syntax.DictEntry)
			kt := c.exprType(de.Key)
			vt := c.exprType(de.Value)
			if keyType == nil {
				keyType = kt
				valType = vt
			} else {
				keyType = c.unify(keyType, kt)
				valType = c.unify(valType, vt)
			}
		}
		return &Dict{keyType, valType}

	case *syntax.TupleExpr:
		elems := make([]Type, len(e.List))
		for i, el := range e.List {
			elems[i] = c.exprType(el)
		}
		return &Tuple{Elems: elems}

	case *syntax.Comprehension:
		bodyType := c.exprType(e.Body)
		if e.Curly {
			if de, ok := e.Body.(*syntax.DictEntry); ok {
				return &Dict{c.exprType(de.Key), c.exprType(de.Value)}
			}
			return &Set{bodyType}
		}
		return &List{bodyType}

	case *syntax.BinaryExpr:
		return c.binaryExprType(e)

	case *syntax.UnaryExpr:
		return c.unaryExprType(e)

	case *syntax.CallExpr:
		return c.callExprType(e)

	case *syntax.DotExpr:
		return c.dotExprType(e)

	case *syntax.IndexExpr:
		return c.indexExprType(e)

	case *syntax.SliceExpr:
		return c.sliceExprType(e)

	case *syntax.CondExpr:
		trueType := c.exprType(e.True)
		falseType := c.exprType(e.False)
		return c.unify(trueType, falseType)

	case *syntax.ParenExpr:
		return c.exprType(e.X)

	case *syntax.LambdaExpr:
		return &Callable{Name: "lambda", ReturnType: Any}

	case *syntax.TypeAnnotatedExpr:
		return c.exprType(e.X)

	case *syntax.FStringExpr:
		return String
	}

	return Any
}

func (c *Checker) binaryExprType(e *syntax.BinaryExpr) Type {
	left := c.exprType(e.X)
	right := c.exprType(e.Y)

	switch e.Op {
	case syntax.PLUS:
		if t := c.plusType(left, right); t != Any {
			return t
		}
		if hb, ok := left.(HasBinaryType); ok {
			if t := hb.BinaryType(e.Op); t != nil {
				return t
			}
		}
		return Any
	case syntax.MINUS, syntax.PERCENT, syntax.SLASHSLASH:
		if t := c.arithmeticType(left, right); t != Any {
			return t
		}
		if hb, ok := left.(HasBinaryType); ok {
			if t := hb.BinaryType(e.Op); t != nil {
				return t
			}
		}
		return Any
	case syntax.STAR:
		if t := c.starType(left, right); t != Any {
			return t
		}
		if hb, ok := left.(HasBinaryType); ok {
			if t := hb.BinaryType(e.Op); t != nil {
				return t
			}
		}
		return Any
	case syntax.SLASH:
		// Division returns float for built-in numeric types.
		if (left == Int || left == Float) && (right == Int || right == Float) {
			return Float
		}
		if hb, ok := left.(HasBinaryType); ok {
			if t := hb.BinaryType(e.Op); t != nil {
				return t
			}
		}
		return Any
	case syntax.EQL, syntax.NEQ:
		return Bool
	case syntax.LT, syntax.GT, syntax.LE, syntax.GE:
		// Warn if an extension type does not support ordering.
		for _, operand := range []Type{left, right} {
			if ct, ok := operand.(ComparableType); ok && !ct.IsComparable() {
				c.errorf(e.OpPos, "type %s does not support %s", operand, e.Op)
				break
			}
		}
		return Bool
	case syntax.IN, syntax.NOT_IN:
		return Bool
	case syntax.AND:
		return c.unify(left, right)
	case syntax.OR:
		return c.unify(left, right)
	case syntax.PIPE, syntax.AMP, syntax.CIRCUMFLEX, syntax.LTLT, syntax.GTGT:
		if left == Int && right == Int {
			return Int
		}
		if hb, ok := left.(HasBinaryType); ok {
			if t := hb.BinaryType(e.Op); t != nil {
				return t
			}
		}
		return Any
	}

	return Any
}

func (c *Checker) plusType(left, right Type) Type {
	if left == Int && right == Int {
		return Int
	}
	if left == Float || right == Float {
		if (left == Int || left == Float) && (right == Int || right == Float) {
			return Float
		}
	}
	if left == String && right == String {
		return String
	}
	if left == Bytes && right == Bytes {
		return Bytes
	}
	if ll, ok := left.(*List); ok {
		if rl, ok := right.(*List); ok {
			return &List{c.unify(ll.Elem, rl.Elem)}
		}
	}
	if _, ok := left.(*Tuple); ok {
		if _, ok := right.(*Tuple); ok {
			return &Tuple{} // approximate
		}
	}
	return Any
}

func (c *Checker) arithmeticType(left, right Type) Type {
	if left == Int && right == Int {
		return Int
	}
	if (left == Int || left == Float) && (right == Int || right == Float) {
		return Float
	}
	return Any
}

func (c *Checker) starType(left, right Type) Type {
	if left == Int && right == Int {
		return Int
	}
	if (left == Int || left == Float) && (right == Int || right == Float) {
		return Float
	}
	// int * str or str * int
	if (left == Int && right == String) || (left == String && right == Int) {
		return String
	}
	// int * list or list * int
	if left == Int {
		if rl, ok := right.(*List); ok {
			return rl
		}
	}
	if right == Int {
		if ll, ok := left.(*List); ok {
			return ll
		}
	}
	return Any
}

func (c *Checker) unaryExprType(e *syntax.UnaryExpr) Type {
	if e.X == nil {
		return Any
	}
	x := c.exprType(e.X)
	switch e.Op {
	case syntax.NOT:
		return Bool
	case syntax.MINUS, syntax.PLUS:
		if x == Int || x == Float {
			return x
		}
		if hu, ok := x.(HasUnaryType); ok {
			if t := hu.UnaryType(e.Op); t != nil {
				return t
			}
		}
		return Any
	case syntax.TILDE:
		if x == Int {
			return Int
		}
		if hu, ok := x.(HasUnaryType); ok {
			if t := hu.UnaryType(e.Op); t != nil {
				return t
			}
		}
		return Any
	}
	return Any
}

func (c *Checker) callExprType(e *syntax.CallExpr) Type {
	fnType := c.exprType(e.Fn)
	if callable, ok := fnType.(*Callable); ok {
		// Check argument types.
		c.checkCallArgs(e, callable)
		if callable.ReturnType != nil {
			return callable.ReturnType
		}
		return Any
	}
	// Check if extension type is callable via CallableType interface.
	if ct, ok := fnType.(CallableType); ok {
		if sig := ct.CallSignature(); sig != nil {
			c.checkCallArgs(e, sig)
			if sig.ReturnType != nil {
				return sig.ReturnType
			}
		}
	}
	return Any
}

func (c *Checker) dotExprType(e *syntax.DotExpr) Type {
	recvType := c.exprType(e.X)
	name := e.Name.Name

	// Try built-in method tables.
	if methods := c.builtinMethods(recvType); methods != nil {
		if m, ok := methods[name]; ok {
			return m
		}
	}

	// Try HasAttrsType (covers Object, Named, and other types with attrs).
	if hat, ok := recvType.(HasAttrsType); ok {
		if t := hat.AttrType(name); t != nil {
			return t
		}
	}

	// Report error for known types without the attribute.
	if recvType != Any && !isUnknownType(recvType) {
		c.errorf(e.Dot, "type %s has no attribute %s", recvType, name)
	}

	return Any
}

func isUnknownType(t Type) bool {
	switch t.(type) {
	case *Named:
		return true
	}
	return false
}

func (c *Checker) indexExprType(e *syntax.IndexExpr) Type {
	xType := c.exprType(e.X)
	switch t := xType.(type) {
	case *List:
		return t.Elem
	case *Dict:
		return t.Value
	case *Tuple:
		if len(t.Elems) > 0 {
			// For constant integer index, we could return the exact type.
			// For simplicity, return the union of all element types.
			result := t.Elems[0]
			for _, el := range t.Elems[1:] {
				result = c.unify(result, el)
			}
			return result
		}
		return Any
	}
	if xType == String {
		return String
	}
	if xType == Bytes {
		return Int
	}
	// Check IndexableType interface.
	if it, ok := xType.(IndexableType); ok {
		if t := it.ElemType(); t != nil {
			return t
		}
	}
	return Any
}

func (c *Checker) sliceExprType(e *syntax.SliceExpr) Type {
	xType := c.exprType(e.X)
	switch t := xType.(type) {
	case *List:
		return &List{t.Elem}
	case *Tuple:
		return &Tuple{Elems: t.Elems}
	}
	if xType == String {
		return String
	}
	if xType == Bytes {
		return Bytes
	}
	if st, ok := xType.(SliceableType); ok {
		if t := st.SliceResultType(); t != nil {
			return t
		}
	}
	return Any
}

// unify returns a type that encompasses both a and b.
func (c *Checker) unify(a, b Type) Type {
	if a == Any || b == Any {
		return Any
	}
	if a == b {
		return a
	}
	if Assignable(a, b) {
		return b
	}
	if Assignable(b, a) {
		return a
	}
	return &Union{Types: []Type{a, b}}
}
