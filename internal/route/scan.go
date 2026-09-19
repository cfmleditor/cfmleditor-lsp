package route

import (
	"sort"
	"strings"
)

// Source says which syntax a route was written in.
type Source string

// The syntaxes a route can appear in.
const (
	// SourceAttribute is an HTML attribute: data-view="a.b.c".
	SourceAttribute Source = "attribute"

	// SourceQueryParam is a URL parameter: href="index.cfm?do=a.b.c".
	SourceQueryParam Source = "query"

	// SourceProperty is a JavaScript object key: { view: "a.b.c" }.
	SourceProperty Source = "property"

	// SourceFunctionArg is a function's first string argument: redirect("a.b.c").
	SourceFunctionArg Source = "function"
)

// Ref is one route string found in source, with enough position for an editor to
// put a link on it.
type Ref struct {
	Source Source // which syntax it was written in
	Name   string // the attribute, parameter or property name, lower-cased
	Value  string // the route, as written
	Line   uint32 // 0-based, as the parser numbers lines
	Start  uint32 // byte offset of the value's first character, from the file start
	End    uint32 // byte offset one past its last character
	Col    uint32 // 0-based byte column of the value's first character
}

// Scan finds every route in content.
//
// A byte scan rather than a tree-sitter walk on purpose. Routes live in HTML
// interleaved with CFML tags, in URLs inside attribute values, and in JavaScript
// object literals inside <script> blocks — places where the CST either has no node
// for the thing or hands back the whole run as one opaque token. A scan finds them
// all, and the cost of a wrong one is a route that resolves to nothing and
// produces no edge, so over-reaching here is cheap and under-reaching is not.
//
// Matching is done first and positions are assigned afterwards, in one pass over
// the matches in offset order. Tracking lines inside the matchers meant every one
// of them had to remember to count the newlines inside a value it skipped over,
// which is the sort of bookkeeping that is right until a fourth syntax is added.
func Scan(content string, cfg Config) []Ref {
	if content == "" {
		return nil
	}

	found := make([]Ref, 0, 8)
	found = append(found, scanAttributes(content, normalise(cfg.Attributes))...)
	found = append(found, scanQueryParams(content, normalise(cfg.QueryParams))...)
	found = append(found, scanProperties(content, normalise(cfg.Properties))...)
	found = append(found, scanFunctionArgs(content, normalise(cfg.Functions))...)

	if len(found) == 0 {
		return nil
	}

	sort.Slice(found, func(i, j int) bool { return found[i].Start < found[j].Start })

	// An attribute value can hold a query parameter — href="x.cfm?do=a.b.c" is
	// matched by both an attribute rule for href and a query rule for do — so of
	// two overlapping matches the shorter wins. The inner one is the route; the
	// outer is the URL that carries it, and keeping that instead would leave every
	// link on the page as an unresolvable ref.
	deduped := found[:0]

	for _, r := range found {
		if n := len(deduped); n > 0 && r.Start < deduped[n-1].End {
			if r.End-r.Start < deduped[n-1].End-deduped[n-1].Start {
				deduped[n-1] = r
			}

			continue
		}

		deduped = append(deduped, r)
	}

	assignPositions(content, deduped)

	return deduped
}

// assignPositions fills Line and Col from each match's byte offset, walking the
// content once. The refs must already be sorted by Start.
func assignPositions(content string, refs []Ref) {
	line := uint32(0)
	lineStart := 0
	at := 0

	for i := range refs {
		for ; at < int(refs[i].Start) && at < len(content); at++ {
			if content[at] == '\n' {
				line++
				lineStart = at + 1
			}
		}

		refs[i].Line = line
		refs[i].Col = refs[i].Start - uint32(lineStart)
	}
}

// scanAttributes finds name="value" and name='value'.
func scanAttributes(content string, names []string) []Ref {
	if len(names) == 0 {
		return nil
	}

	var out []Ref

	for i := 0; i < len(content); i++ {
		if !isNameStart(content[i]) || (i > 0 && isNamePart(content[i-1])) {
			continue
		}

		name, start, end, next := matchNamedValue(content, i, names, true)
		if name == "" {
			continue
		}

		if value := content[start:end]; value != "" {
			out = append(out, Ref{
				Source: SourceAttribute, Name: name, Value: value,
				Start: uint32(start), End: uint32(end),
			})
		}

		i = next - 1
	}

	return out
}

// scanProperties finds key: "value", "key": "value" and 'key': 'value'.
func scanProperties(content string, names []string) []Ref {
	if len(names) == 0 {
		return nil
	}

	var out []Ref

	for i := 0; i < len(content); i++ {
		at := i

		// A quoted key: step over the opening quote so the name matcher sees the
		// name, and require the matching quote before the colon.
		quoted := byte(0)
		if content[i] == '"' || content[i] == '\'' {
			quoted = content[i]
			at = i + 1
		}

		if at >= len(content) || !isNameStart(content[at]) {
			continue
		}

		if quoted == 0 && at > 0 && isPropNamePart(content[at-1]) {
			continue
		}

		name, start, end, next := matchPropertyValue(content, at, names, quoted)
		if name == "" {
			continue
		}

		if value := content[start:end]; value != "" {
			out = append(out, Ref{
				Source: SourceProperty, Name: name, Value: value,
				Start: uint32(start), End: uint32(end),
			})
		}

		i = next - 1
	}

	return out
}

// scanQueryParams finds ?name=value and &name=value.
//
// The value is unquoted — it sits inside the enclosing href's quotes — so it runs
// to the next URL delimiter. "&amp;" is treated as a delimiter too, because that
// is how an ampersand is written in the markup these links live in and a value
// that swallowed it would never resolve.
func scanQueryParams(content string, names []string) []Ref {
	if len(names) == 0 {
		return nil
	}

	var out []Ref

	for i := 0; i < len(content); i++ {
		if content[i] != '?' && content[i] != '&' {
			continue
		}

		at := i + 1
		if strings.HasPrefix(strings.ToLower(content[min(at, len(content)):]), "amp;") {
			at += 4
		}

		name, start, end, next := matchNamedValue(content, at, names, false)
		if name == "" {
			continue
		}

		if value := content[start:end]; value != "" {
			out = append(out, Ref{
				Source: SourceQueryParam, Name: name, Value: value,
				Start: uint32(start), End: uint32(end),
			})
		}

		i = next - 1
	}

	return out
}

// matchNamedValue reads `name=value` at i. With quotedOnly the value must be
// quoted; otherwise an unquoted value runs to the next URL delimiter.
func matchNamedValue(content string, i int, names []string, quotedOnly bool) (string, int, int, int) {
	for _, name := range names {
		if len(content)-i < len(name)+2 || !strings.EqualFold(content[i:i+len(name)], name) {
			continue
		}

		// The name must end here, or "do" would match inside "domain".
		j := i + len(name)
		if j < len(content) && isNamePart(content[j]) {
			continue
		}

		j = skipSpace(content, j)
		if j >= len(content) || content[j] != '=' {
			continue
		}

		j = skipSpace(content, j+1)
		if j >= len(content) {
			continue
		}

		if content[j] == '"' || content[j] == '\'' {
			quote := content[j]
			j++
			start := j

			for j < len(content) && content[j] != quote && content[j] != '\n' {
				j++
			}

			if j >= len(content) || content[j] != quote {
				continue
			}

			return name, start, j, j + 1
		}

		if quotedOnly {
			continue
		}

		start := j
		for j < len(content) && !isURLDelimiter(content[j]) {
			j++
		}

		return name, start, j, j
	}

	return "", 0, 0, 0
}

// matchPropertyValue reads `name: "value"` at i, where quote is the quote that
// opened the key name, or 0 when the key is bare.
func matchPropertyValue(content string, i int, names []string, keyQuote byte) (string, int, int, int) {
	for _, name := range names {
		if len(content)-i < len(name)+3 || !strings.EqualFold(content[i:i+len(name)], name) {
			continue
		}

		// isNamePart is not used here: it counts ":" as part of a name, which is
		// right for an attribute like xlink:href and exactly wrong for an object
		// key, where the colon is the thing that ends the name.
		j := i + len(name)
		if j < len(content) && isPropNamePart(content[j]) {
			continue
		}

		if keyQuote != 0 {
			if j >= len(content) || content[j] != keyQuote {
				continue
			}

			j++
		}

		j = skipSpace(content, j)
		if j >= len(content) || content[j] != ':' {
			continue
		}

		j = skipSpace(content, j+1)
		if j >= len(content) || (content[j] != '"' && content[j] != '\'') {
			continue
		}

		quote := content[j]
		j++
		start := j

		for j < len(content) && content[j] != quote && content[j] != '\n' {
			j++
		}

		if j >= len(content) || content[j] != quote {
			continue
		}

		return name, start, j, j + 1
	}

	return "", 0, 0, 0
}

// scanFunctionArgs finds name("value") and name('value'), taking the first
// argument.
//
// The value is cut at the first URL delimiter, because a route passed to a
// redirect routinely carries what follows it in the URL:
// redirect('intranet.client.detail&customercode=x') names the route
// intranet.client.detail. Rejecting the whole string for containing an ampersand
// would lose every one of those.
func scanFunctionArgs(content string, names []string) []Ref {
	if len(names) == 0 {
		return nil
	}

	var out []Ref

	for i := 0; i < len(content); i++ {
		if !isNameStart(content[i]) || (i > 0 && isPropNamePart(content[i-1])) {
			continue
		}

		name, start, end, next := matchCallArg(content, i, names)
		if name == "" {
			continue
		}

		// Trim at the delimiter, keeping Start/End on what is left so an editor
		// underlines the route and not the query string after it.
		value := content[start:end]
		for j := 0; j < len(value); j++ {
			if isURLDelimiter(value[j]) {
				value = value[:j]

				break
			}
		}

		if value != "" {
			out = append(out, Ref{
				Source: SourceFunctionArg, Name: name, Value: value,
				Start: uint32(start), End: uint32(start + len(value)),
			})
		}

		i = next - 1
	}

	return out
}

// matchCallArg reads `name("value"` at i.
func matchCallArg(content string, i int, names []string) (string, int, int, int) {
	for _, name := range names {
		if len(content)-i < len(name)+3 || !strings.EqualFold(content[i:i+len(name)], name) {
			continue
		}

		j := i + len(name)
		if j < len(content) && isPropNamePart(content[j]) {
			continue
		}

		j = skipSpace(content, j)
		if j >= len(content) || content[j] != '(' {
			continue
		}

		j = skipSpace(content, j+1)
		if j >= len(content) || (content[j] != '"' && content[j] != '\'') {
			continue
		}

		quote := content[j]
		j++
		start := j

		for j < len(content) && content[j] != quote && content[j] != '\n' {
			j++
		}

		if j >= len(content) || content[j] != quote {
			continue
		}

		return name, start, j, j + 1
	}

	return "", 0, 0, 0
}

func isURLDelimiter(c byte) bool {
	switch c {
	case '&', '"', '\'', '#', '<', '>', ' ', '\t', '\r', '\n', ')', '}':
		return true
	default:
		return false
	}
}

func isNameStart(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_'
}

func isNamePart(c byte) bool {
	return isPropNamePart(c) || c == ':'
}

// isPropNamePart is isNamePart without the colon, for JavaScript object keys.
func isPropNamePart(c byte) bool {
	return isNameStart(c) || c >= '0' && c <= '9' || c == '-' || c == '$'
}

func skipSpace(content string, i int) int {
	for i < len(content) && (content[i] == ' ' || content[i] == '\t' || content[i] == '\r' || content[i] == '\n') {
		i++
	}

	return i
}
