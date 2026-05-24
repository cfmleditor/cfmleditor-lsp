package cfparser

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
}

// isRegexPattern returns true if the pattern contains backslash escapes (definitive regex indicator).
func isRegexPattern(s string) bool {
	return strings.Contains(s, `\`)
}

// compiledRe returns the compiled regex for this resolver, caching it.
// Sets r.simple=true if the pattern needs no regex.
func (r *Resolver) compiledRe() *regexp.Regexp {
	r.reOnce.Do(func() {
		pattern := r.Match
		hasPlaceholder := placeholderRe.MatchString(pattern)
		switch {
		case !hasPlaceholder && !isRegexPattern(pattern):
			// No placeholders, no regex chars — simple exact match
			r.simple = true
		case hasPlaceholder && !isRegexPattern(pattern) && !strings.Contains(pattern, "$2"):
			// Has $1 only, no regex chars — simple prefix/suffix match
			r.simple = true
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
	return r.re
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
	idx := strings.Index(pattern, "$1")
	if idx < 0 {
		// Exact match
		if strings.EqualFold(value, pattern) {
			return resolve
		}
		return ""
	}
	prefix := pattern[:idx]
	suffix := pattern[idx+2:]
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
		return simpleMatch(expr, r.Match, r.Resolve)
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
func simpleMatch(expr, pattern, resolve string) string {
	idx := strings.Index(pattern, "$1")
	if idx < 0 {
		// Exact match
		if strings.EqualFold(expr, pattern) {
			return resolve
		}
		return ""
	}
	prefix := pattern[:idx]
	suffix := pattern[idx+2:]

	// Normalize quotes in prefix/suffix for matching: replace " with ' in both
	normPrefix := strings.ReplaceAll(prefix, `"`, `'`)
	normSuffix := strings.ReplaceAll(suffix, `"`, `'`)

	if len(expr) < len(prefix)+len(suffix) {
		return ""
	}

	exprPrefix := strings.ReplaceAll(expr[:len(prefix)], `"`, `'`)
	if !strings.EqualFold(exprPrefix, normPrefix) {
		return ""
	}

	if normSuffix != "" {
		exprSuffix := strings.ReplaceAll(expr[len(expr)-len(suffix):], `"`, `'`)
		if !strings.EqualFold(exprSuffix, normSuffix) {
			return ""
		}
	}

	captured := expr[len(prefix):]
	if suffix != "" {
		captured = captured[:len(captured)-len(suffix)]
	}
	captured = strings.Trim(captured, "\"'")
	result := strings.ReplaceAll(resolve, "$1", captured)
	result = strings.TrimSuffix(result, ".cfc")
	result = strings.ReplaceAll(result, "/", ".")
	return result
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
