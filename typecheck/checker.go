package typecheck

import (
	"fmt"

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
	// Names maps predeclared/universal names to their types.
	Names map[string]Type

	// TypeDescriptors maps type names (as returned by Value.Type()) to
	// descriptors that provide attribute, method, and operator type info.
	TypeDescriptors map[string]*TypeDescriptor

	// Load resolves type information for a loaded module.
	// It receives the module string from the load() statement and returns
	// a mapping of exported names to their types, or nil if unknown.
	Load func(module string) map[string]Type
}

// TypeDescriptor describes the static type information for a Go-defined
// Starlark type.
type TypeDescriptor struct {
	// Read-side operations:
	Attrs     map[string]Type       // attribute name → type (HasAttrs)
	Methods   map[string]*Callable  // method name → signature (HasAttrs)
	BinaryOps map[syntax.Token]Type // op → result type (HasBinary)
	UnaryOps  map[syntax.Token]Type // op → result type (HasUnary)
	CallSig   *Callable             // call signature if Callable
	IndexType Type                  // result of x[i] (Indexable)
	SliceType Type                  // result of x[i:j] (Sliceable); nil = not sliceable
	IterElem  Type                  // element type (Iterable)

	// Write-side operations:
	SetIndexType  Type            // type accepted by x[i]=v (HasSetIndex); nil = not settable
	KeyType       Type            // key type for mapping get/set (Mapping)
	ValueType     Type            // value type for mapping get/set (Mapping/HasSetKey)
	SetFieldTypes map[string]Type // field name → accepted type (HasSetField); nil = not settable

	// Capabilities:
	Comparable bool // whether values support ordering operators (<, >, <=, >=)
	Sequence   bool // whether value is a finite-length sequence
}

// scope represents a lexical scope mapping names to bindings.
type scope struct {
	parent   *scope
	bindings map[string]*Binding
}

func (s *scope) lookup(name string) *Binding {
	for sc := s; sc != nil; sc = sc.parent {
		if b, ok := sc.bindings[name]; ok {
			return b
		}
	}
	return nil
}

func (s *scope) lookupType(name string) Type {
	if b := s.lookup(name); b != nil {
		return b.Type
	}
	return nil
}

func (s *scope) set(b *Binding) {
	if s.bindings == nil {
		s.bindings = make(map[string]*Binding)
	}
	s.bindings[b.Name] = b
}

// Checker performs static type checking on resolved Starlark ASTs.
type Checker struct {
	env    *scope    // current type environment
	errors []Error   // accumulated type errors
	fn     *Callable // current function (for return type checks)
	tenv   *Env      // type environment for extension type lookups
	info   *Info     // type info to populate (may be nil)
}

func (c *Checker) errorf(pos syntax.Position, format string, args ...interface{}) {
	c.errors = append(c.errors, Error{pos, fmt.Sprintf(format, args...)})
}

func (c *Checker) pushScope() {
	c.env = &scope{parent: c.env}
}

func (c *Checker) popScope() {
	c.env = c.env.parent
}

// lookupDescriptor returns the TypeDescriptor for a type, if available.
func (c *Checker) lookupDescriptor(t Type) *TypeDescriptor {
	if c.tenv == nil || c.tenv.TypeDescriptors == nil {
		return nil
	}
	switch t := t.(type) {
	case *Named:
		if desc, ok := c.tenv.TypeDescriptors[t.Name]; ok {
			return desc
		}
	case *Object:
		if desc, ok := c.tenv.TypeDescriptors[t.Name]; ok {
			return desc
		}
	}
	return nil
}

// Check type-checks a resolved file and returns any type errors.
// If info is non-nil, the checker populates its non-nil maps with type information.
func Check(file *syntax.File, env *Env, info *Info) []Error {
	if env == nil {
		env = StandardEnv()
	}

	c := &Checker{
		env:  &scope{},
		tenv: env,
		info: info,
	}

	// Populate the base scope with predeclared names.
	if env.Names != nil {
		for name, t := range env.Names {
			c.defineName(name, t)
		}
	}

	// Check each statement.
	c.stmts(file.Stmts)

	if len(c.errors) == 0 {
		return nil
	}
	return c.errors
}

// define creates a Binding for id with the given type, adds it to the current
// scope, and records it in info.Defs if the map is non-nil.
func (c *Checker) define(id *syntax.Ident, typ Type) *Binding {
	b := &Binding{Pos: id.NamePos, Name: id.Name, Type: typ}
	c.env.set(b)
	if c.info != nil && c.info.Defs != nil {
		c.info.Defs[id] = b
	}
	return b
}

// defineName creates a Binding for a predeclared name (no AST node).
func (c *Checker) defineName(name string, typ Type) *Binding {
	b := &Binding{Name: name, Type: typ}
	c.env.set(b)
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
		c.exprType(s.X)
		c.pushScope()
		c.bindForVars(s.Vars, c.exprType(s.X))
		c.stmts(s.Body)
		c.popScope()

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
		} else if desc := c.lookupDescriptor(iterType); desc != nil && desc.IterElem != nil {
			elemType = desc.IterElem
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
				c.define(id, declaredType)
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
				existingType := c.env.lookupType(id.Name)
				if existingType != nil && existingType != Any {
					// Check that the result is assignable.
					resultType := c.binaryExprType(&syntax.BinaryExpr{
						X:  s.LHS,
						Op: augmentedToOp(s.Op),
						Y:  s.RHS,
					})
					_ = lhsType
					_ = rhsType
					if !Assignable(resultType, existingType) {
						c.errorf(s.OpPos, "cannot use %s as %s", resultType, existingType)
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
		existingType := c.env.lookupType(lhs.Name)
		if existingType != nil && existingType != Any {
			if !Assignable(rhsType, existingType) {
				c.errorf(lhs.NamePos, "cannot use %s as %s", rhsType, existingType)
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
			if desc := c.lookupDescriptor(xType); desc != nil {
				if desc.ValueType != nil {
					// Mapping-style key assignment (HasSetKey).
					if !Assignable(rhsType, desc.ValueType) {
						pos, _ := lhs.Span()
						c.errorf(pos, "cannot use %s as %s", rhsType, desc.ValueType)
					}
				} else if desc.SetIndexType != nil {
					// Sequence-style index assignment (HasSetIndex).
					if !Assignable(rhsType, desc.SetIndexType) {
						pos, _ := lhs.Span()
						c.errorf(pos, "cannot use %s as %s", rhsType, desc.SetIndexType)
					}
				}
			}
		}
	case *syntax.DotExpr:
		recvType := c.exprType(lhs.X)
		name := lhs.Name.Name
		// Validate the assigned value's type against the field's declared type.
		if obj, ok := recvType.(*Object); ok {
			if attrType, ok := obj.Attrs[name]; ok {
				if !Assignable(rhsType, attrType) {
					c.errorf(lhs.Name.NamePos, "cannot use %s as %s in field assignment", rhsType, attrType)
				}
			}
		} else if desc := c.lookupDescriptor(recvType); desc != nil {
			if desc.SetFieldTypes != nil {
				if fieldType, ok := desc.SetFieldTypes[name]; ok {
					if !Assignable(rhsType, fieldType) {
						c.errorf(lhs.Name.NamePos, "cannot use %s as %s in field assignment", rhsType, fieldType)
					}
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

	// Register the function in the current scope.
	c.define(s.Name, callable)

	// Check the function body in a new scope.
	c.pushScope()
	oldFn := c.fn
	c.fn = callable

	// Bind parameters in the function scope.
	// Walk AST params to extract *syntax.Ident nodes for Defs recording.
	for _, param := range s.Params {
		c.defineParam(param)
	}

	c.stmts(s.Body)

	c.fn = oldFn
	c.popScope()
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
			c.define(inner, typ)
		case *syntax.UnaryExpr:
			if inner.X != nil {
				if id, ok := inner.X.(*syntax.Ident); ok {
					c.define(id, typ)
				}
			}
		}

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
				c.define(id, typ)
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
