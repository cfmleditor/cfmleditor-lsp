package cfparser

import (
	"fmt"
	"regexp"
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
	for _, r := range resolvers {
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
		if resolved := matchResolverPattern(sub, r.Match, r.Resolve); resolved != "" {
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

func matchResolverPattern(expr, pattern, resolve string) string {
	// Build regex from pattern: replace $N placeholders with capture groups
	if !placeholderRe.MatchString(pattern) {
		// No $N in pattern — use pattern as regex directly
		re, err := regexp.Compile("(?i)" + pattern)
		if err != nil {
			if strings.EqualFold(expr, pattern) {
				return resolve
			}
			return ""
		}
		m := re.FindStringSubmatch(expr)
		if m == nil {
			return ""
		}
		// Substitute any capture groups into resolve template
		result := resolve
		for i := 1; i < len(m); i++ {
			result = strings.ReplaceAll(result, fmt.Sprintf("$%d", i), m[i])
		}
		if result == resolve && len(m) == 1 {
			return resolve
		}
		result = strings.TrimSuffix(result, ".cfc")
		result = strings.ReplaceAll(result, "/", ".")
		return result
	}

	// Replace $N with capture groups in the pattern.
	// If the pattern contains backslash sequences (regex escapes), use it as raw regex
	// where capture groups are already defined in the pattern itself.
	// Otherwise, escape literal parts around the placeholders and insert capture groups.
	isRawRegex := strings.Contains(pattern, `\`)
	var reStr string
	if isRawRegex {
		// Raw regex: remove $N references (they refer to existing capture groups)
		reStr = "(?i)" + placeholderRe.ReplaceAllString(pattern, "")
	} else {
		parts := placeholderRe.Split(pattern, -1)
		var b strings.Builder
		b.WriteString("(?i)")
		for i, part := range parts {
			b.WriteString(regexp.QuoteMeta(part))
			if i < len(parts)-1 {
				b.WriteString(`(.+?)`)
			}
		}
		reStr = b.String()
	}

	re, err := regexp.Compile(reStr)
	if err != nil {
		return ""
	}
	m := re.FindStringSubmatch(expr)
	if m == nil {
		return ""
	}

	// Replace $N in resolve template with captured groups
	result := resolve
	for i := 1; i < len(m); i++ {
		captured := strings.Trim(m[i], "\"'")
		result = strings.ReplaceAll(result, fmt.Sprintf("$%d", i), captured)
	}
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
