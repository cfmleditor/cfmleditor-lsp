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
	Hint     string // hint attribute value (used as supplemental type when Type is generic)
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

	// ChainBase and ChainMethod record the receiver.method() shape that produced
	// this ref (e.g. "var x = jss.getInstance()" → ChainBase "jss", ChainMethod
	// "getInstance"). Empty unless Component came from a receiver.method(...) call.
	// Used by applyChainedReturnLookup to prefer the callee's own declared return
	// type (verified via FuncLookup) over a componentResolver guess on the RHS text.
	ChainBase   string
	ChainMethod string
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

	// Chain lists the method-name hops between the resolved base (Variable or
	// Component) and this call, for calls chained off a prior call's return value
	// rather than a plain variable (e.g. "kpg.generateKeyPair().getPublic().getParams()":
	// the getParams CallSite has Variable "kpg" and Chain []string{"generateKeyPair",
	// "getPublic"}). Empty for a plain "x.method()" call. CanResolveCall walks Chain
	// via ResolveFunc, applying each hop's declared return type in turn, before
	// checking FuncName against the final component.
	Chain []string
}

// RegionKind classifies a span of CFC content.
type RegionKind int

// RegionKind values.
const (
	RegionScript RegionKind = iota
	RegionTag
	// RegionSkip is opaque content — currently a literal <script>...</script>
	// block containing no CFML of its own — that is deliberately left
	// unparsed. See findScriptSkipSpans in cfparser.go.
	RegionSkip
)

// Region is a contiguous span of source with a single kind.
type Region struct {
	Kind      RegionKind
	StartLine int
	Text      string
}

// Resolver maps a call pattern to a component path.
type Resolver struct {
	Match    string
	Resolve  string
	Prefix   string
	NoFollow bool // if true, skip method verification when this resolver matches
	// Anchored requires Prefix to sit at the very start of the expression rather
	// than anywhere inside it. See findPrefixPos.
	Anchored bool
	re       *regexp.Regexp // compiled regex, lazily initialized
	simple   bool           // true if pattern is a plain string (no regex, no $N)
	reOnce   sync.Once
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

		seenByte := [256]bool{}

		for _, alt := range splitPrefix(resolvers[i].Prefix) {
			if alt == "" {
				continue
			}

			b := alt[0]
			if b >= 'A' && b <= 'Z' {
				b += 32
			}

			if seenByte[b] {
				continue
			}

			seenByte[b] = true
			rs.byByte[b] = append(rs.byByte[b], i)
		}
	}

	rs.built = true

	return rs
}

// splitPrefix splits a `prefix` field on `|` into its alternatives, allowing a single
// resolver to share one match/resolve pair across multiple fast-check prefixes
// (e.g. "getPageTools|getLockBroker") instead of requiring a separate resolver entry
// per prefix. A prefix with no `|` returns a single-element slice unchanged.
func splitPrefix(prefix string) []string {
	if !strings.Contains(prefix, "|") {
		return []string{prefix}
	}

	return strings.Split(prefix, "|")
}

// prefixContainsFold reports whether expr contains any of prefix's pipe-delimited
// alternatives as a case-insensitive substring. Used by the quick-rejection checks
// that don't need the match position, only a yes/no signal.
func prefixContainsFold(expr, prefix string) bool {
	for _, alt := range splitPrefix(prefix) {
		if alt != "" && containsFold(expr, alt) {
			return true
		}
	}

	return false
}

// prefixEqualFold reports whether expr case-insensitively equals any of prefix's
// pipe-delimited alternatives.
func prefixEqualFold(expr, prefix string) bool {
	for _, alt := range splitPrefix(prefix) {
		if alt != "" && strings.EqualFold(expr, alt) {
			return true
		}
	}

	return false
}

// findPrefixPos returns the position of the first matching prefix alternative in expr,
// or -1 if none match. Which alternative matched doesn't affect the subsequent
// Match/Resolve step, since only the matched position (not the matched text) is used
// to slice expr for matching.
//
// Unanchored (the default) finds the prefix anywhere in expr, which is why a resolver
// with prefix "document" also fires on "domobject_document", and why a catch-all like
// prefix "get" fires on the "get" inside "VARIABLES._document.getDirectContent()".
// Both slice expr from the prefix position, so the short pattern then matches exactly
// and produces a confidently wrong component.
//
// Anchored requires an alternative at position 0, so a resolver only ever claims an
// expression that genuinely starts with its prefix. It also makes the order of
// pipe-delimited alternatives irrelevant: every alternative that matches matches at 0,
// so a shorter alternative can no longer fix the slice position ahead of a longer one.
func findPrefixPos(expr, prefix string, anchored bool) int {
	for _, alt := range splitPrefix(prefix) {
		if alt == "" {
			continue
		}

		if anchored {
			if len(expr) >= len(alt) && strings.EqualFold(expr[:len(alt)], alt) {
				return 0
			}

			continue
		}

		if idx := indexFold(expr, alt); idx >= 0 {
			return idx
		}
	}

	return -1
}

// ResolveFromCallFull matches an expression against resolvers and returns the component dot-path
// and whether the matching resolver has NoFollow set.
func ResolveFromCallFull(expr string, resolvers []Resolver) (string, bool) {
	comp, noFollow, _ := ResolveFromCallMatch(expr, resolvers)

	return comp, noFollow
}

// ResolveFromCallMatch is ResolveFromCallFull plus the index of the resolver that matched
// (-1 when none did). A wrong component produced by a resolver firing on a substring of an
// unrelated identifier is indistinguishable from a correct one in the result alone, so the
// explain trace needs to be able to name which entry claimed the expression.
func ResolveFromCallMatch(expr string, resolvers []Resolver) (string, bool, int) {
	expr = strings.TrimSpace(expr)

	for i := range resolvers {
		r := &resolvers[i]
		if r.Prefix == "" {
			continue
		}
		// Find prefix position and only match from there, limited forward
		idx := findPrefixPos(expr, r.Prefix, r.Anchored)
		if idx < 0 {
			continue
		}

		sub := expr[idx:]
		if len(sub) > 200 {
			sub = sub[:200]
		}

		if resolved := matchResolverWithCache(sub, r); resolved != "" {
			return resolved, r.NoFollow, i
		}
	}

	return "", false, -1
}

// Describe renders a resolver as its config fields, for trace output that has to identify
// which entry of componentResolvers fired.
func (r *Resolver) Describe() string {
	desc := fmt.Sprintf("match %q, prefix %q", r.Match, r.Prefix)

	if r.Anchored {
		desc += ", anchored"
	}

	return desc
}

// ResolveFromCall matches an expression against resolvers and returns the component dot-path.
func ResolveFromCall(expr string, resolvers []Resolver) string {
	comp, _ := ResolveFromCallFull(expr, resolvers)

	return comp
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

		pos := findPrefixPos(expr, r.Prefix, r.Anchored)
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

// substitutePlaceholder replaces $N and its case-folded ${N:lower}/${N:upper} variants in
// result with captured. The case-folded forms let a resolver normalize a captured call-site
// value — typically written in whatever case the caller used — to match a lowercase (or
// uppercase) file/package naming convention, e.g. `resolve: "packages.tass.${1:lower}"` for
// `match: "get$1()"` turns `getPageTools()` into `packages.tass.pagetools`.
func substitutePlaceholder(result string, n int, captured string) string {
	result = strings.ReplaceAll(result, fmt.Sprintf("${%d:lower}", n), strings.ToLower(captured))
	result = strings.ReplaceAll(result, fmt.Sprintf("${%d:upper}", n), strings.ToUpper(captured))
	result = strings.ReplaceAll(result, fmt.Sprintf("$%d", n), captured)

	return result
}

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
		result = substitutePlaceholder(result, i, captured)
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
		// Exact match — also accept match() or match(...) form for method-call patterns
		if strings.EqualFold(expr, r.Match) {
			return r.Resolve
		}

		if len(expr) > len(r.Match) && expr[len(r.Match)] == '(' && strings.EqualFold(expr[:len(r.Match)], r.Match) {
			// Only allow match(...) when the pattern contains a dot (method call).
			// Plain variable names like "document" should not match "document(args)"
			// because prefix matching may have found it inside "createDocument(args)".
			if strings.ContainsRune(r.Match, '.') {
				return r.Resolve
			}

			// For patterns without a dot, only allow empty parens
			if expr[len(r.Match)+1:] == ")" || strings.HasPrefix(expr[len(r.Match):], "()") {
				return r.Resolve
			}
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
	result := substitutePlaceholder(r.Resolve, 1, captured)
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

// isComponentType returns true if a type string looks like a genuine dotted
// component path (e.g. "models.User") rather than a primitive type or free
// text. Used both for explicit type/returntype attributes and for hint
// promotion — where a prose hint that merely ends in a sentence "." (e.g.
// "Get Reference to Java Utilities.") must NOT be mistaken for a component
// path, so this requires every dot-separated segment to be a valid
// identifier, not just "contains a dot somewhere".
func isComponentType(t string) bool {
	if t == "" || !strings.Contains(t, ".") {
		return false
	}

	for seg := range strings.SplitSeq(t, ".") {
		if !isIdentifier(seg) {
			return false
		}
	}

	return true
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
