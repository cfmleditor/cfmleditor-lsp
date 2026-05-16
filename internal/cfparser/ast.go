package cfparser

import (
	"strings"

	"go.lsp.dev/uri"
)

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

// Resolver maps a call pattern to a component path.
type Resolver struct {
	Match   string
	Resolve string
	Prefix  string
}

// ResolveFromCall matches an expression against resolvers and returns the component dot-path.
func ResolveFromCall(expr string, resolvers []Resolver) string {
	expr = strings.TrimSpace(expr)
	for _, r := range resolvers {
		if r.Prefix != "" && !containsFold(expr, r.Prefix) {
			continue
		}
		if val := matchResolverPattern(expr, r.Match); val != "" {
			resolved := strings.ReplaceAll(r.Resolve, "$1", val)
			resolved = strings.TrimSuffix(resolved, ".cfc")
			resolved = strings.ReplaceAll(resolved, "/", ".")
			return resolved
		}
	}
	return ""
}

func matchResolverPattern(expr, pattern string) string {
	idx := strings.Index(pattern, "$1")
	if idx < 0 {
		if strings.EqualFold(expr, pattern) {
			return "1"
		}
		return ""
	}
	prefix := pattern[:idx]
	suffix := pattern[idx+2:]
	// Find the prefix within the expression (may be preceded by qualifier like VARIABLES._parent.)
	start := -1
	for i := 0; i <= len(expr)-len(prefix); i++ {
		if strings.EqualFold(expr[i:i+len(prefix)], prefix) {
			start = i
			break
		}
	}
	if start < 0 {
		return ""
	}
	rest := expr[start+len(prefix):]
	if len(suffix) > 0 {
		if !strings.EqualFold(rest[len(rest)-len(suffix):], suffix) {
			return ""
		}
		rest = rest[:len(rest)-len(suffix)]
	}
	return strings.Trim(rest, "\"'")
}

func containsFold(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if strings.EqualFold(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}
