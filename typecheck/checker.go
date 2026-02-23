package typecheck

import (
	"fmt"

	"github.com/pgavlin/starlark-go/resolve"
	"github.com/pgavlin/starlark-go/syntax"
)

// Error describes a type-checking error.
type Error struct {
	Pos syntax.Position
	Msg string
}

func (e Error) Error() string {
	return e.Pos.String() + ": " + e.Msg
}

// Env provides type information for predeclared names and extension types.
type Env struct {
	// Predeclared maps predeclared names to their types.
	// Extension types should be registered here under both their type name
	// (for use in annotations) and variable names (for bindings).
	Predeclared map[string]Type

	// Load resolves type information for a loaded module.
	// It receives the module string from the load() statement and returns
	// a mapping of exported names to their types, or nil if unknown.
	Load func(module string) map[string]Type
}

// Checker performs static type checking on resolved Starlark ASTs.
type Checker struct {
	bindings map[*resolve.Binding]*Binding // type for each resolve.Binding
	errors   []Error                       // accumulated type errors
	fn       *Callable                     // current function (for return type checks)
	tenv     *Env                          // type environment for extension type lookups
	info     *Info                         // type info to populate (may be nil)
}

func (c *Checker) errorf(pos syntax.Position, format string, args ...interface{}) {
	c.errors = append(c.errors, Error{pos, fmt.Sprintf(format, args...)})
}

// lookupBinding returns the type binding for the given identifier, lazily
// populating predeclared and universal bindings on first access.
func (c *Checker) lookupBinding(id *syntax.Ident) *Binding {
	rb, ok := id.Binding.(*resolve.Binding)
	if !ok {
		return nil
	}
	if b, ok := c.bindings[rb]; ok {
		return b
	}
	// Lazy populate predeclared/universal types.
	var typ Type
	switch rb.Scope {
	case resolve.Predeclared:
		if c.tenv != nil && c.tenv.Predeclared != nil {
			typ = c.tenv.Predeclared[id.Name]
		}
	case resolve.Universal:
		typ = Universe[id.Name]
	}
	if typ == nil {
		return nil
	}
	b := &Binding{Name: id.Name, Type: typ}
	c.bindings[rb] = b
	return b
}

// Check type-checks a resolved file and returns any type errors.
// If info is non-nil, the checker populates its non-nil maps with type information.
func Check(file *syntax.File, env *Env, info *Info) []Error {
	c := &Checker{
		bindings: make(map[*resolve.Binding]*Binding),
		tenv:     env,
		info:     info,
	}

	// Check each statement.
	c.stmts(file.Stmts)

	if len(c.errors) == 0 {
		return nil
	}
	return c.errors
}

// define creates a Binding for id with the given type, stores it keyed on
// the identifier's resolve.Binding, and records it in info.Defs if non-nil.
func (c *Checker) define(id *syntax.Ident, typ Type) *Binding {
	b := &Binding{Pos: id.NamePos, Name: id.Name, Type: typ}
	if rb, ok := id.Binding.(*resolve.Binding); ok {
		c.bindings[rb] = b
	}
	if c.info != nil && c.info.Defs != nil {
		c.info.Defs[id] = b
	}
	return b
}

// record records the type of an expression in info.Types if the map is non-nil.
func (c *Checker) record(expr syntax.Expr, typ Type) Type {
	if c.info != nil && c.info.Types != nil {
		c.info.Types[expr] = TypeAndValue{Type: typ}
	}
	return typ
}

func (c *Checker) stmts(stmts []syntax.Stmt) {
	for _, stmt := range stmts {
		c.stmt(stmt)
	}
}

func (c *Checker) stmt(stmt syntax.Stmt) {
	switch s := stmt.(type) {
	case *syntax.ExprStmt:
		c.exprType(s.X)

	case *syntax.AssignStmt:
		c.checkAssignStmt(s)

	case *syntax.DefStmt:
		c.checkDefStmt(s)

	case *syntax.ReturnStmt:
		c.checkReturnStmt(s)

	case *syntax.ForStmt:
		iterType := c.exprType(s.X)
		c.bindForVars(s.Vars, iterType)
		c.stmts(s.Body)

	case *syntax.WhileStmt:
		c.exprType(s.Cond)
		c.stmts(s.Body)

	case *syntax.IfStmt:
		c.exprType(s.Cond)
		c.stmts(s.True)
		c.stmts(s.False)

	case *syntax.LoadStmt:
		// Resolve module types via callback if available.
		var moduleTypes map[string]Type
		if c.tenv != nil && c.tenv.Load != nil {
			moduleTypes = c.tenv.Load(s.ModuleName())
		}
		for i, to := range s.To {
			typ := Type(Any)
			if moduleTypes != nil {
				if t, ok := moduleTypes[s.From[i].Name]; ok {
					typ = t
				}
			}
			c.define(to, typ)
		}

	case *syntax.BranchStmt:
		// no-op
	}
}

func (c *Checker) bindForVars(vars syntax.Expr, iterType Type) {
	// Extract element type from iterable.
	var elemType Type
	switch t := iterType.(type) {
	case *List:
		elemType = t.Elem
	case *Dict:
		elemType = t.Key
	case *Set:
		elemType = t.Elem
	case *Tuple:
		if len(t.Elems) > 0 {
			elemType = t.Elems[0]
			for _, el := range t.Elems[1:] {
				elemType = c.unify(elemType, el)
			}
		} else {
			elemType = Any
		}
	default:
		if iterType == String {
			elemType = String
		} else if iterType == Bytes {
			elemType = Int
		} else if it, ok := iterType.(IterableType); ok && it.IterElemType() != nil {
			elemType = it.IterElemType()
		} else {
			elemType = Any
		}
	}

	switch v := vars.(type) {
	case *syntax.Ident:
		c.define(v, elemType)
	case *syntax.TupleExpr:
		for _, el := range v.List {
			if id, ok := el.(*syntax.Ident); ok {
				c.define(id, Any) // conservative
			}
		}
	case *syntax.ParenExpr:
		c.bindForVars(v.X, iterType)
	}
}

func (c *Checker) checkAssignStmt(s *syntax.AssignStmt) {
	if s.Op == syntax.EQ {
		if s.TypeExpr != nil {
			// Typed assignment: x: type = expr
			declaredType := c.evalType(s.TypeExpr)
			if s.RHS != nil {
				rhsType := c.exprType(s.RHS)
				if !Assignable(rhsType, declaredType) {
					pos, _ := s.RHS.Span()
					c.errorf(pos, "cannot use %s as %s", rhsType, declaredType)
				}
			}
			if id, ok := s.LHS.(*syntax.Ident); ok {
				b := c.define(id, declaredType)
				b.Declared = true
			}
		} else if s.RHS != nil {
			// Simple assignment: x = expr
			rhsType := c.exprType(s.RHS)
			c.bindAssign(s.LHS, rhsType)
		}
	} else {
		// Augmented assignment: x += expr, etc.
		if s.RHS != nil {
			lhsType := c.exprType(s.LHS)
			rhsType := c.exprType(s.RHS)
			// Check if the existing variable has a declared type.
			if id, ok := s.LHS.(*syntax.Ident); ok {
				if b := c.lookupBinding(id); b != nil && b.Declared && b.Type != Any {
					// Check that the result is assignable.
					resultType := c.binaryExprType(&syntax.BinaryExpr{
						X:  s.LHS,
						Op: augmentedToOp(s.Op),
						Y:  s.RHS,
					})
					_ = lhsType
					_ = rhsType
					if !Assignable(resultType, b.Type) {
						c.errorf(s.OpPos, "cannot use %s as %s", resultType, b.Type)
					}
				}
			}
		}
	}
}

func augmentedToOp(op syntax.Token) syntax.Token {
	switch op {
	case syntax.PLUS_EQ:
		return syntax.PLUS
	case syntax.MINUS_EQ:
		return syntax.MINUS
	case syntax.STAR_EQ:
		return syntax.STAR
	case syntax.SLASH_EQ:
		return syntax.SLASH
	case syntax.SLASHSLASH_EQ:
		return syntax.SLASHSLASH
	case syntax.PERCENT_EQ:
		return syntax.PERCENT
	case syntax.AMP_EQ:
		return syntax.AMP
	case syntax.PIPE_EQ:
		return syntax.PIPE
	case syntax.CIRCUMFLEX_EQ:
		return syntax.CIRCUMFLEX
	case syntax.LTLT_EQ:
		return syntax.LTLT
	case syntax.GTGT_EQ:
		return syntax.GTGT
	}
	return op
}

func (c *Checker) bindAssign(lhs syntax.Expr, rhsType Type) {
	switch lhs := lhs.(type) {
	case *syntax.Ident:
		// Check if the variable was previously declared with a type.
		if b := c.lookupBinding(lhs); b != nil && b.Declared && b.Type != Any {
			if !Assignable(rhsType, b.Type) {
				c.errorf(lhs.NamePos, "cannot use %s as %s", rhsType, b.Type)
			}
			// Keep the declared type.
		} else {
			c.define(lhs, rhsType)
		}
	case *syntax.TupleExpr:
		for _, el := range lhs.List {
			c.bindAssign(el, Any) // conservative
		}
	case *syntax.ListExpr:
		for _, el := range lhs.List {
			c.bindAssign(el, Any)
		}
	case *syntax.ParenExpr:
		c.bindAssign(lhs.X, rhsType)
	case *syntax.IndexExpr:
		xType := c.exprType(lhs.X)
		c.exprType(lhs.Y)
		// Validate the assigned value's type against the container's element/value type.
		switch t := xType.(type) {
		case *List:
			if !Assignable(rhsType, t.Elem) {
				pos, _ := lhs.Span()
				c.errorf(pos, "cannot use %s as %s in list assignment", rhsType, t.Elem)
			}
		case *Dict:
			if !Assignable(rhsType, t.Value) {
				pos, _ := lhs.Span()
				c.errorf(pos, "cannot use %s as %s in dict assignment", rhsType, t.Value)
			}
		default:
			if mt, ok := xType.(MappingType); ok && mt.MappingValueType() != nil {
				// Mapping-style key assignment.
				vt := mt.MappingValueType()
				if !Assignable(rhsType, vt) {
					pos, _ := lhs.Span()
					c.errorf(pos, "cannot use %s as %s", rhsType, vt)
				}
			} else if si, ok := xType.(HasSetIndexType); ok && si.SetIndexType() != nil {
				// Sequence-style index assignment.
				st := si.SetIndexType()
				if !Assignable(rhsType, st) {
					pos, _ := lhs.Span()
					c.errorf(pos, "cannot use %s as %s", rhsType, st)
				}
			}
		}
	case *syntax.DotExpr:
		recvType := c.exprType(lhs.X)
		name := lhs.Name.Name
		// Validate the assigned value's type against the field's declared type.
		if hsf, ok := recvType.(HasSetFieldType); ok {
			if fieldType := hsf.SetFieldType(name); fieldType != nil {
				if !Assignable(rhsType, fieldType) {
					c.errorf(lhs.Name.NamePos, "cannot use %s as %s in field assignment", rhsType, fieldType)
				}
			}
		}
	}
}

func (c *Checker) checkDefStmt(s *syntax.DefStmt) {
	// Build callable type.
	var params []Param
	for _, param := range s.Params {
		params = append(params, c.paramType(param)...)
	}

	var returnType Type
	if s.ResultType != nil {
		returnType = c.evalType(s.ResultType)
	} else {
		returnType = Any
	}

	callable := &Callable{
		Name:       s.Name.Name,
		Params:     params,
		ReturnType: returnType,
	}

	// Register the function in the enclosing scope.
	c.define(s.Name, callable)

	// Check the function body.
	oldFn := c.fn
	c.fn = callable

	// Bind parameters in the function scope.
	// Walk AST params to extract *syntax.Ident nodes for Defs recording.
	for _, param := range s.Params {
		c.defineParam(param)
	}

	c.stmts(s.Body)

	c.fn = oldFn
}

// defineParam extracts the identifier from a parameter AST node and defines it.
func (c *Checker) defineParam(param syntax.Expr) {
	switch p := param.(type) {
	case *syntax.Ident:
		c.define(p, Any)

	case *syntax.TypeAnnotatedExpr:
		typ := c.evalType(p.Type)
		switch inner := p.X.(type) {
		case *syntax.Ident:
			b := c.define(inner, typ)
			b.Declared = true
		case *syntax.UnaryExpr:
			if inner.X != nil {
				if id, ok := inner.X.(*syntax.Ident); ok {
					b := c.define(id, typ)
					b.Declared = true
				}
			}
		}

	case *syntax.BinaryExpr:
		if p.Op == syntax.EQ {
			// param = default  or  param: type = default
			x := p.X
			var typ Type
			var declared bool
			if ta, ok := x.(*syntax.TypeAnnotatedExpr); ok {
				typ = c.evalType(ta.Type)
				x = ta.X
				declared = true
			} else {
				typ = Any
			}
			if id, ok := x.(*syntax.Ident); ok {
				b := c.define(id, typ)
				b.Declared = declared
			}
		}

	case *syntax.UnaryExpr:
		if p.X != nil {
			if id, ok := p.X.(*syntax.Ident); ok {
				c.define(id, Any)
			}
		}
	}
}

func (c *Checker) paramType(param syntax.Expr) []Param {
	switch p := param.(type) {
	case *syntax.Ident:
		return []Param{{Name: p.Name, Type: Any}}

	case *syntax.TypeAnnotatedExpr:
		typ := c.evalType(p.Type)
		switch inner := p.X.(type) {
		case *syntax.Ident:
			return []Param{{Name: inner.Name, Type: typ}}
		case *syntax.UnaryExpr:
			if inner.Op == syntax.STAR {
				if inner.X != nil {
					id := inner.X.(*syntax.Ident)
					return []Param{{Name: id.Name, Type: typ, Star: true}}
				}
				return nil
			} else if inner.Op == syntax.STARSTAR {
				id := inner.X.(*syntax.Ident)
				return []Param{{Name: id.Name, Type: typ, StarStar: true}}
			}
		}
		return []Param{{Type: typ}}

	case *syntax.BinaryExpr:
		if p.Op == syntax.EQ {
			// param = default  or  param: type = default
			x := p.X
			var typ Type
			if ta, ok := x.(*syntax.TypeAnnotatedExpr); ok {
				typ = c.evalType(ta.Type)
				x = ta.X
			} else {
				typ = Any
			}
			if id, ok := x.(*syntax.Ident); ok {
				return []Param{{Name: id.Name, Type: typ, Optional: true}}
			}
		}
		return nil

	case *syntax.UnaryExpr:
		if p.Op == syntax.STAR {
			if p.X != nil {
				id := p.X.(*syntax.Ident)
				return []Param{{Name: id.Name, Type: Any, Star: true}}
			}
			return nil // bare *
		} else if p.Op == syntax.STARSTAR {
			id := p.X.(*syntax.Ident)
			return []Param{{Name: id.Name, Type: Any, StarStar: true}}
		}
		return nil
	}
	return nil
}

func (c *Checker) checkReturnStmt(s *syntax.ReturnStmt) {
	if c.fn == nil {
		return // return outside function — resolver catches this
	}

	if s.Result == nil {
		// return without value is always OK (returns None).
		if c.fn.ReturnType != nil && c.fn.ReturnType != Any && c.fn.ReturnType != None {
			c.errorf(s.Return, "cannot use %s as %s", None, c.fn.ReturnType)
		}
		return
	}

	resultType := c.exprType(s.Result)
	if c.fn.ReturnType != nil && c.fn.ReturnType != Any {
		if !Assignable(resultType, c.fn.ReturnType) {
			pos, _ := s.Result.Span()
			c.errorf(pos, "cannot use %s as %s", resultType, c.fn.ReturnType)
		}
	}
}

// checkCallArgs checks the argument types of a function call.
func (c *Checker) checkCallArgs(call *syntax.CallExpr, callable *Callable) {
	if callable == nil || len(callable.Params) == 0 {
		return
	}

	// Simple positional argument checking.
	paramIdx := 0
	for _, arg := range call.Args {
		switch a := arg.(type) {
		case *syntax.UnaryExpr:
			// *args or **kwargs — skip checking
			continue
		case *syntax.BinaryExpr:
			if a.Op == syntax.EQ {
				// keyword argument: name=value
				name := a.X.(*syntax.Ident).Name
				argType := c.exprType(a.Y)
				// Find the parameter by name.
				for _, p := range callable.Params {
					if p.Name == name && p.Type != Any {
						if !Assignable(argType, p.Type) {
							pos, _ := a.Y.Span()
							c.errorf(pos, "cannot use %s as %s", argType, p.Type)
						}
						break
					}
				}
				continue
			}
		}

		// Positional argument.
		argType := c.exprType(arg)
		for paramIdx < len(callable.Params) && (callable.Params[paramIdx].Star || callable.Params[paramIdx].StarStar) {
			paramIdx++
		}
		if paramIdx < len(callable.Params) {
			p := callable.Params[paramIdx]
			if p.Type != Any && !Assignable(argType, p.Type) {
				pos, _ := arg.Span()
				c.errorf(pos, "cannot use %s as %s", argType, p.Type)
			}
			paramIdx++
		}
	}
}
