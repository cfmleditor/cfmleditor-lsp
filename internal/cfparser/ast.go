package cfparser

import "go.lsp.dev/uri"

// Argument represents a parameter of a user-defined function.
type Argument struct {
	Name     string
	Type     string // empty if untyped
	Required bool
}

// FunctionDef represents a user-defined function found in a CFC file.
type FunctionDef struct {
	Name      string
	URI       uri.URI
	Line      uint32
	Arguments []Argument
}

// Scope represents the CFML variable scope.
type Scope int

// Scope enumerates variable scope qualifiers.
const (
	ScopeLocal     Scope = iota // var x or local.x
	ScopeArguments              // arguments.x
	ScopeThis                   // this.x
	ScopeVariables              // variables.x or unscoped assignment
)

// VarDef represents a variable declaration in source.
type VarDef struct {
	Name      string
	Scope     Scope
	Line      uint32
	FuncStart int // -1 if file-scope
	FuncEnd   int // -1 if file-scope
}

// FuncScope represents a function's line range.
type FuncScope struct {
	Start int
	End   int
}

// ComponentRef represents a reference to a component instance.
type ComponentRef struct {
	Variable  string
	Component string
	URI       uri.URI
	Line      uint32
}

// RegionKind classifies a span of CFC content.
type RegionKind int

// RegionKind values.
const (
	RegionScript RegionKind = iota
	RegionTag
)

// Region is a contiguous span of source with a single kind.
type Region struct {
	Kind      RegionKind
	StartLine int
	Text      string
}
