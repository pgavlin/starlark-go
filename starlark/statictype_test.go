package starlark_test

import (
	"testing"

	"github.com/pgavlin/starlark-go/starlark"
	"github.com/pgavlin/starlark-go/typecheck"
)

func TestStaticType(t *testing.T) {
	tests := []struct {
		name string
		val  starlark.Value
		want string
	}{
		{"None", starlark.None, "None"},
		{"True", starlark.True, "bool"},
		{"False", starlark.False, "bool"},
		{"Int", starlark.MakeInt(42), "int"},
		{"Float", starlark.Float(3.14), "float"},
		{"String", starlark.String("hello"), "string"},
		{"Bytes", starlark.Bytes("abc"), "bytes"},

		// Empty containers
		{"EmptyList", starlark.NewList(nil), "list[any]"},
		{"EmptyDict", starlark.NewDict(0), "dict[any, any]"},
		{"EmptyTuple", starlark.Tuple{}, "tuple"},
		{"EmptySet", starlark.NewSet(0), "set[any]"},

		// Homogeneous containers
		{"ListOfInt", starlark.NewList([]starlark.Value{starlark.MakeInt(1), starlark.MakeInt(2)}), "list[int]"},
		{"ListOfString", starlark.NewList([]starlark.Value{starlark.String("a"), starlark.String("b")}), "list[string]"},
		{"Tuple2", starlark.Tuple{starlark.MakeInt(1), starlark.String("x")}, "tuple[int, string]"},

		// Mixed containers fall back to any
		{"MixedList", starlark.NewList([]starlark.Value{starlark.MakeInt(1), starlark.String("x")}), "list[any]"},

		// Builtin (default signature accepts any args)
		{"Builtin", starlark.NewBuiltin("myfunc", func(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			return starlark.None, nil
		}), "myfunc(*args: any, **kwargs: any) -> any"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := starlark.StaticType(tt.val)
			if got.String() != tt.want {
				t.Errorf("StaticType(%v) = %s, want %s", tt.val, got.String(), tt.want)
			}
		})
	}
}

func TestStaticTypeDict(t *testing.T) {
	d := starlark.NewDict(2)
	d.SetKey(starlark.String("a"), starlark.MakeInt(1))
	d.SetKey(starlark.String("b"), starlark.MakeInt(2))

	got := starlark.StaticType(d)
	if got.String() != "dict[string, int]" {
		t.Errorf("StaticType(dict) = %s, want dict[string, int]", got.String())
	}
}

func TestStaticTypeSet(t *testing.T) {
	s := starlark.NewSet(2)
	s.Insert(starlark.MakeInt(1))
	s.Insert(starlark.MakeInt(2))

	got := starlark.StaticType(s)
	if got.String() != "set[int]" {
		t.Errorf("StaticType(set) = %s, want set[int]", got.String())
	}
}

func TestStaticTypeFunction(t *testing.T) {
	thread := &starlark.Thread{Name: "test"}
	globals, err := starlark.ExecFile(thread, "test.star", `
def greet(name, greeting="hello"):
    return greeting + " " + name
`, nil)
	if err != nil {
		t.Fatal(err)
	}

	fn := globals["greet"]
	got := starlark.StaticType(fn)
	callable, ok := got.(*typecheck.Callable)
	if !ok {
		t.Fatalf("StaticType(function) = %T, want *typecheck.Callable", got)
	}
	if callable.Name != "greet" {
		t.Errorf("Name = %q, want %q", callable.Name, "greet")
	}
	if len(callable.Params) != 2 {
		t.Fatalf("len(Params) = %d, want 2", len(callable.Params))
	}
	if callable.Params[0].Name != "name" {
		t.Errorf("Params[0].Name = %q, want %q", callable.Params[0].Name, "name")
	}
	if callable.Params[1].Name != "greeting" || !callable.Params[1].Optional {
		t.Errorf("Params[1] = %+v, want {Name:greeting Optional:true}", callable.Params[1])
	}
}

func TestStaticTypeFunctionVarargs(t *testing.T) {
	thread := &starlark.Thread{Name: "test"}
	globals, err := starlark.ExecFile(thread, "test.star", `
def variadic(a, *args, **kwargs):
    pass
`, nil)
	if err != nil {
		t.Fatal(err)
	}

	fn := globals["variadic"]
	got := starlark.StaticType(fn)
	callable, ok := got.(*typecheck.Callable)
	if !ok {
		t.Fatalf("StaticType(function) = %T, want *typecheck.Callable", got)
	}
	if len(callable.Params) != 3 {
		t.Fatalf("len(Params) = %d, want 3", len(callable.Params))
	}
	if callable.Params[0].Name != "a" || callable.Params[0].Star || callable.Params[0].StarStar {
		t.Errorf("Params[0] = %+v, want regular param 'a'", callable.Params[0])
	}
	if callable.Params[1].Name != "args" || !callable.Params[1].Star {
		t.Errorf("Params[1] = %+v, want *args", callable.Params[1])
	}
	if callable.Params[2].Name != "kwargs" || !callable.Params[2].StarStar {
		t.Errorf("Params[2] = %+v, want **kwargs", callable.Params[2])
	}
}

func TestStaticTypeNamed(t *testing.T) {
	type myValue struct {
		starlark.Value
	}
	v := &myValue{Value: starlark.None}
	got := starlark.StaticType(v)
	named, ok := got.(typecheck.Named)
	if !ok {
		t.Fatalf("StaticType(custom) = %T, want typecheck.Named", got)
	}
	if named.Name() != "NoneType" {
		t.Errorf("Named.Name() = %q, want %q", named.Name(), "NoneType")
	}
}

func TestBuiltinStaticTypeDefault(t *testing.T) {
	b := starlark.NewBuiltin("myfunc", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return starlark.None, nil
	})

	got := b.StaticType()
	if got.Name != "myfunc" {
		t.Errorf("Name = %q, want %q", got.Name, "myfunc")
	}
	if got.ReturnType != typecheck.Any {
		t.Errorf("ReturnType = %s, want any", got.ReturnType)
	}
	if len(got.Params) != 2 {
		t.Fatalf("len(Params) = %d, want 2", len(got.Params))
	}
	if got.Params[0].Name != "args" || !got.Params[0].Star {
		t.Errorf("Params[0] = %+v, want *args", got.Params[0])
	}
	if got.Params[1].Name != "kwargs" || !got.Params[1].StarStar {
		t.Errorf("Params[1] = %+v, want **kwargs", got.Params[1])
	}
}

func TestNewBuiltinWithSignature(t *testing.T) {
	sig := &typecheck.Callable{
		Name:       "add",
		Params:     []typecheck.Param{{Name: "x", Type: typecheck.Int}, {Name: "y", Type: typecheck.Int}},
		ReturnType: typecheck.Int,
	}
	b := starlark.NewBuiltinWithSignature("add", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return starlark.None, nil
	}, sig)

	got := b.StaticType()
	if got != sig {
		t.Errorf("StaticType() returned different object than the one supplied")
	}
	if got.String() != "add(x: int, y: int) -> int" {
		t.Errorf("StaticType().String() = %s, want add(x: int, y: int) -> int", got.String())
	}

	// StaticType(Value) should also return the custom signature.
	fromValue := starlark.StaticType(b)
	if fromValue != sig {
		t.Errorf("StaticType(Value) returned different object than the one supplied")
	}
}

func TestBuiltinBindReceiverPreservesSignature(t *testing.T) {
	sig := &typecheck.Callable{
		Name:       "method",
		Params:     []typecheck.Param{{Name: "x", Type: typecheck.String}},
		ReturnType: typecheck.Bool,
	}
	b := starlark.NewBuiltinWithSignature("method", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return starlark.None, nil
	}, sig)

	bound := b.BindReceiver(starlark.String("hello"))
	got := bound.StaticType()
	if got != sig {
		t.Errorf("BindReceiver did not preserve signature")
	}
}
