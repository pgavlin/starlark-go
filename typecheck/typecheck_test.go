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
			if _, ok := env.Names[name]; ok {
				return true
			}
		}
		return false
	}, func(name string) bool {
		switch name {
		case "None", "True", "False", "len", "str", "int", "float", "bool",
			"list", "dict", "tuple", "range", "print", "type", "repr",
			"hash", "enumerate", "zip", "any", "all", "chr", "ord",
			"dir", "fail", "max", "min", "sorted", "reversed",
			"hasattr", "getattr", "abs", "bytes", "set",
			"struct", "module":
			return true
		}
		return false
	}); err != nil {
		t.Fatalf("resolve error: %v", err)
	}
	return typecheck.Check(f, env)
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
	env := typecheck.StandardEnv()
	env.TypeDescriptors = map[string]*typecheck.TypeDescriptor{
		"mytype": {
			Attrs: map[string]typecheck.Type{
				"count": typecheck.Int,
			},
		},
	}
	env.Names["val"] = &typecheck.Named{Name: "mytype"}

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
