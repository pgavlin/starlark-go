package typecheck_test

import (
	"strings"
	"testing"

	"github.com/pgavlin/starlark-go/resolve"
	"github.com/pgavlin/starlark-go/syntax"
	"github.com/pgavlin/starlark-go/typecheck"
)

func check(t *testing.T, src string, env *typecheck.Env) []typecheck.Error {
	t.Helper()
	f, err := syntax.Parse("test.star", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if err := resolve.File(f, func(name string) bool {
		if env != nil {
			if _, ok := env.Predeclared[name]; ok {
				return true
			}
		}
		return false
	}, func(name string) bool {
		_, ok := typecheck.Universe[name]
		return ok
	}); err != nil {
		t.Fatalf("resolve error: %v", err)
	}
	return typecheck.Check(f, env, nil)
}

func TestTypedAssignments(t *testing.T) {
	tests := []struct {
		src     string
		wantErr string
	}{
		// OK cases
		{"x: int = 5", ""},
		{"x: str = 'hello'", ""},
		{"x: bool = True", ""},
		{"x: float = 1.5", ""},
		{"x: list = [1, 2, 3]", ""},
		// Union types
		{"x: int = 5", ""},
		// Error cases
		{"x: int = 'hello'", "cannot use string as int"},
		{"x: str = 5", "cannot use int as string"},
		{"x: bool = 5", "cannot use int as bool"},
	}

	for _, test := range tests {
		errs := check(t, test.src, nil)
		if test.wantErr == "" {
			if len(errs) > 0 {
				t.Errorf("check(%q): unexpected errors: %v", test.src, errs)
			}
		} else {
			if len(errs) == 0 {
				t.Errorf("check(%q): expected error containing %q, got none", test.src, test.wantErr)
			} else if !containsError(errs, test.wantErr) {
				t.Errorf("check(%q): expected error containing %q, got %v", test.src, test.wantErr, errs)
			}
		}
	}
}

func TestFunctionParams(t *testing.T) {
	tests := []struct {
		src     string
		wantErr string
	}{
		// OK cases
		{"def f(x: int): pass\nf(5)", ""},
		{"def f(x: int, y: str): pass\nf(1, 'a')", ""},
		{"def f(x: int = 5): pass\nf()", ""},
		// Error cases
		{"def f(x: int): pass\nf('hi')", "cannot use string as int"},
	}

	for _, test := range tests {
		errs := check(t, test.src, nil)
		if test.wantErr == "" {
			if len(errs) > 0 {
				t.Errorf("check(%q): unexpected errors: %v", test.src, errs)
			}
		} else {
			if len(errs) == 0 {
				t.Errorf("check(%q): expected error containing %q, got none", test.src, test.wantErr)
			} else if !containsError(errs, test.wantErr) {
				t.Errorf("check(%q): expected error containing %q, got %v", test.src, test.wantErr, errs)
			}
		}
	}
}

func TestReturnTypes(t *testing.T) {
	tests := []struct {
		src     string
		wantErr string
	}{
		// OK cases
		{"def f() -> int:\n  return 5", ""},
		{"def f() -> str:\n  return 'hi'", ""},
		// Error cases
		{"def f() -> int:\n  return 'hi'", "cannot use string as int"},
		{"def f() -> str:\n  return 5", "cannot use int as string"},
	}

	for _, test := range tests {
		errs := check(t, test.src, nil)
		if test.wantErr == "" {
			if len(errs) > 0 {
				t.Errorf("check(%q): unexpected errors: %v", test.src, errs)
			}
		} else {
			if len(errs) == 0 {
				t.Errorf("check(%q): expected error containing %q, got none", test.src, test.wantErr)
			} else if !containsError(errs, test.wantErr) {
				t.Errorf("check(%q): expected error containing %q, got %v", test.src, test.wantErr, errs)
			}
		}
	}
}

func TestTypeInference(t *testing.T) {
	tests := []struct {
		src     string
		wantErr string
	}{
		// Inferred types from literals
		{"x = 5\ny: int = x", ""},
		{"x = 'hi'\ny: str = x", ""},
		{"x = 'hi'\ny: int = x", "cannot use string as int"},
		// Built-in function return types
		{"x: int = len('hello')", ""},
		{"x: str = str(42)", ""},
	}

	for _, test := range tests {
		errs := check(t, test.src, nil)
		if test.wantErr == "" {
			if len(errs) > 0 {
				t.Errorf("check(%q): unexpected errors: %v", test.src, errs)
			}
		} else {
			if len(errs) == 0 {
				t.Errorf("check(%q): expected error containing %q, got none", test.src, test.wantErr)
			} else if !containsError(errs, test.wantErr) {
				t.Errorf("check(%q): expected error containing %q, got %v", test.src, test.wantErr, errs)
			}
		}
	}
}

func TestMethodCalls(t *testing.T) {
	tests := []struct {
		src     string
		wantErr string
	}{
		// String methods
		{"x: str = 'hello'.upper()", ""},
		{"x: int = 'hello'.count('l')", ""},
		{"x: bool = 'hello'.startswith('h')", ""},
		{"x: int = 'hello'.upper()", "cannot use string as int"}, // upper() returns str, not int
		// List methods
		{"x: int = [1,2,3].pop()", ""},
		// Split returns list[str]
		{"x: list = 'a,b,c'.split(',')", ""},
	}

	for _, test := range tests {
		errs := check(t, test.src, nil)
		if test.wantErr == "" {
			if len(errs) > 0 {
				t.Errorf("check(%q): unexpected errors: %v", test.src, errs)
			}
		} else {
			if len(errs) == 0 {
				t.Errorf("check(%q): expected error containing %q, got none", test.src, test.wantErr)
			} else if !containsError(errs, test.wantErr) {
				t.Errorf("check(%q): expected error containing %q, got %v", test.src, test.wantErr, errs)
			}
		}
	}
}

func TestAttributeErrors(t *testing.T) {
	tests := []struct {
		src     string
		wantErr string
	}{
		{"x = 5\nx.foo", "type int has no attribute foo"},
	}

	for _, test := range tests {
		errs := check(t, test.src, nil)
		if test.wantErr == "" {
			if len(errs) > 0 {
				t.Errorf("check(%q): unexpected errors: %v", test.src, errs)
			}
		} else {
			if len(errs) == 0 {
				t.Errorf("check(%q): expected error containing %q, got none", test.src, test.wantErr)
			} else if !containsError(errs, test.wantErr) {
				t.Errorf("check(%q): expected error containing %q, got %v", test.src, test.wantErr, errs)
			}
		}
	}
}

func TestFunctionTypesAsValues(t *testing.T) {
	tests := []struct {
		src     string
		wantErr string
	}{
		{"def f(x: int) -> str:\n  return str(x)\ng = f\ny: str = g(5)", ""},
	}

	for _, test := range tests {
		errs := check(t, test.src, nil)
		if test.wantErr == "" {
			if len(errs) > 0 {
				t.Errorf("check(%q): unexpected errors: %v", test.src, errs)
			}
		} else {
			if len(errs) == 0 {
				t.Errorf("check(%q): expected error containing %q, got none", test.src, test.wantErr)
			} else if !containsError(errs, test.wantErr) {
				t.Errorf("check(%q): expected error containing %q, got %v", test.src, test.wantErr, errs)
			}
		}
	}
}

func TestExtensionTypes(t *testing.T) {
	mytype := &typecheck.Named{
		Name: "mytype",
		Attrs: map[string]typecheck.Type{
			"count": typecheck.Int,
		},
	}
	env := &typecheck.Env{Predeclared: map[string]typecheck.Type{
		"mytype": mytype,
		"val":    mytype,
	}}

	tests := []struct {
		src     string
		wantErr string
	}{
		{"x: int = val.count", ""},
		{"x: str = val.count", "cannot use int as string"},
	}

	for _, test := range tests {
		errs := check(t, test.src, env)
		if test.wantErr == "" {
			if len(errs) > 0 {
				t.Errorf("check(%q): unexpected errors: %v", test.src, errs)
			}
		} else {
			if len(errs) == 0 {
				t.Errorf("check(%q): expected error containing %q, got none", test.src, test.wantErr)
			} else if !containsError(errs, test.wantErr) {
				t.Errorf("check(%q): expected error containing %q, got %v", test.src, test.wantErr, errs)
			}
		}
	}
}

func TestUnionTypes(t *testing.T) {
	tests := []struct {
		src     string
		wantErr string
	}{
		{"def f(x: int) -> int:\n  return x\nx: int = f(5)", ""},
	}

	for _, test := range tests {
		errs := check(t, test.src, nil)
		if test.wantErr == "" {
			if len(errs) > 0 {
				t.Errorf("check(%q): unexpected errors: %v", test.src, errs)
			}
		} else {
			if len(errs) == 0 {
				t.Errorf("check(%q): expected error containing %q, got none", test.src, test.wantErr)
			} else if !containsError(errs, test.wantErr) {
				t.Errorf("check(%q): expected error containing %q, got %v", test.src, test.wantErr, errs)
			}
		}
	}
}

func TestAssignability(t *testing.T) {
	tests := []struct {
		src typecheck.Type
		dst typecheck.Type
		ok  bool
	}{
		{typecheck.Int, typecheck.Int, true},
		{typecheck.Int, typecheck.String, false},
		{typecheck.Any, typecheck.Int, true},
		{typecheck.Int, typecheck.Any, true},
		{typecheck.Int, &typecheck.Union{Types: []typecheck.Type{typecheck.Int, typecheck.String}}, true},
		{typecheck.String, &typecheck.Union{Types: []typecheck.Type{typecheck.Int, typecheck.String}}, true},
		{typecheck.Bool, &typecheck.Union{Types: []typecheck.Type{typecheck.Int, typecheck.String}}, false},
		{&typecheck.List{Elem: typecheck.Int}, &typecheck.List{Elem: typecheck.Int}, true},
		{&typecheck.List{Elem: typecheck.Int}, &typecheck.List{Elem: typecheck.String}, false},
		{typecheck.None, typecheck.None, true},
	}

	for _, test := range tests {
		got := typecheck.Assignable(test.src, test.dst)
		if got != test.ok {
			t.Errorf("Assignable(%s, %s) = %v, want %v", test.src, test.dst, got, test.ok)
		}
	}
}

func containsError(errs []typecheck.Error, substr string) bool {
	for _, e := range errs {
		if strings.Contains(e.Msg, substr) {
			return true
		}
	}
	return false
}

// checkWithInfo parses, resolves, and type-checks src, populating info.
func checkWithInfo(t *testing.T, src string, info *typecheck.Info) (*syntax.File, []typecheck.Error) {
	t.Helper()
	f, err := syntax.Parse("test.star", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if err := resolve.File(f, func(name string) bool { return false }, func(name string) bool {
		_, ok := typecheck.Universe[name]
		return ok
	}); err != nil {
		t.Fatalf("resolve error: %v", err)
	}
	errs := typecheck.Check(f, nil, info)
	return f, errs
}

func TestInfoTypes(t *testing.T) {
	info := &typecheck.Info{
		Types: make(map[syntax.Expr]typecheck.TypeAndValue),
	}
	f, errs := checkWithInfo(t, "x = 5 + 3", info)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	// Find the BinaryExpr (5+3) and verify it has type Int.
	assign := f.Stmts[0].(*syntax.AssignStmt)
	binExpr := assign.RHS.(*syntax.BinaryExpr)
	if tv, ok := info.Types[binExpr]; !ok {
		t.Error("BinaryExpr not in Types map")
	} else if tv.Type != typecheck.Int {
		t.Errorf("BinaryExpr type = %v, want int", tv.Type)
	}

	// Verify the literal 5 has type Int.
	lit5 := binExpr.X.(*syntax.Literal)
	if tv, ok := info.Types[lit5]; !ok {
		t.Error("Literal 5 not in Types map")
	} else if tv.Type != typecheck.Int {
		t.Errorf("Literal 5 type = %v, want int", tv.Type)
	}

	// Verify the literal 3 has type Int.
	lit3 := binExpr.Y.(*syntax.Literal)
	if tv, ok := info.Types[lit3]; !ok {
		t.Error("Literal 3 not in Types map")
	} else if tv.Type != typecheck.Int {
		t.Errorf("Literal 3 type = %v, want int", tv.Type)
	}
}

func TestInfoDefs(t *testing.T) {
	info := &typecheck.Info{
		Defs: make(map[*syntax.Ident]*typecheck.Binding),
	}
	f, errs := checkWithInfo(t, "x: int = 5", info)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	// Find the Ident "x" and verify it's in Defs with Type=Int.
	assign := f.Stmts[0].(*syntax.AssignStmt)
	id := assign.LHS.(*syntax.Ident)
	b, ok := info.Defs[id]
	if !ok {
		t.Fatal("Ident 'x' not in Defs map")
	}
	if b.Type != typecheck.Int {
		t.Errorf("Binding type = %v, want int", b.Type)
	}
	if b.Name != "x" {
		t.Errorf("Binding name = %q, want %q", b.Name, "x")
	}
}

func TestInfoUses(t *testing.T) {
	info := &typecheck.Info{
		Defs: make(map[*syntax.Ident]*typecheck.Binding),
		Uses: make(map[*syntax.Ident]*typecheck.UseBinding),
	}
	f, errs := checkWithInfo(t, "x = 5\ny = x", info)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	// "x = 5": x should be in Defs with Type=Any (undeclared).
	assign1 := f.Stmts[0].(*syntax.AssignStmt)
	defId := assign1.LHS.(*syntax.Ident)
	defBinding, ok := info.Defs[defId]
	if !ok {
		t.Fatal("Ident 'x' in 'x = 5' not in Defs map")
	}
	if defBinding.Type != typecheck.Any {
		t.Errorf("Def binding type = %v, want any", defBinding.Type)
	}

	// "y = x": x should be in Uses, with UseBinding.Binding pointing to same def Binding.
	assign2 := f.Stmts[1].(*syntax.AssignStmt)
	useId := assign2.RHS.(*syntax.Ident)
	useBinding, ok := info.Uses[useId]
	if !ok {
		t.Fatal("Ident 'x' in 'y = x' not in Uses map")
	}
	if useBinding.Binding != defBinding {
		t.Errorf("Use binding (%p) != Def binding (%p)", useBinding.Binding, defBinding)
	}
	// The inferred type at the use site should be Int.
	if useBinding.Type != typecheck.Int {
		t.Errorf("UseBinding.Type = %v, want int", useBinding.Type)
	}
}

func TestInfoFunctionDef(t *testing.T) {
	info := &typecheck.Info{
		Defs: make(map[*syntax.Ident]*typecheck.Binding),
		Uses: make(map[*syntax.Ident]*typecheck.UseBinding),
	}
	_, errs := checkWithInfo(t, "def f(x: int) -> str:\n  return str(x)", info)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	// Check that f is in Defs with a Callable type.
	var fBinding *typecheck.Binding
	for id, b := range info.Defs {
		if id.Name == "f" {
			fBinding = b
			break
		}
	}
	if fBinding == nil {
		t.Fatal("function 'f' not found in Defs")
	}
	if _, ok := fBinding.Type.(*typecheck.Callable); !ok {
		t.Errorf("function binding type = %T, want *Callable", fBinding.Type)
	}

	// Check that x param is in Defs with Int type.
	var xDef *typecheck.Binding
	for id, b := range info.Defs {
		if id.Name == "x" {
			xDef = b
			break
		}
	}
	if xDef == nil {
		t.Fatal("param 'x' not found in Defs")
	}
	if xDef.Type != typecheck.Int {
		t.Errorf("param binding type = %v, want int", xDef.Type)
	}

	// Check that x in "str(x)" is in Uses.
	var xUse *typecheck.UseBinding
	for id, ub := range info.Uses {
		if id.Name == "x" {
			xUse = ub
			break
		}
	}
	if xUse == nil {
		t.Fatal("use of 'x' not found in Uses")
	}
	if xUse.Binding != xDef {
		t.Error("use-site binding for x != def-site binding")
	}
}

func TestInfoTypeOf(t *testing.T) {
	info := &typecheck.Info{
		Types: make(map[syntax.Expr]typecheck.TypeAndValue),
		Defs:  make(map[*syntax.Ident]*typecheck.Binding),
	}
	f, errs := checkWithInfo(t, "x: int = 5", info)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	assign := f.Stmts[0].(*syntax.AssignStmt)

	// TypeOf on a literal (non-ident) should return Int.
	lit := assign.RHS.(*syntax.Literal)
	if typ := info.TypeOf(lit); typ != typecheck.Int {
		t.Errorf("TypeOf(literal 5) = %v, want int", typ)
	}

	// TypeOf on a def-site ident should return Int via BindingOf fallback.
	id := assign.LHS.(*syntax.Ident)
	if typ := info.TypeOf(id); typ != typecheck.Int {
		t.Errorf("TypeOf(ident x) = %v, want int", typ)
	}

	// BindingOf on the def-site ident.
	b := info.BindingOf(id)
	if b == nil {
		t.Fatal("BindingOf(x) = nil")
	}
	if b.Type != typecheck.Int {
		t.Errorf("BindingOf(x).Type = %v, want int", b.Type)
	}
}

func TestInfoOptIn(t *testing.T) {
	// Pass Info with only Types allocated (Defs/Uses nil).
	info := &typecheck.Info{
		Types: make(map[syntax.Expr]typecheck.TypeAndValue),
	}
	f, errs := checkWithInfo(t, "x = 5\ny = x", info)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	// Types should be populated.
	if len(info.Types) == 0 {
		t.Error("Types map is empty")
	}

	// Defs/Uses should remain nil (no panic).
	if info.Defs != nil {
		t.Error("Defs should be nil")
	}
	if info.Uses != nil {
		t.Error("Uses should be nil")
	}

	_ = f
}

func TestUnaryOpsDescriptor(t *testing.T) {
	durationType := &typecheck.Named{
		Name: "duration",
		UnaryOps: map[syntax.Token]typecheck.Type{
			syntax.MINUS: &typecheck.Named{Name: "duration"},
		},
	}
	env := &typecheck.Env{Predeclared: map[string]typecheck.Type{
		"duration": durationType,
		"d":        durationType,
	}}

	tests := []struct {
		src     string
		wantErr string
	}{
		{"x: duration = -d", ""},
	}

	for _, test := range tests {
		errs := check(t, test.src, env)
		if test.wantErr == "" {
			if len(errs) > 0 {
				t.Errorf("check(%q): unexpected errors: %v", test.src, errs)
			}
		} else {
			if len(errs) == 0 {
				t.Errorf("check(%q): expected error containing %q, got none", test.src, test.wantErr)
			} else if !containsError(errs, test.wantErr) {
				t.Errorf("check(%q): expected error containing %q, got %v", test.src, test.wantErr, errs)
			}
		}
	}
}

func TestCallSigDescriptor(t *testing.T) {
	regexType := &typecheck.Named{
		Name: "regex",
		CallSig: &typecheck.Callable{
			Name:       "regex",
			Params:     []typecheck.Param{{Name: "s", Type: typecheck.String}},
			ReturnType: typecheck.Bool,
		},
	}
	env := &typecheck.Env{Predeclared: map[string]typecheck.Type{
		"regex":   regexType,
		"pattern": regexType,
	}}

	tests := []struct {
		src     string
		wantErr string
	}{
		// Calling the extension type uses CallSig.
		{"x: bool = pattern('hello')", ""},
		{"x: int = pattern('hello')", "cannot use bool as int"},
		// Wrong argument type.
		{"pattern(42)", "cannot use int as string"},
	}

	for _, test := range tests {
		errs := check(t, test.src, env)
		if test.wantErr == "" {
			if len(errs) > 0 {
				t.Errorf("check(%q): unexpected errors: %v", test.src, errs)
			}
		} else {
			if len(errs) == 0 {
				t.Errorf("check(%q): expected error containing %q, got none", test.src, test.wantErr)
			} else if !containsError(errs, test.wantErr) {
				t.Errorf("check(%q): expected error containing %q, got %v", test.src, test.wantErr, errs)
			}
		}
	}
}

func TestIterElemDescriptor(t *testing.T) {
	strsetType := &typecheck.Named{
		Name:     "strset",
		IterElem: typecheck.String,
	}
	env := &typecheck.Env{Predeclared: map[string]typecheck.Type{
		"strset": strsetType,
		"ss":     strsetType,
	}}

	tests := []struct {
		src     string
		wantErr string
	}{
		{"def f():\n  for x in ss:\n    y: str = x", ""},
		{"def f():\n  for x in ss:\n    y: int = x", "cannot use string as int"},
	}

	for _, test := range tests {
		errs := check(t, test.src, env)
		if test.wantErr == "" {
			if len(errs) > 0 {
				t.Errorf("check(%q): unexpected errors: %v", test.src, errs)
			}
		} else {
			if len(errs) == 0 {
				t.Errorf("check(%q): expected error containing %q, got none", test.src, test.wantErr)
			} else if !containsError(errs, test.wantErr) {
				t.Errorf("check(%q): expected error containing %q, got %v", test.src, test.wantErr, errs)
			}
		}
	}
}

func TestSliceTypeDescriptor(t *testing.T) {
	bufferType := &typecheck.Named{
		Name:  "buffer",
		Slice: &typecheck.Named{Name: "buffer"},
		Index: typecheck.Int,
	}
	env := &typecheck.Env{Predeclared: map[string]typecheck.Type{
		"buffer": bufferType,
		"buf":    bufferType,
	}}

	tests := []struct {
		src     string
		wantErr string
	}{
		{"x: buffer = buf[1:3]", ""},
		{"x: int = buf[0]", ""},
		{"x: str = buf[1:3]", "cannot use buffer as string"},
	}

	for _, test := range tests {
		errs := check(t, test.src, env)
		if test.wantErr == "" {
			if len(errs) > 0 {
				t.Errorf("check(%q): unexpected errors: %v", test.src, errs)
			}
		} else {
			if len(errs) == 0 {
				t.Errorf("check(%q): expected error containing %q, got none", test.src, test.wantErr)
			} else if !containsError(errs, test.wantErr) {
				t.Errorf("check(%q): expected error containing %q, got %v", test.src, test.wantErr, errs)
			}
		}
	}
}

func TestBinaryOpsAllOperators(t *testing.T) {
	vecType := &typecheck.Named{
		Name: "vec",
		BinaryOps: map[syntax.Token]typecheck.Type{
			syntax.PLUS:  &typecheck.Named{Name: "vec"},
			syntax.MINUS: &typecheck.Named{Name: "vec"},
			syntax.STAR:  &typecheck.Named{Name: "vec"},
		},
	}
	env := &typecheck.Env{Predeclared: map[string]typecheck.Type{
		"vec": vecType,
		"v":   vecType,
	}}

	tests := []struct {
		src     string
		wantErr string
	}{
		{"x: vec = v + v", ""},
		{"x: vec = v - v", ""},
		{"x: vec = v * v", ""},
		{"x: str = v + v", "cannot use vec as string"},
	}

	for _, test := range tests {
		errs := check(t, test.src, env)
		if test.wantErr == "" {
			if len(errs) > 0 {
				t.Errorf("check(%q): unexpected errors: %v", test.src, errs)
			}
		} else {
			if len(errs) == 0 {
				t.Errorf("check(%q): expected error containing %q, got none", test.src, test.wantErr)
			} else if !containsError(errs, test.wantErr) {
				t.Errorf("check(%q): expected error containing %q, got %v", test.src, test.wantErr, errs)
			}
		}
	}
}

func TestSetIndexTypeDescriptor(t *testing.T) {
	intarrayType := &typecheck.Named{
		Name:     "intarray",
		Index:    typecheck.Int,
		SetIndex: typecheck.Int,
	}
	env := &typecheck.Env{Predeclared: map[string]typecheck.Type{
		"intarray": intarrayType,
		"arr":      intarrayType,
	}}

	tests := []struct {
		src     string
		wantErr string
	}{
		{"arr[0] = 5", ""},
		{"arr[0] = 'x'", "cannot use string as int"},
	}

	for _, test := range tests {
		errs := check(t, test.src, env)
		if test.wantErr == "" {
			if len(errs) > 0 {
				t.Errorf("check(%q): unexpected errors: %v", test.src, errs)
			}
		} else {
			if len(errs) == 0 {
				t.Errorf("check(%q): expected error containing %q, got none", test.src, test.wantErr)
			} else if !containsError(errs, test.wantErr) {
				t.Errorf("check(%q): expected error containing %q, got %v", test.src, test.wantErr, errs)
			}
		}
	}
}

func TestMappingKeyValueDescriptor(t *testing.T) {
	configType := &typecheck.Named{
		Name:   "config",
		SetKey: typecheck.String,
		Index:  typecheck.String,
	}
	env := &typecheck.Env{Predeclared: map[string]typecheck.Type{
		"config": configType,
		"cfg":    configType,
	}}

	tests := []struct {
		src     string
		wantErr string
	}{
		{"cfg['key'] = 'value'", ""},
		{"cfg['key'] = 42", "cannot use int as string"},
		{"x: str = cfg['key']", ""},
	}

	for _, test := range tests {
		errs := check(t, test.src, env)
		if test.wantErr == "" {
			if len(errs) > 0 {
				t.Errorf("check(%q): unexpected errors: %v", test.src, errs)
			}
		} else {
			if len(errs) == 0 {
				t.Errorf("check(%q): expected error containing %q, got none", test.src, test.wantErr)
			} else if !containsError(errs, test.wantErr) {
				t.Errorf("check(%q): expected error containing %q, got %v", test.src, test.wantErr, errs)
			}
		}
	}
}

func TestSetFieldTypesDescriptor(t *testing.T) {
	pointType := &typecheck.Named{
		Name: "point",
		Attrs: map[string]typecheck.Type{
			"x": typecheck.Int,
			"y": typecheck.Int,
		},
		SetFields: map[string]typecheck.Type{
			"x": typecheck.Int,
			"y": typecheck.Int,
		},
	}
	env := &typecheck.Env{Predeclared: map[string]typecheck.Type{
		"point": pointType,
		"p":     pointType,
	}}

	tests := []struct {
		src     string
		wantErr string
	}{
		{"p.x = 5", ""},
		{"p.y = 10", ""},
		{"p.x = 'hi'", "cannot use string as int in field assignment"},
		// Reading attrs still works.
		{"z: int = p.x", ""},
		{"z: str = p.x", "cannot use int as string"},
	}

	for _, test := range tests {
		errs := check(t, test.src, env)
		if test.wantErr == "" {
			if len(errs) > 0 {
				t.Errorf("check(%q): unexpected errors: %v", test.src, errs)
			}
		} else {
			if len(errs) == 0 {
				t.Errorf("check(%q): expected error containing %q, got none", test.src, test.wantErr)
			} else if !containsError(errs, test.wantErr) {
				t.Errorf("check(%q): expected error containing %q, got %v", test.src, test.wantErr, errs)
			}
		}
	}
}

func TestComparableDescriptor(t *testing.T) {
	boolTrue := true
	boolFalse := false
	orderedType := &typecheck.Named{
		Name:       "ordered",
		Comparable: &boolTrue,
	}
	unorderedType := &typecheck.Named{
		Name:       "unordered",
		Comparable: &boolFalse,
	}
	env := &typecheck.Env{Predeclared: map[string]typecheck.Type{
		"ordered":   orderedType,
		"unordered": unorderedType,
		"a":         orderedType,
		"b":         unorderedType,
	}}

	tests := []struct {
		src     string
		wantErr string
	}{
		// Equality is always allowed.
		{"x = a == a", ""},
		{"x = b == b", ""},
		{"x = b != b", ""},
		// Ordering is allowed for comparable types.
		{"x = a < a", ""},
		{"x = a >= a", ""},
		// Ordering is not allowed for non-comparable types.
		{"x = b < b", "does not support"},
		{"x = b > b", "does not support"},
		{"x = b <= b", "does not support"},
		{"x = b >= b", "does not support"},
	}

	for _, test := range tests {
		errs := check(t, test.src, env)
		if test.wantErr == "" {
			if len(errs) > 0 {
				t.Errorf("check(%q): unexpected errors: %v", test.src, errs)
			}
		} else {
			if len(errs) == 0 {
				t.Errorf("check(%q): expected error containing %q, got none", test.src, test.wantErr)
			} else if !containsError(errs, test.wantErr) {
				t.Errorf("check(%q): expected error containing %q, got %v", test.src, test.wantErr, errs)
			}
		}
	}
}

func TestIndexAssignmentBuiltins(t *testing.T) {
	tests := []struct {
		src     string
		wantErr string
	}{
		// List index assignment.
		{"xs = [1, 2, 3]\nxs[0] = 5", ""},
		{"xs = [1, 2, 3]\nxs[0] = 'hi'", "cannot use string as int in list assignment"},
		// Dict key assignment.
		{"d = {'a': 1}\nd['b'] = 2", ""},
		{"d = {'a': 1}\nd['b'] = 'hi'", "cannot use string as int in dict assignment"},
	}

	for _, test := range tests {
		errs := check(t, test.src, nil)
		if test.wantErr == "" {
			if len(errs) > 0 {
				t.Errorf("check(%q): unexpected errors: %v", test.src, errs)
			}
		} else {
			if len(errs) == 0 {
				t.Errorf("check(%q): expected error containing %q, got none", test.src, test.wantErr)
			} else if !containsError(errs, test.wantErr) {
				t.Errorf("check(%q): expected error containing %q, got %v", test.src, test.wantErr, errs)
			}
		}
	}
}

func TestFieldAssignmentObject(t *testing.T) {
	env := &typecheck.Env{Predeclared: map[string]typecheck.Type{
		"obj": &typecheck.Object{
			Name: "myobj",
			Attrs: map[string]typecheck.Type{
				"name": typecheck.String,
				"age":  typecheck.Int,
			},
		},
	}}

	tests := []struct {
		src     string
		wantErr string
	}{
		{"obj.name = 'alice'", ""},
		{"obj.age = 30", ""},
		{"obj.name = 42", "cannot use int as string in field assignment"},
		{"obj.age = 'old'", "cannot use string as int in field assignment"},
	}

	for _, test := range tests {
		errs := check(t, test.src, env)
		if test.wantErr == "" {
			if len(errs) > 0 {
				t.Errorf("check(%q): unexpected errors: %v", test.src, errs)
			}
		} else {
			if len(errs) == 0 {
				t.Errorf("check(%q): expected error containing %q, got none", test.src, test.wantErr)
			} else if !containsError(errs, test.wantErr) {
				t.Errorf("check(%q): expected error containing %q, got %v", test.src, test.wantErr, errs)
			}
		}
	}
}

func TestLoadCallback(t *testing.T) {
	mathTypes := map[string]typecheck.Type{
		"pi":  typecheck.Float,
		"add": typecheck.Int,
	}

	tests := []struct {
		name    string
		src     string
		env     *typecheck.Env
		wantErr string
	}{
		{
			name: "known module, known name",
			src:  "load('math.star', 'pi')\nx: float = pi",
			env: &typecheck.Env{
				Load: func(module string) map[string]typecheck.Type {
					if module == "math.star" {
						return mathTypes
					}
					return nil
				},
			},
			wantErr: "",
		},
		{
			name: "known module, unknown name",
			src:  "load('math.star', 'unknown')",
			env: &typecheck.Env{
				Load: func(module string) map[string]typecheck.Type {
					if module == "math.star" {
						return mathTypes
					}
					return nil
				},
			},
			wantErr: "",
		},
		{
			name: "unknown module returns nil",
			src:  "load('other.star', 'foo')",
			env: &typecheck.Env{
				Load: func(module string) map[string]typecheck.Type {
					return nil
				},
			},
			wantErr: "",
		},
		{
			name:    "nil Load callback",
			src:     "load('math.star', 'pi')",
			env:     nil,
			wantErr: "",
		},
		{
			name: "aliased load",
			src:  "load('math.star', x='pi')\ny: float = x",
			env: &typecheck.Env{
				Load: func(module string) map[string]typecheck.Type {
					return mathTypes
				},
			},
			wantErr: "",
		},
		{
			name: "type error through load",
			src:  "load('math.star', 'pi')\nx: int = pi",
			env: &typecheck.Env{
				Load: func(module string) map[string]typecheck.Type {
					return mathTypes
				},
			},
			wantErr: "cannot use float as int",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errs := check(t, test.src, test.env)
			if test.wantErr == "" {
				if len(errs) > 0 {
					t.Errorf("unexpected errors: %v", errs)
				}
			} else {
				if len(errs) == 0 {
					t.Errorf("expected error containing %q, got none", test.wantErr)
				} else if !containsError(errs, test.wantErr) {
					t.Errorf("expected error containing %q, got %v", test.wantErr, errs)
				}
			}
		})
	}
}

func TestUntypedReassignment(t *testing.T) {
	tests := []struct {
		src     string
		wantErr string
	}{
		// Untyped variables can be reassigned to any type.
		{"def f():\n  x = 5\n  x = 'hello'", ""},
		{"def f():\n  x = 'hello'\n  x = 5", ""},
		{"def f():\n  x = [1,2,3]\n  x = 'hello'", ""},
		// Typed variables cannot be reassigned to a different type.
		{"def f():\n  x: int = 5\n  x = 'hello'", "cannot use string as int"},
		{"def f():\n  x: str = 'hi'\n  x = 5", "cannot use int as string"},
		// Augmented assignment on untyped variables is OK if the op is valid.
		{"def f():\n  x = 5\n  x += 1", ""},
		// Augmented assignment on typed variables checks the result type.
		{"def f():\n  x: int = 5\n  x += 1", ""},
	}

	for _, test := range tests {
		errs := check(t, test.src, nil)
		if test.wantErr == "" {
			if len(errs) > 0 {
				t.Errorf("check(%q): unexpected errors: %v", test.src, errs)
			}
		} else {
			if len(errs) == 0 {
				t.Errorf("check(%q): expected error containing %q, got none", test.src, test.wantErr)
			} else if !containsError(errs, test.wantErr) {
				t.Errorf("check(%q): expected error containing %q, got %v", test.src, test.wantErr, errs)
			}
		}
	}
}

func TestFlowSensitiveTypes(t *testing.T) {
	// Env with predeclared "c" (bool) for conditions and "xs" (list[int]) for iteration.
	env := &typecheck.Env{Predeclared: map[string]typecheck.Type{
		"c":  typecheck.Bool,
		"xs": &typecheck.List{Elem: typecheck.Int},
	}}

	tests := []struct {
		name    string
		src     string
		wantErr string
	}{
		{
			name:    "if no else, reassign",
			src:     "def f():\n  x = 4\n  if c:\n    x = 'hello'\n  y: int = x",
			wantErr: "cannot use",
		},
		{
			name:    "if no else, union ok",
			src:     "def f():\n  x = 4\n  if c:\n    x = 'hello'\n  y: int | str = x",
			wantErr: "",
		},
		{
			name:    "if no else, no reassign",
			src:     "def f():\n  x = 4\n  if c:\n    pass\n  y: int = x",
			wantErr: "",
		},
		{
			name:    "if/else both reassign",
			src:     "def f():\n  x = 4\n  if c:\n    x = 'hello'\n  else:\n    x = 1.5\n  y: int = x",
			wantErr: "cannot use",
		},
		{
			name:    "if/else one reassign",
			src:     "def f():\n  x = 4\n  if c:\n    x = 'hello'\n  else:\n    pass\n  y: int = x",
			wantErr: "cannot use",
		},
		{
			name:    "for loop reassign",
			src:     "def f():\n  x = 'hello'\n  for i in xs:\n    x = i\n  y: str = x",
			wantErr: "cannot use",
		},
		{
			name:    "declared var unaffected",
			src:     "def f():\n  x: int = 4\n  if c:\n    x = 5\n  y: int = x",
			wantErr: "",
		},
		{
			name:    "no modification",
			src:     "def f():\n  x = 4\n  if c:\n    y = 5\n  z: int = x",
			wantErr: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errs := check(t, test.src, env)
			if test.wantErr == "" {
				if len(errs) > 0 {
					t.Errorf("unexpected errors: %v", errs)
				}
			} else {
				if len(errs) == 0 {
					t.Errorf("expected error containing %q, got none", test.wantErr)
				} else if !containsError(errs, test.wantErr) {
					t.Errorf("expected error containing %q, got %v", test.wantErr, errs)
				}
			}
		})
	}

	// While loop test needs AllowRecursion.
	t.Run("while loop reassign", func(t *testing.T) {
		old := resolve.AllowRecursion
		resolve.AllowRecursion = true
		defer func() { resolve.AllowRecursion = old }()

		src := "def f():\n  x = 4\n  while c:\n    x = 'hello'\n  y: int = x"
		errs := check(t, src, env)
		if len(errs) == 0 {
			t.Errorf("expected error containing %q, got none", "cannot use")
		} else if !containsError(errs, "cannot use") {
			t.Errorf("expected error containing %q, got %v", "cannot use", errs)
		}
	})
}

func TestFlowSensitiveBindings(t *testing.T) {
	env := &typecheck.Env{Predeclared: map[string]typecheck.Type{
		"c": typecheck.Bool,
	}}

	info := &typecheck.Info{
		Types: make(map[syntax.Expr]typecheck.TypeAndValue),
		Defs:  make(map[*syntax.Ident]*typecheck.Binding),
		Uses:  make(map[*syntax.Ident]*typecheck.UseBinding),
	}

	src := "def f():\n  x = 4\n  if c:\n    x = 'hello'\n  y = x"

	f, err := syntax.Parse("test.star", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if err := resolve.File(f, func(name string) bool {
		_, ok := env.Predeclared[name]
		return ok
	}, func(name string) bool {
		_, ok := typecheck.Universe[name]
		return ok
	}); err != nil {
		t.Fatalf("resolve error: %v", err)
	}
	errs := typecheck.Check(f, env, info)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	// Navigate into the function body to find "y = x".
	defStmt := f.Stmts[0].(*syntax.DefStmt)
	body := defStmt.Body
	// body[0] = x = 4, body[1] = if ..., body[2] = y = x
	assign := body[2].(*syntax.AssignStmt)
	useId := assign.RHS.(*syntax.Ident)
	ub, ok := info.Uses[useId]
	if !ok {
		t.Fatal("x in 'y = x' not in Uses")
	}

	// The type should be a Union of int and string.
	union, ok := ub.Type.(*typecheck.Union)
	if !ok {
		t.Fatalf("UseBinding.Type = %T (%v), want *Union", ub.Type, ub.Type)
	}

	// Check that the union contains both int and string.
	hasInt, hasStr := false, false
	for _, ut := range union.Types {
		if ut == typecheck.Int {
			hasInt = true
		}
		if ut == typecheck.String {
			hasStr = true
		}
	}
	if !hasInt || !hasStr {
		t.Errorf("Union types = %v, want {int, string}", union.Types)
	}
}

func TestReassignmentBindings(t *testing.T) {
	info := &typecheck.Info{
		Types: make(map[syntax.Expr]typecheck.TypeAndValue),
		Defs:  make(map[*syntax.Ident]*typecheck.Binding),
		Uses:  make(map[*syntax.Ident]*typecheck.UseBinding),
	}
	// x = 5; y = x; x = "hello"; z = x
	f, errs := checkWithInfo(t, "def f():\n  x = 5\n  y = x\n  x = 'hello'\n  z = x", info)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	// Collect all Defs and Uses for "x".
	defStmt := f.Stmts[0].(*syntax.DefStmt)
	body := defStmt.Body

	// x = 5 (stmt 0)
	assign1 := body[0].(*syntax.AssignStmt)
	xDef1 := assign1.LHS.(*syntax.Ident)
	def1, ok := info.Defs[xDef1]
	if !ok {
		t.Fatal("x in 'x = 5' not in Defs")
	}

	// y = x (stmt 1): x is a use
	assign2 := body[1].(*syntax.AssignStmt)
	xUse1 := assign2.RHS.(*syntax.Ident)
	use1, ok := info.Uses[xUse1]
	if !ok {
		t.Fatal("x in 'y = x' not in Uses")
	}

	// x = "hello" (stmt 2)
	assign3 := body[2].(*syntax.AssignStmt)
	xDef2 := assign3.LHS.(*syntax.Ident)
	def2, ok := info.Defs[xDef2]
	if !ok {
		t.Fatal("x in 'x = hello' not in Defs")
	}

	// z = x (stmt 3): x is a use
	assign4 := body[3].(*syntax.AssignStmt)
	xUse2 := assign4.RHS.(*syntax.Ident)
	use2, ok := info.Uses[xUse2]
	if !ok {
		t.Fatal("x in 'z = x' not in Uses")
	}

	// Both Defs entries for x should point to the same binding (canonical def).
	if def1 != def2 {
		t.Errorf("Defs[x=5] (%p) != Defs[x=hello] (%p); want same canonical binding", def1, def2)
	}

	// The canonical def binding should have Type=Any (undeclared variable).
	if def1.Type != typecheck.Any {
		t.Errorf("canonical def binding type = %v, want any", def1.Type)
	}

	// UseBinding.Binding should be pointer-equal to the canonical def binding.
	if use1.Binding != def1 {
		t.Errorf("use1.Binding (%p) != def1 (%p)", use1.Binding, def1)
	}
	if use2.Binding != def1 {
		t.Errorf("use2.Binding (%p) != def1 (%p)", use2.Binding, def1)
	}

	// Use-site inferred types should reflect the current inferred type.
	if use1.Type != typecheck.Int {
		t.Errorf("use1.Type = %v, want int", use1.Type)
	}
	if use2.Type != typecheck.String {
		t.Errorf("use2.Type = %v, want string", use2.Type)
	}
}
