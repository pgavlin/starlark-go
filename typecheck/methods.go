package typecheck

// builtinMethods returns the method table for a built-in type.
// For parameterized types (list[T], dict[K,V]), method signatures use
// the concrete element types.
func (c *Checker) builtinMethods(typ Type) map[string]*Callable {
	switch t := typ.(type) {
	case *basicType:
		switch t {
		case String:
			return stringMethods()
		case Bytes:
			return bytesMethods()
		case Int:
			return nil // no methods on int
		case Float:
			return nil
		case Bool:
			return nil
		case None:
			return nil
		}
	case *List:
		return listMethods(t.Elem)
	case *Dict:
		return dictMethods(t.Key, t.Value)
	case *Set:
		return setMethods(t.Elem)
	}
	return nil
}

func stringMethods() map[string]*Callable {
	return map[string]*Callable{
		"capitalize":    {Name: "capitalize", ReturnType: String},
		"lower":         {Name: "lower", ReturnType: String},
		"upper":         {Name: "upper", ReturnType: String},
		"title":         {Name: "title", ReturnType: String},
		"strip":         {Name: "strip", Params: []Param{{Name: "chars", Type: String, Optional: true}}, ReturnType: String},
		"lstrip":        {Name: "lstrip", Params: []Param{{Name: "chars", Type: String, Optional: true}}, ReturnType: String},
		"rstrip":        {Name: "rstrip", Params: []Param{{Name: "chars", Type: String, Optional: true}}, ReturnType: String},
		"count":         {Name: "count", Params: []Param{{Name: "sub", Type: String}, {Name: "start", Type: Int, Optional: true}, {Name: "end", Type: Int, Optional: true}}, ReturnType: Int},
		"endswith":      {Name: "endswith", Params: []Param{{Name: "suffix", Type: Any}, {Name: "start", Type: Int, Optional: true}, {Name: "end", Type: Int, Optional: true}}, ReturnType: Bool},
		"startswith":    {Name: "startswith", Params: []Param{{Name: "prefix", Type: Any}, {Name: "start", Type: Int, Optional: true}, {Name: "end", Type: Int, Optional: true}}, ReturnType: Bool},
		"find":          {Name: "find", Params: []Param{{Name: "sub", Type: String}, {Name: "start", Type: Int, Optional: true}, {Name: "end", Type: Int, Optional: true}}, ReturnType: Int},
		"rfind":         {Name: "rfind", Params: []Param{{Name: "sub", Type: String}, {Name: "start", Type: Int, Optional: true}, {Name: "end", Type: Int, Optional: true}}, ReturnType: Int},
		"index":         {Name: "index", Params: []Param{{Name: "sub", Type: String}, {Name: "start", Type: Int, Optional: true}, {Name: "end", Type: Int, Optional: true}}, ReturnType: Int},
		"rindex":        {Name: "rindex", Params: []Param{{Name: "sub", Type: String}, {Name: "start", Type: Int, Optional: true}, {Name: "end", Type: Int, Optional: true}}, ReturnType: Int},
		"format":        {Name: "format", Params: []Param{{Name: "args", Type: Any, Star: true}, {Name: "kwargs", Type: Any, StarStar: true}}, ReturnType: String},
		"isalnum":       {Name: "isalnum", ReturnType: Bool},
		"isalpha":       {Name: "isalpha", ReturnType: Bool},
		"isdigit":       {Name: "isdigit", ReturnType: Bool},
		"islower":       {Name: "islower", ReturnType: Bool},
		"isspace":       {Name: "isspace", ReturnType: Bool},
		"istitle":       {Name: "istitle", ReturnType: Bool},
		"isupper":       {Name: "isupper", ReturnType: Bool},
		"join":          {Name: "join", Params: []Param{{Name: "iterable", Type: Any}}, ReturnType: String},
		"partition":     {Name: "partition", Params: []Param{{Name: "sep", Type: String}}, ReturnType: &Tuple{Elems: []Type{String, String, String}}},
		"rpartition":    {Name: "rpartition", Params: []Param{{Name: "sep", Type: String}}, ReturnType: &Tuple{Elems: []Type{String, String, String}}},
		"removeprefix":  {Name: "removeprefix", Params: []Param{{Name: "prefix", Type: String}}, ReturnType: String},
		"removesuffix":  {Name: "removesuffix", Params: []Param{{Name: "suffix", Type: String}}, ReturnType: String},
		"replace":       {Name: "replace", Params: []Param{{Name: "old", Type: String}, {Name: "new", Type: String}, {Name: "count", Type: Int, Optional: true}}, ReturnType: String},
		"split":         {Name: "split", Params: []Param{{Name: "sep", Type: String, Optional: true}, {Name: "maxsplit", Type: Int, Optional: true}}, ReturnType: &List{String}},
		"rsplit":        {Name: "rsplit", Params: []Param{{Name: "sep", Type: String, Optional: true}, {Name: "maxsplit", Type: Int, Optional: true}}, ReturnType: &List{String}},
		"splitlines":    {Name: "splitlines", Params: []Param{{Name: "keepends", Type: Bool, Optional: true}}, ReturnType: &List{String}},
		"elems":         {Name: "elems", ReturnType: &List{String}},
		"codepoints":    {Name: "codepoints", ReturnType: &List{String}},
		"elem_ords":     {Name: "elem_ords", ReturnType: &List{Int}},
		"codepoint_ords": {Name: "codepoint_ords", ReturnType: &List{Int}},
	}
}

func bytesMethods() map[string]*Callable {
	return map[string]*Callable{
		"elems": {Name: "elems", ReturnType: &List{Int}},
	}
}

func listMethods(elem Type) map[string]*Callable {
	return map[string]*Callable{
		"append": {Name: "append", Params: []Param{{Name: "x", Type: elem}}, ReturnType: None},
		"clear":  {Name: "clear", ReturnType: None},
		"copy":   {Name: "copy", ReturnType: &List{elem}},
		"extend": {Name: "extend", Params: []Param{{Name: "x", Type: Any}}, ReturnType: None},
		"index":  {Name: "index", Params: []Param{{Name: "x", Type: elem}, {Name: "start", Type: Int, Optional: true}, {Name: "end", Type: Int, Optional: true}}, ReturnType: Int},
		"insert": {Name: "insert", Params: []Param{{Name: "i", Type: Int}, {Name: "x", Type: elem}}, ReturnType: None},
		"pop":    {Name: "pop", Params: []Param{{Name: "i", Type: Int, Optional: true}}, ReturnType: elem},
		"remove": {Name: "remove", Params: []Param{{Name: "x", Type: elem}}, ReturnType: None},
	}
}

func dictMethods(key, value Type) map[string]*Callable {
	return map[string]*Callable{
		"clear":      {Name: "clear", ReturnType: None},
		"get":        {Name: "get", Params: []Param{{Name: "key", Type: key}, {Name: "default", Type: value, Optional: true}}, ReturnType: value},
		"items":      {Name: "items", ReturnType: &List{&Tuple{Elems: []Type{key, value}}}},
		"keys":       {Name: "keys", ReturnType: &List{key}},
		"values":     {Name: "values", ReturnType: &List{value}},
		"pop":        {Name: "pop", Params: []Param{{Name: "key", Type: key}, {Name: "default", Type: value, Optional: true}}, ReturnType: value},
		"popitem":    {Name: "popitem", ReturnType: &Tuple{Elems: []Type{key, value}}},
		"setdefault": {Name: "setdefault", Params: []Param{{Name: "key", Type: key}, {Name: "default", Type: value, Optional: true}}, ReturnType: value},
		"update":     {Name: "update", Params: []Param{{Name: "pairs", Type: Any, Optional: true}, {Name: "kwargs", Type: Any, StarStar: true}}, ReturnType: None},
	}
}

func setMethods(elem Type) map[string]*Callable {
	return map[string]*Callable{
		"add":   {Name: "add", Params: []Param{{Name: "x", Type: elem}}, ReturnType: None},
		"union": {Name: "union", Params: []Param{{Name: "other", Type: Any}}, ReturnType: &Set{elem}},
	}
}
