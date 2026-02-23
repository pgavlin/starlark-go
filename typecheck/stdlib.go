package typecheck

// Universe defines the set of universal built-in types, such as None, True, and len.
//
// The Go application may add or remove items from the
// universe dictionary before type-checking begins.
var Universe map[string]Type

func init() {
	Universe = map[string]Type{
		// Constants
		"None":  None,
		"True":  Bool,
		"False": Bool,
		// Type constructors / converters
		"bool":  &Callable{Name: "bool", Params: []Param{{Name: "x", Type: Any, Optional: true}}, ReturnType: Bool},
		"int":   &Callable{Name: "int", Params: []Param{{Name: "x", Type: Any}, {Name: "base", Type: Int, Optional: true}}, ReturnType: Int},
		"float": &Callable{Name: "float", Params: []Param{{Name: "x", Type: Any, Optional: true}}, ReturnType: Float},
		"str":   &Callable{Name: "str", Params: []Param{{Name: "x", Type: Any}}, ReturnType: String},
		"bytes": &Callable{Name: "bytes", Params: []Param{{Name: "x", Type: Any}}, ReturnType: Bytes},
		"list":  &Callable{Name: "list", Params: []Param{{Name: "x", Type: Any, Optional: true}}, ReturnType: &List{Any}},
		"dict":  &Callable{Name: "dict", Params: []Param{{Name: "pairs", Type: Any, Star: true}}, ReturnType: &Dict{Any, Any}},
		"tuple": &Callable{Name: "tuple", Params: []Param{{Name: "x", Type: Any, Optional: true}}, ReturnType: &Tuple{}},
		// Functions
		"len":       &Callable{Name: "len", Params: []Param{{Name: "x", Type: Any}}, ReturnType: Int},
		"range":     &Callable{Name: "range", Params: []Param{{Name: "start_or_stop", Type: Int}, {Name: "stop", Type: Int, Optional: true}, {Name: "step", Type: Int, Optional: true}}, ReturnType: &List{Int}},
		"abs":       &Callable{Name: "abs", Params: []Param{{Name: "x", Type: &Union{[]Type{Int, Float}}}}, ReturnType: &Union{[]Type{Int, Float}}},
		"sorted":    &Callable{Name: "sorted", Params: []Param{{Name: "x", Type: Any}}, ReturnType: &List{Any}},
		"reversed":  &Callable{Name: "reversed", Params: []Param{{Name: "x", Type: Any}}, ReturnType: &List{Any}},
		"hasattr":   &Callable{Name: "hasattr", Params: []Param{{Name: "x", Type: Any}, {Name: "name", Type: String}}, ReturnType: Bool},
		"getattr":   &Callable{Name: "getattr", Params: []Param{{Name: "x", Type: Any}, {Name: "name", Type: String}, {Name: "default", Type: Any, Optional: true}}, ReturnType: Any},
		"type":      &Callable{Name: "type", Params: []Param{{Name: "x", Type: Any}}, ReturnType: String},
		"print":     &Callable{Name: "print", Params: []Param{{Name: "args", Type: Any, Star: true}}, ReturnType: None},
		"repr":      &Callable{Name: "repr", Params: []Param{{Name: "x", Type: Any}}, ReturnType: String},
		"hash":      &Callable{Name: "hash", Params: []Param{{Name: "x", Type: Any}}, ReturnType: Int},
		"enumerate": &Callable{Name: "enumerate", Params: []Param{{Name: "x", Type: Any}}, ReturnType: &List{Any}},
		"zip":       &Callable{Name: "zip", Params: []Param{{Name: "iterables", Type: Any, Star: true}}, ReturnType: &List{Any}},
		"any":       &Callable{Name: "any", Params: []Param{{Name: "x", Type: Any}}, ReturnType: Bool},
		"all":       &Callable{Name: "all", Params: []Param{{Name: "x", Type: Any}}, ReturnType: Bool},
		"chr":       &Callable{Name: "chr", Params: []Param{{Name: "i", Type: Int}}, ReturnType: String},
		"ord":       &Callable{Name: "ord", Params: []Param{{Name: "c", Type: String}}, ReturnType: Int},
		"dir":       &Callable{Name: "dir", Params: []Param{{Name: "x", Type: Any}}, ReturnType: &List{String}},
		"fail":      &Callable{Name: "fail", Params: []Param{{Name: "args", Type: Any, Star: true}}, ReturnType: Any},
		"max":       &Callable{Name: "max", Params: []Param{{Name: "args", Type: Any, Star: true}}, ReturnType: Any},
		"min":       &Callable{Name: "min", Params: []Param{{Name: "args", Type: Any, Star: true}}, ReturnType: Any},
		"set":       &Callable{Name: "set", Params: []Param{{Name: "x", Type: Any, Optional: true}}, ReturnType: &Set{Any}},
	}
}
