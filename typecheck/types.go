// Package typecheck provides static type checking for Starlark programs.
package typecheck

import (
	"fmt"
	"strings"

	"github.com/pgavlin/starlark-go/syntax"
)

// Type represents a Starlark type.
type Type interface {
	String() string
	typeNode()
}

// Built-in singleton types.
var (
	// None is the type of Starlark's None value.
	None Type = &basicType{"None"}
	// Bool is the type of Starlark's True and False values.
	Bool Type = &basicType{"bool"}
	// Int is the type of Starlark integer values.
	Int Type = &basicType{"int"}
	// Float is the type of Starlark floating-point values.
	Float Type = &basicType{"float"}
	// String is the type of Starlark string values.
	String Type = &basicType{"string"}
	// Bytes is the type of Starlark bytes values.
	Bytes Type = &basicType{"bytes"}
	// Any is the type that matches any Starlark value. It is used for
	// unannotated variables and as a fallback when type inference fails.
	Any Type = &basicType{"any"}
)

type basicType struct {
	name string
}

func (t *basicType) String() string { return t.name }
func (t *basicType) typeNode()      {}

// List represents the type list[Elem].
type List struct {
	Elem Type
}

func (t *List) String() string { return fmt.Sprintf("list[%s]", t.Elem) }
func (t *List) typeNode()      {}

// Dict represents the type dict[Key, Value].
type Dict struct {
	Key   Type
	Value Type
}

func (t *Dict) String() string { return fmt.Sprintf("dict[%s, %s]", t.Key, t.Value) }
func (t *Dict) typeNode()      {}

// Tuple represents tuple[T1, T2, ...].
type Tuple struct {
	Elems []Type
}

func (t *Tuple) String() string {
	if len(t.Elems) == 0 {
		return "tuple"
	}
	parts := make([]string, len(t.Elems))
	for i, e := range t.Elems {
		parts[i] = e.String()
	}
	return fmt.Sprintf("tuple[%s]", strings.Join(parts, ", "))
}
func (t *Tuple) typeNode() {}

// Set represents the type set[Elem].
type Set struct {
	Elem Type
}

func (t *Set) String() string { return fmt.Sprintf("set[%s]", t.Elem) }
func (t *Set) typeNode()      {}

// Union represents T1 | T2 | ...
type Union struct {
	Types []Type
}

func (t *Union) String() string {
	parts := make([]string, len(t.Types))
	for i, typ := range t.Types {
		parts[i] = typ.String()
	}
	return strings.Join(parts, " | ")
}
func (t *Union) typeNode() {}

// Callable represents a function type.
type Callable struct {
	Name       string
	Params     []Param
	ReturnType Type
}

func (t *Callable) String() string {
	parts := make([]string, len(t.Params))
	for i, p := range t.Params {
		parts[i] = p.String()
	}
	ret := ""
	if t.ReturnType != nil {
		ret = " -> " + t.ReturnType.String()
	}
	if t.Name != "" {
		return fmt.Sprintf("%s(%s)%s", t.Name, strings.Join(parts, ", "), ret)
	}
	return fmt.Sprintf("(%s)%s", strings.Join(parts, ", "), ret)
}
func (t *Callable) typeNode() {}

// Param represents a function parameter type.
type Param struct {
	Name     string
	Type     Type
	Optional bool
	Star     bool // *args
	StarStar bool // **kwargs
}

func (p Param) String() string {
	prefix := ""
	if p.Star {
		prefix = "*"
	} else if p.StarStar {
		prefix = "**"
	}
	suffix := ""
	if p.Optional {
		suffix = "?"
	}
	if p.Type != nil {
		return fmt.Sprintf("%s%s: %s%s", prefix, p.Name, p.Type, suffix)
	}
	return prefix + p.Name + suffix
}

// Object represents a value with a known set of attributes and methods.
type Object struct {
	Name    string
	Attrs   map[string]Type
	Methods map[string]*Callable
}

func (t *Object) String() string { return t.Name }
func (t *Object) typeNode()      {}

// AttrType implements HasAttrsType for Object.
func (t *Object) AttrType(name string) Type {
	if t.Attrs != nil {
		if typ, ok := t.Attrs[name]; ok {
			return typ
		}
	}
	if t.Methods != nil {
		if m, ok := t.Methods[name]; ok {
			return m
		}
	}
	return nil
}

// SetFieldType implements HasSetFieldType for Object.
// Object uses Attrs for both read and write.
func (t *Object) SetFieldType(name string) Type {
	if t.Attrs != nil {
		if typ, ok := t.Attrs[name]; ok {
			return typ
		}
	}
	return nil
}

// HasAttrsType mirrors starlark.HasAttrs — type has typed attributes/methods.
type HasAttrsType interface {
	Type
	AttrType(name string) Type // nil if not found
}

// HasSetFieldType mirrors starlark.HasSetField — type supports field assignment.
type HasSetFieldType interface {
	Type
	SetFieldType(name string) Type // nil if not settable
}

// HasBinaryType mirrors starlark.HasBinary — typed binary operations.
type HasBinaryType interface {
	Type
	BinaryType(op syntax.Token) Type // nil if not supported
}

// HasUnaryType mirrors starlark.HasUnary — typed unary operations.
type HasUnaryType interface {
	Type
	UnaryType(op syntax.Token) Type // nil if not supported
}

// CallableType mirrors starlark.Callable — type can be called.
type CallableType interface {
	Type
	CallSignature() *Callable
}

// IndexableType mirrors starlark.Indexable — supports x[i].
type IndexableType interface {
	Type
	ElemType() Type
}

// SliceableType mirrors starlark.Sliceable — supports x[i:j].
type SliceableType interface {
	Type
	SliceResultType() Type
}

// IterableType mirrors starlark.Iterable — can be iterated.
type IterableType interface {
	Type
	IterElemType() Type
}

// HasSetIndexType mirrors starlark.HasSetIndex — supports x[i]=v.
type HasSetIndexType interface {
	Type
	SetIndexType() Type
}

// MappingType mirrors starlark.Mapping — has key-value get/set semantics.
type MappingType interface {
	Type
	MappingValueType() Type
}

// ComparableType mirrors starlark.Comparable — supports ordering.
type ComparableType interface {
	Type
	IsComparable() bool
}

// Named represents an extension type with optional typed capabilities.
type Named struct {
	Name       string
	Attrs      map[string]Type
	Methods    map[string]*Callable
	BinaryOps  map[syntax.Token]Type
	UnaryOps   map[syntax.Token]Type
	CallSig    *Callable
	Index      Type            // ElemType for x[i]
	Slice      Type            // result of x[i:j]
	IterElem   Type            // element type for iteration
	SetIndex   Type            // accepted type for x[i]=v (sequence-style)
	SetKey     Type            // accepted value type for x[k]=v (mapping-style)
	SetFields  map[string]Type // accepted types for x.field=v
	Comparable *bool           // nil=unknown (assume comparable), &true=ordered, &false=not ordered
}

func (t *Named) String() string { return t.Name }
func (t *Named) typeNode()      {}

// AttrType implements HasAttrsType.
func (t *Named) AttrType(name string) Type {
	if t.Attrs != nil {
		if typ, ok := t.Attrs[name]; ok {
			return typ
		}
	}
	if t.Methods != nil {
		if m, ok := t.Methods[name]; ok {
			return m
		}
	}
	return nil
}

// SetFieldType implements HasSetFieldType.
func (t *Named) SetFieldType(name string) Type {
	if t.SetFields != nil {
		if typ, ok := t.SetFields[name]; ok {
			return typ
		}
	}
	return nil
}

// BinaryType implements HasBinaryType.
func (t *Named) BinaryType(op syntax.Token) Type {
	if t.BinaryOps != nil {
		if typ, ok := t.BinaryOps[op]; ok {
			return typ
		}
	}
	return nil
}

// UnaryType implements HasUnaryType.
func (t *Named) UnaryType(op syntax.Token) Type {
	if t.UnaryOps != nil {
		if typ, ok := t.UnaryOps[op]; ok {
			return typ
		}
	}
	return nil
}

// CallSignature implements CallableType.
func (t *Named) CallSignature() *Callable {
	return t.CallSig
}

// ElemType implements IndexableType.
func (t *Named) ElemType() Type {
	return t.Index
}

// SliceResultType implements SliceableType.
func (t *Named) SliceResultType() Type {
	return t.Slice
}

// IterElemType implements IterableType.
func (t *Named) IterElemType() Type {
	return t.IterElem
}

// SetIndexType implements HasSetIndexType.
func (t *Named) SetIndexType() Type {
	return t.SetIndex
}

// MappingValueType implements MappingType.
func (t *Named) MappingValueType() Type {
	return t.SetKey
}

// IsComparable implements ComparableType.
// Returns true if Comparable is nil (unknown, assume comparable) or *true.
func (t *Named) IsComparable() bool {
	return t.Comparable == nil || *t.Comparable
}

// TypeAndValue reports the type of an expression.
type TypeAndValue struct {
	Type Type
}

// Binding represents a named Starlark entity such as a variable,
// function, parameter, or predeclared name.
type Binding struct {
	Pos  syntax.Position // definition position (zero for predeclared)
	Name string
	Type Type
}

// Info holds the type information resulting from type-checking a Starlark file.
// Only non-nil maps are populated during type-checking.
type Info struct {
	// Types maps expressions to their types. Use-site identifiers appear
	// in both Types and Uses.
	Types map[syntax.Expr]TypeAndValue

	// Defs maps identifiers at definition sites to their bindings.
	// This includes: assignment targets, def statement names, function
	// parameters, for-loop variables, and load statement bindings.
	Defs map[*syntax.Ident]*Binding

	// Uses maps identifiers at use sites to the bindings they reference.
	Uses map[*syntax.Ident]*Binding
}

// TypeOf returns the type of expression e, or nil if unknown.
// For identifiers, it checks Types first, then Defs, then Uses.
func (info *Info) TypeOf(e syntax.Expr) Type {
	if info == nil {
		return nil
	}
	if info.Types != nil {
		if tv, ok := info.Types[e]; ok {
			return tv.Type
		}
	}
	if id, ok := e.(*syntax.Ident); ok {
		if b := info.BindingOf(id); b != nil {
			return b.Type
		}
	}
	return nil
}

// BindingOf returns the binding for identifier id, checking Defs first, then Uses.
func (info *Info) BindingOf(id *syntax.Ident) *Binding {
	if info == nil {
		return nil
	}
	if info.Defs != nil {
		if b, ok := info.Defs[id]; ok {
			return b
		}
	}
	if info.Uses != nil {
		if b, ok := info.Uses[id]; ok {
			return b
		}
	}
	return nil
}

// Assignable reports whether a value of type src can be assigned to a variable of type dst.
func Assignable(src, dst Type) bool {
	// Any is assignable to/from anything.
	if src == Any || dst == Any {
		return true
	}

	// Identical basic types.
	if src == dst {
		return true
	}

	// Union types.
	if u, ok := dst.(*Union); ok {
		// src is assignable to union if src is assignable to any member.
		for _, t := range u.Types {
			if Assignable(src, t) {
				return true
			}
		}
		return false
	}

	switch src := src.(type) {
	case *Union:
		// union is assignable to dst if all members are assignable to dst.
		for _, t := range src.Types {
			if !Assignable(t, dst) {
				return false
			}
		}
		return true
	case *List:
		// List covariance.
		if dl, ok := dst.(*List); ok {
			return Assignable(src.Elem, dl.Elem)
		}
	case *Dict:
		// Dict covariance.
		if dd, ok := dst.(*Dict); ok {
			return Assignable(src.Key, dd.Key) && Assignable(src.Value, dd.Value)
		}
	case *Set:
		// Set covariance.
		if ds, ok := dst.(*Set); ok {
			return Assignable(src.Elem, ds.Elem)
		}
	case *Named:
		// Named types match by name.
		if dn, ok := dst.(*Named); ok {
			return src.Name == dn.Name
		}
	case *Object:
		// Object types match by name.
		if do, ok := dst.(*Object); ok {
			if src.Name == do.Name {
				return true
			}
			// Structural matching: src has all attrs of dst.
			for name, dstType := range do.Attrs {
				srcType, ok := src.Attrs[name]
				if !ok || !Assignable(srcType, dstType) {
					return false
				}
			}
			return true
		}
	case *Callable:
		// Callable assignability: same structure.
		if dc, ok := dst.(*Callable); ok {
			if src.ReturnType != nil && dc.ReturnType != nil {
				if !Assignable(src.ReturnType, dc.ReturnType) {
					return false
				}
			}
			return true // conservative: don't check param types
		}
	}

	return false
}
