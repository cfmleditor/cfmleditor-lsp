package resolve

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/cfmleditor/clif/internal/conv"
	"github.com/cfmleditor/clif/internal/parser"
	cfpath "github.com/cfmleditor/clif/internal/path"
)

// A loop variable holds an element of the collection it iterates:
// `for ( var c in getComments() )` and `<cfloop array="#comments#" index="c">`
// give c an entity whenever the collection's element type is in the source. The
// parser files a loop variable as a declaration with no component, so a call on
// it read as one on an untyped variable.
//
// The collection's element type comes from what the source states:
//
//   - a persistent entity's one-to-many or many-to-many property, through its
//     generated getter (FunctionDef.ElementComponent), whether the loop calls the
//     getter or reads the property;
//   - a cborm service bound to an entity, whose getAll() returns an array of it,
//     unless it is asked for `properties`, when it returns structs;
//   - a local variable assigned from either, by its nearest preceding assignment
//     in the function, the rule a receiver's own ref follows.
//
// Anything else gives no element type and the variable stays untyped.

var (
	loopTagRe      = regexp.MustCompile(`(?is)<(/?)cfloop\b([^>]*)>`)
	loopAttrRe     = regexp.MustCompile(`(?i)\b(array|index|item)\s*=\s*["']([^"']*)["']`)
	loopCallRe     = regexp.MustCompile(`^(?:([\w.]+)\.)?(\w+)\s*\((.*)\)$`)
	loopPathRe     = regexp.MustCompile(`^(?:(local|variables|this|arguments)\.)?(\w+)$`)
	maxElementHops = 4
)

// loopElement is the component call's receiver holds as the variable of a loop
// around the call, or "".
func (r *Resolver) loopElement(call *parser.CallSite, pr *parser.ParseResult, baseDir string, tr *callTrace) string {
	name := strings.TrimPrefix(strings.ToLower(call.Variable), "local.")
	if name == "" || strings.ContainsAny(name, ".[") || pr == nil {
		return ""
	}

	start, end, caller := 0, -1, ""
	if scope, ok := enclosingScope(pr, call.Line); ok {
		start, end, caller = scope.Start, scope.End, scope.Name
	}

	collection, header, ok := enclosingLoop(r.loopsOf(pr), name, int(call.Line), start, end)
	if !ok {
		return ""
	}

	comp := r.elementOf(collection, header, start, caller, pr, baseDir, 0, "")
	if comp != "" {
		tr.addf("%q is the variable of a loop over %q, whose elements are %q", call.Variable, collection, comp)
	}

	return comp
}

// enclosingScope is the innermost function scope holding line.
func enclosingScope(pr *parser.ParseResult, line uint32) (parser.FuncScope, bool) {
	var (
		best  parser.FuncScope
		found bool
	)

	for _, scope := range pr.Scopes {
		if int(line) < scope.Start || int(line) > scope.End {
			continue
		}

		if !found || scope.End-scope.Start < best.End-best.Start {
			best, found = scope, true
		}
	}

	return best, found
}

type loopSpan struct {
	name, collection   string // the loop variable, lowercased without local., and what it iterates
	header, start, end int
}

// enclosingLoop finds, among loops, the innermost one between lines start
// and end (end < 0 for the rest of the file) that binds name and holds line in
// its body, and returns the collection it iterates and the line of its header.
func enclosingLoop(loops []loopSpan, name string, line, start, end int) (collection string, header int, ok bool) {
	var best *loopSpan

	for i := range loops {
		s := &loops[i]
		if s.name != name || s.start > line || s.end < line || s.header < start || end >= 0 && s.header > end {
			continue
		}

		if best == nil || s.end-s.start < best.end-best.start {
			best = s
		}
	}

	if best == nil {
		return "", 0, false
	}

	return best.collection, best.header, true
}

// loopsOf is every loop in pr's file, found once per version of its content:
// the lookup runs for every untyped receiver, and finding them anew each time
// tokenised the whole file per call.
func (r *Resolver) loopsOf(pr *parser.ParseResult) []loopSpan {
	return r.loopCache.get(&r.mu, pr, func() []loopSpan {
		return append(scriptLoops(pr.Content), tagLoops(pr.Content)...)
	})
}

// fileSpans is what a per-file cache holds: the spans found in a file and the
// text they were found in.
//
// The caches were keyed by file and a hash of its text, so every edit of an
// open document added an entry and none replaced one, and each entry's spans
// were slices of the text it was found in: up to 4,096 versions of a document
// kept alive, ~2.8MB each for a 65,000-line component. Keyed by file, a new
// version replaces the last, and the text kept is the one the file's parse
// holds anyway. Comparing it costs nothing while it is the same string, where
// the hash read the whole file on every lookup.
type fileSpans[T any] struct {
	content string
	spans   []T
}

// spanCache is a per-file cache of fileSpans, guarded by the resolver's mu.
type spanCache[T any] struct {
	files map[string]fileSpans[T]
}

// get answers find for pr from the cache while pr's text is what it was last
// found in. mu guards c.
func (c *spanCache[T]) get(mu *sync.RWMutex, pr *parser.ParseResult, find func() []T) []T {
	key := string(pr.URI)

	mu.RLock()

	e, ok := c.files[key]

	mu.RUnlock()

	if ok && e.content == pr.Content {
		return e.spans
	}

	spans := find()

	mu.Lock()
	if c.files == nil || len(c.files) >= 4096 {
		c.files = make(map[string]fileSpans[T])
	}

	c.files[key] = fileSpans[T]{content: pr.Content, spans: spans}
	mu.Unlock()

	return spans
}

// scriptLoops are the `for ( [var] name in collection ) { … }` loops in content.
func scriptLoops(content string) []loopSpan {
	tokens := allTokens(content)

	var out []loopSpan

	for i := 0; i+4 < len(tokens); i++ {
		if !strings.EqualFold(tokens[i].Value, "for") || tokens[i+1].Kind != parser.TokLParen {
			continue
		}

		j := i + 2
		if strings.EqualFold(tokens[j].Value, "var") {
			j++
		}

		if j+1 >= len(tokens) || tokens[j].Kind != parser.TokIdent || !strings.EqualFold(tokens[j+1].Value, "in") {
			continue
		}

		closeParen := producerGroupEnd(tokens, i+1, parser.TokLParen, parser.TokRParen)
		if closeParen < 0 || closeParen+1 >= len(tokens) || tokens[closeParen+1].Kind != parser.TokLBrace {
			continue
		}

		closeBrace := producerGroupEnd(tokens, closeParen+1, parser.TokLBrace, parser.TokRBrace)
		if closeBrace < 0 {
			continue
		}

		out = append(out, loopSpan{
			name:       strings.TrimPrefix(strings.ToLower(tokens[j].Value), "local."),
			collection: producerText(tokens[j+2 : closeParen]),
			header:     tokens[i].Line,
			start:      tokens[closeParen+1].Line,
			end:        tokens[closeBrace].Line,
		})
	}

	return out
}

// tagLoops are the `<cfloop array="#collection#" index|item="name">` loops in
// content, each to its matching </cfloop>.
func tagLoops(content string) []loopSpan {
	type open struct {
		span  loopSpan
		match bool
	}

	var (
		out   []loopSpan
		stack []open
	)

	for _, m := range loopTagRe.FindAllStringSubmatchIndex(content, -1) {
		line := strings.Count(content[:m[0]], "\n")

		if m[3] > m[2] { // </cfloop>
			if len(stack) == 0 {
				continue
			}

			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]

			if top.match {
				top.span.end = line
				out = append(out, top.span)
			}

			continue
		}

		attrs := map[string]string{}
		for _, a := range loopAttrRe.FindAllStringSubmatch(content[m[4]:m[5]], -1) {
			attrs[strings.ToLower(a[1])] = a[2]
		}

		variable := attrs["index"]
		if variable == "" {
			variable = attrs["item"]
		}

		array := strings.TrimSpace(attrs["array"])
		match := array != "" && variable != "" &&
			strings.HasPrefix(array, "#") && strings.HasSuffix(array, "#") && strings.Count(array, "#") == 2

		stack = append(stack, open{span: loopSpan{
			name:       strings.TrimPrefix(strings.ToLower(variable), "local."),
			collection: strings.Trim(array, "#"), header: line, start: line,
		}, match: match})
	}

	return out
}

func allTokens(content string) []parser.Token {
	sc := parser.NewScanner(content)

	var tokens []parser.Token

	for {
		tok := sc.NextSkipComments()
		if tok.Kind == parser.TokEOF {
			return tokens
		}

		tokens = append(tokens, tok)
	}
}

// elementOf is the component each element of the collection expression holds,
// as written on line header of the function between start and header, or "".
func (r *Resolver) elementOf(expression string, header, start int, caller string, pr *parser.ParseResult, baseDir string, depth int, leaf string) string {
	expression = strings.TrimSpace(producerText(producerTokens(expression)))
	expression = strings.ReplaceAll(strings.ReplaceAll(expression, " . ", "."), " (", "(")

	if depth > maxElementHops || expression == "" {
		return ""
	}

	self := pr.URI.Path()

	if m := loopCallRe.FindStringSubmatch(expression); m != nil {
		receiver, method, arguments := m[1], m[2], m[3]

		target := self

		if receiver != "" && !strings.EqualFold(receiver, "this") && !strings.EqualFold(receiver, "variables") {
			comp, _ := r.receiverComponentD(receiver, conv.Uint32(header), caller, method, pr, baseDir, nil, lookupCtx{leaf: leaf})
			target = comp
		}

		return r.methodElement(target, method, arguments, baseDir)
	}

	// A view's prc.X is what the handler action rendering the view assigns,
	// and its args.X what the renders of it pass.
	if name, ok := prcMember(expression); ok {
		return r.viewPrcElement(name, pr, depth)
	}

	if scope, name, ok := strings.Cut(expression, "."); ok && strings.EqualFold(scope, "args") {
		return r.viewArgsElement(name, pr, depth)
	}

	// A field of the struct a local's call returned: results.comments.
	if f := structFieldRe.FindStringSubmatch(expression); f != nil && !isScopeWord(f[1]) {
		if rhs, ok := localAssignment(pr.Content, f[1], start, header); ok {
			return r.fieldElement(rhs, f[2], header, caller, pr, baseDir, leaf)
		}

		return ""
	}

	// The same through a key the subclass spells: results[ variables.entityPlural ],
	// where each subclass sets entityPlural to the name its service's struct uses.
	if f := keyedFieldRe.FindStringSubmatch(expression); f != nil && !isScopeWord(f[1]) && leaf != "" {
		key := r.leafLiteral(leaf, pr.URI.Path(), f[2])
		if key == "" {
			return ""
		}

		if rhs, ok := localAssignment(pr.Content, f[1], start, header); ok {
			return r.fieldElement(rhs, key, header, caller, pr, baseDir, leaf)
		}

		return ""
	}

	m := loopPathRe.FindStringSubmatch(expression)
	if m == nil {
		return ""
	}

	scope, name := strings.ToLower(m[1]), m[2]

	if scope == "" || scope == "local" {
		if rhs, ok := localAssignment(pr.Content, name, start, header); ok {
			return r.elementOf(rhs, header, start, caller, pr, baseDir, depth+1, leaf)
		}

		if scope == "local" {
			return ""
		}
	}

	if scope == "arguments" {
		return ""
	}

	// The component's own property, read through the variable it fills.
	return r.methodElement(self, "get"+name, "", baseDir)
}

// methodElement is the element type of what method returns on comp.
func (r *Resolver) methodElement(comp, method, arguments, baseDir string) string {
	if comp == "" || strings.HasPrefix(comp, "$") {
		return ""
	}

	fd := r.ResolveFunc(comp, method, baseDir)
	if fd == nil {
		return ""
	}

	if fd.ElementComponent != "" {
		return r.withSubclasses(r.ComponentPath(fd.ElementComponent, filepath.Dir(fd.URI.Path())))
	}

	// cborm: a service bound to an entity returns an array of it from getAll(),
	// and structs when asked for some of its properties; and from a finder of
	// its own built on a criteria list() (returnsCriteriaList).
	if strings.EqualFold(method, "getAll") && !strings.Contains(strings.ToLower(arguments), "properties") || r.returnsCriteriaList(fd) {
		if entity := r.boundEntity(r.ComponentPath(comp, baseDir)); entity != "" {
			return r.withSubclasses(r.ComponentPath(entity, baseDir))
		}
	}

	return ""
}

// withSubclasses is the entity at path and every component in the workspace
// that extends it, directly or not, as alternatives: an ORM collection of an
// entity holds its subclasses too. ContentBox's subscriber holds
// BaseSubscription entities and calls getRelatedContent() on the comment ones,
// a CommentSubscription method, under `case "Comment"`. A method any of them
// declares is found.
func (r *Resolver) withSubclasses(path string) string {
	if path == "" || r.Index == nil {
		return path
	}

	all := []string{path}

	for i := 0; i < len(all) && len(all) < 16; i++ {
		base := all[i]
		name := strings.TrimSuffix(filepath.Base(base), filepath.Ext(base))

		for _, u := range r.Index.FilesExtendingName(name) {
			sub := cfpath.FromURI(u)

			if slices.ContainsFunc(all, func(p string) bool { return cfpath.SamePath(p, sub) }) {
				continue
			}

			if r.descendsFrom(sub, base) {
				all = append(all, sub)
			}
		}
	}

	// The entity itself last: a missing method's reason names the last
	// alternative, and the collection is declared as the entity.
	return strings.Join(append(all[1:], all[0]), "|")
}

var localAssignRe = regexp.MustCompile(`(?im)^\s*(?:<cfset\s+)?(?:var\s+)?(?:local\.|variables\.)?(\w+)\s*=\s*([^=].*?)\s*/?>?\s*;?\s*$`)

// localAssignment is the right-hand side of the last assignment to name on a
// line from start to before header, when its parentheses balance. A statement
// that runs on over the next lines (a call with its arguments one to a line,
// ContentBox's `var results = svc.search(` …) is joined, up to the line
// before header and at most maxAssignmentLines of them.
func localAssignment(content, name string, start, header int) (string, bool) {
	rhs, _, ok := localAssignmentAt(content, name, start, header)

	return rhs, ok
}

// localAssignmentAt is localAssignment with the source line retained for
// consumers that must prove the assignment reaches a particular use.
func localAssignmentAt(content, name string, start, header int) (string, int, bool) {
	lines := strings.Split(content, "\n")

	for i := min(header, len(lines)) - 1; i >= start && i >= 0; i-- {
		m := localAssignRe.FindStringSubmatch(lines[i])
		if m == nil || !strings.EqualFold(m[1], name) {
			continue
		}

		rhs := strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(m[2]), ";"), "/"))

		for j := i + 1; strings.Count(rhs, "(") > strings.Count(rhs, ")") && j < min(header, len(lines)) && j <= i+maxAssignmentLines; j++ {
			rhs += " " + strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(lines[j]), ";"), "/"))
			rhs = strings.TrimSpace(rhs)
		}

		if strings.Count(rhs, "(") != strings.Count(rhs, ")") {
			return "", 0, false
		}

		return rhs, i, true
	}

	return "", 0, false
}

const maxAssignmentLines = 24

func isScopeWord(s string) bool {
	switch strings.ToLower(s) {
	case "local", "variables", "this", "arguments", "prc", "rc", "session", "application", "request":
		return true
	default:
		return false
	}
}
