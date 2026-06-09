package parser

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

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
	Name            string
	URI             uri.URI
	Line            uint32
	Arguments       []Argument
	ReturnType      string // declared return type (e.g. "query", "models.User")
	ReturnComponent string // inferred component from return statements (e.g. "services.Foo")
	returnVar       string // unexported: variable name from "return varName" for deferred resolution
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
	Name       string
	Access     string
	ReturnType string
	Start      int
	End        int
}

// ComponentRef represents a reference to a component instance.
type ComponentRef struct {
	Variable  string
	Component string
	URI       uri.URI
	Line      uint32
}

// DocumentLink represents a file path reference in source (cfinclude, href, etc.).
type DocumentLink struct {
	Path  string // the raw path string
	Line  uint32
	Start uint32 // character offset of path start
	End   uint32 // character offset of path end
}

// CallSite represents a location where a function is called.
type CallSite struct {
	FuncName  string // the function being called
	Component string // resolved component (from variable ref), empty if unresolved
	Variable  string // the variable/qualifier before the dot (empty if bare call)
	Line      uint32
	Caller    string // enclosing function name (empty if global)
	Resolved  bool   // true if qualified (obj.func), false if bare call
	Text      string // the trimmed line text
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
	re      *regexp.Regexp // compiled regex, lazily initialized
	simple  bool           // true if pattern is a plain string (no regex, no $N)
	reOnce  sync.Once
	// Precomputed for simple matches
	simplePrefix   string // part before $1
	simpleSuffix   string // part after $1
	normPrefix     string // prefix with " replaced by '
	normSuffix     string // suffix with " replaced by '
	hasPlaceholder bool
}

// isRegexPattern returns true if the pattern contains backslash escapes (definitive regex indicator).
func isRegexPattern(s string) bool {
	return strings.Contains(s, `\`)
}

// compiledRe ensures the resolver's regex/simple-match fields are initialized.
// Sets r.simple=true if the pattern needs no regex.
func (r *Resolver) compiledRe() {
	r.reOnce.Do(func() {
		pattern := r.Match
		hasPlaceholder := placeholderRe.MatchString(pattern)
		r.hasPlaceholder = hasPlaceholder

		switch {
		case !hasPlaceholder && !isRegexPattern(pattern):
			r.simple = true
		case hasPlaceholder && !isRegexPattern(pattern) && !strings.Contains(pattern, "$2"):
			r.simple = true
			before, after, _ := strings.Cut(pattern, "$1")
			r.simplePrefix = before
			r.simpleSuffix = after
			r.normPrefix = strings.ReplaceAll(before, `"`, `'`)
			r.normSuffix = strings.ReplaceAll(after, `"`, `'`)
		case !hasPlaceholder:
			re, err := regexp.Compile("(?i)" + pattern)
			if err == nil {
				r.re = re
			}
		case strings.Contains(pattern, `\`):
			reStr := "(?i)" + placeholderRe.ReplaceAllString(pattern, "")

			re, err := regexp.Compile(reStr)
			if err == nil {
				r.re = re
			}
		default:
			parts := placeholderRe.Split(pattern, -1)

			var b strings.Builder

			b.WriteString("(?i)")

			for i, part := range parts {
				b.WriteString(regexp.QuoteMeta(part))

				if i < len(parts)-1 {
					b.WriteString(`(.+?)`)
				}
			}

			re, err := regexp.Compile(b.String())
			if err == nil {
				r.re = re
			}
		}
	})
}

// PropertyResolver maps a property attribute value to a component path.
type PropertyResolver struct {
	Match     string // pattern to match against the attribute value, $1 is capture placeholder
	Resolve   string // component dot-path template, $1 replaced with captured value
	Attribute string // property attribute to inspect (e.g. "inject")
}

// ResolveProperty matches a property's attributes against property resolvers
// and returns the resolved component dot-path, or empty string.
func ResolveProperty(attrs map[string]string, resolvers []PropertyResolver) string {
	for _, r := range resolvers {
		val, ok := attrs[strings.ToLower(r.Attribute)]
		if !ok || val == "" {
			continue
		}

		if resolved := matchPropertyPattern(val, r.Match, r.Resolve); resolved != "" {
			return resolved
		}
	}

	return ""
}

func matchPropertyPattern(value, pattern, resolve string) string {
	before, after, ok := strings.Cut(pattern, "$1")
	if !ok {
		// Exact match
		if strings.EqualFold(value, pattern) {
			return resolve
		}

		return ""
	}

	prefix := before
	suffix := after

	if len(value) < len(prefix)+len(suffix) {
		return ""
	}

	if !strings.EqualFold(value[:len(prefix)], prefix) {
		return ""
	}

	if !strings.EqualFold(value[len(value)-len(suffix):], suffix) {
		return ""
	}

	captured := value[len(prefix) : len(value)-len(suffix)]
	resolved := strings.ReplaceAll(resolve, "$1", captured)
	resolved = strings.TrimSuffix(resolved, ".cfc")
	resolved = strings.ReplaceAll(resolved, "/", ".")

	return resolved
}

// ResolverSet groups resolvers by the first byte of their prefix for fast rejection.
type ResolverSet struct {
	resolvers []Resolver
	byByte    [256][]int // lowercase first byte → resolver indices
	built     bool
}

// BuildResolverSet creates an optimized resolver set from a slice of resolvers.
func BuildResolverSet(resolvers []Resolver) *ResolverSet {
	rs := &ResolverSet{resolvers: resolvers}
	for i := range resolvers {
		if resolvers[i].Prefix == "" {
			continue
		}

		b := resolvers[i].Prefix[0]
		if b >= 'A' && b <= 'Z' {
			b += 32
		}

		rs.byByte[b] = append(rs.byByte[b], i)
	}

	rs.built = true

	return rs
}

// ResolveFromCall matches an expression against resolvers and returns the component dot-path.
func ResolveFromCall(expr string, resolvers []Resolver) string {
	expr = strings.TrimSpace(expr)

	for i := range resolvers {
		r := &resolvers[i]
		if r.Prefix == "" {
			continue
		}
		// Find prefix position and only match from there, limited forward
		idx := indexFold(expr, r.Prefix)
		if idx < 0 {
			continue
		}

		sub := expr[idx:]
		if len(sub) > 200 {
			sub = sub[:200]
		}

		if resolved := matchResolverWithCache(sub, r); resolved != "" {
			return resolved
		}
	}

	return ""
}

// Resolve matches an expression against the pre-grouped resolvers.
func (rs *ResolverSet) Resolve(expr string) string {
	if rs == nil {
		return ""
	}

	expr = strings.TrimSpace(expr)
	if len(expr) == 0 {
		return ""
	}

	// Collect unique resolver indices whose prefix first-byte appears in expr
	var (
		seen       [256]bool
		candidates []int
	)

	for i := 0; i < len(expr); i++ {
		b := expr[i]
		if b >= 'A' && b <= 'Z' {
			b += 32
		}

		if seen[b] {
			continue
		}

		seen[b] = true
		candidates = append(candidates, rs.byByte[b]...)
	}

	for _, idx := range candidates {
		r := &rs.resolvers[idx]

		pos := indexFold(expr, r.Prefix)
		if pos < 0 {
			continue
		}

		sub := expr[pos:]
		if len(sub) > 200 {
			sub = sub[:200]
		}

		if resolved := matchResolverWithCache(sub, r); resolved != "" {
			return resolved
		}
	}

	return ""
}

func indexFold(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if strings.EqualFold(s[i:i+len(substr)], substr) {
			return i
		}
	}

	return -1
}

// placeholderRe matches $1, $2, etc. in patterns.
var placeholderRe = regexp.MustCompile(`\$(\d+)`)

func matchResolverWithCache(expr string, r *Resolver) string {
	r.compiledRe() // ensure reOnce has run

	if r.simple {
		return simpleMatch(expr, r)
	}

	if r.re == nil {
		return ""
	}

	m := r.re.FindStringSubmatch(expr)
	if m == nil {
		return ""
	}

	result := r.Resolve

	for i := 1; i < len(m); i++ {
		captured := strings.Trim(m[i], "\"'")
		result = strings.ReplaceAll(result, fmt.Sprintf("$%d", i), captured)
	}

	if result == r.Resolve && len(m) == 1 {
		return result
	}

	result = strings.TrimSuffix(result, ".cfc")
	result = strings.ReplaceAll(result, "/", ".")

	return result
}

// simpleMatch handles patterns with $1 placeholder or exact match using string ops only.
// Quote characters (" and ') in the pattern match either quote type in the expression.
func simpleMatch(expr string, r *Resolver) string {
	if !r.hasPlaceholder {
		// Exact match — also accept match() form
		if strings.EqualFold(expr, r.Match) {
			return r.Resolve
		}

		if strings.HasSuffix(expr, "()") && strings.EqualFold(expr[:len(expr)-2], r.Match) {
			return r.Resolve
		}

		return ""
	}

	prefix := r.simplePrefix
	suffix := r.simpleSuffix
	normPrefix := r.normPrefix
	normSuffix := r.normSuffix

	if len(expr) < len(prefix)+len(suffix) {
		return ""
	}

	exprPrefix := strings.ReplaceAll(expr[:len(prefix)], `"`, `'`)
	if !strings.EqualFold(exprPrefix, normPrefix) {
		return ""
	}

	var captured string

	if normSuffix != "" {
		// Find first occurrence of suffix after prefix (not last)
		rest := expr[len(prefix):]
		normRest := strings.ReplaceAll(rest, `"`, `'`)

		suffixIdx := indexFold(normRest, normSuffix)
		if suffixIdx < 0 {
			return ""
		}

		captured = rest[:suffixIdx]
	} else {
		captured = expr[len(prefix):]
	}

	captured = strings.Trim(captured, "\"'")
	result := strings.ReplaceAll(r.Resolve, "$1", captured)
	result = strings.TrimSuffix(result, ".cfc")
	result = strings.ReplaceAll(result, "/", ".")

	return result
}

func containsFold(s, substr string) bool {
	n := len(substr)
	if n == 0 {
		return true
	}

	if n > len(s) {
		return false
	}

	for i := 0; i <= len(s)-n; i++ {
		match := true

		for j := range n {
			if s[i+j]|0x20 != substr[j]|0x20 {
				match = false

				break
			}
		}

		if match {
			return true
		}
	}

	return false
}

// isComponentType returns true if a type string looks like a component path
// (contains a dot) rather than a primitive type.
func isComponentType(t string) bool {
	if t == "" {
		return false
	}

	return strings.Contains(t, ".")
}

// FormatHover returns a markdown-formatted hover string for the function definition.
func (def *FunctionDef) FormatHover() string {
	var b strings.Builder

	b.WriteString("**")
	b.WriteString(def.Name)
	b.WriteString("**\n\n```cfml\n")
	b.WriteString(def.Name)
	b.WriteString("(")

	for i, arg := range def.Arguments {
		if i > 0 {
			b.WriteString(", ")
		}

		if arg.Required {
			b.WriteString("required ")
		}

		if arg.Type != "" {
			b.WriteString(arg.Type)
			b.WriteString(" ")
		}

		b.WriteString(arg.Name)
	}

	b.WriteString(")\n```")

	return b.String()
}
