package resolve

import (
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/conv"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
)

// An untyped argument holds what its callers pass. ContentBox's
// `createSite( required setup )` is called as `createSite( arguments.setup )`
// from a method whose own argument is a `SetupData`; `populateModule( module,
// config )` from code that holds a `Module`. Nothing in the function says so,
// and every `arguments.setup.x()` in it was "has no component ref".
//
// argumentFromCallers reads the answer from the workspace: every call of the
// function that is the function's own call (a bare call in its file or a
// subclass, or a call on a receiver that resolves to it), the argument's
// expression read from the tokens (so a call split over lines reads), and
// that expression typed as a receiver is — `new X()`, a variable or a dotted
// name, a single call. The argument has a type only when **every** caller that
// passes it gets one, and they are the alternatives; a caller whose receiver
// cannot be placed, or whose expression cannot be typed, leaves it untyped
// rather than guessed from the others. A caller that omits the argument says
// nothing about it. Callers the workspace does not hold (a test elsewhere,
// a dynamic invoke) are unseen: this is an inference from the code present.
//
// It runs only where the resolver was given the files to search
// (Resolver.InferArgsFiles, set by a batch scan once its index is complete);
// the editor's per-keystroke paths never pay for the caller index.

const (
	maxArgCallerFiles = 60
	maxArgCallSites   = 40
)

// callerIndex is, for each name followed by an open parenthesis anywhere in
// the files, the files it appears in.
type callerIndex struct {
	byName map[string][]string
}

// IndexCallerFile adds the call-shaped names in content to the batch caller
// index while the unresolved scan already has the file bytes in hand. It
// avoids reading every workspace file again when the first untyped argument
// asks for its callers.
func (r *Resolver) IndexCallerFile(path, content string) {
	if len(r.InferArgsFiles) == 0 {
		return
	}

	seen := callerNames(content)

	r.mu.Lock()
	if r.callerIdx == nil {
		r.callerIdx = &callerIndex{byName: map[string][]string{}}
	}

	for name := range seen {
		r.callerIdx.byName[name] = append(r.callerIdx.byName[name], path)
	}
	r.mu.Unlock()
}

func callerNames(content string) map[string]bool {
	seen := map[string]bool{}

	for open := strings.IndexByte(content, '('); open >= 0; {
		end := open
		for end > 0 && callerSpace(content[end-1]) {
			end--
		}

		start := end
		for start > 0 && callerWord(content[start-1]) {
			start--
		}

		if start < end {
			addCallerName(seen, content[start:end])
		}

		next := open + 1

		rel := strings.IndexByte(content[next:], '(')
		if rel < 0 {
			break
		}

		open = next + rel
	}

	return seen
}

func addCallerName(seen map[string]bool, name string) {
	const stackName = 128

	var folded [stackName]byte

	if len(name) > len(folded) {
		normalized := strings.ToLower(name)
		if !seen[normalized] {
			if normalized == name {
				normalized = strings.Clone(normalized)
			}

			seen[normalized] = true
		}

		return
	}

	changed := false

	for i := range len(name) {
		c := name[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
			changed = true
		}

		folded[i] = c
	}

	if !changed {
		if !seen[name] {
			seen[strings.Clone(name)] = true
		}

		return
	}

	if seen[string(folded[:len(name)])] {
		return
	}

	seen[string(folded[:len(name)])] = true
}

func callerSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f'
}

func callerWord(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_'
}

// owner is the resolver that holds the shared caches: a per-caller view
// delegates to the one it was made from.
func (r *Resolver) owner() *Resolver {
	if r.indexer != nil {
		return r.indexer
	}

	return r
}

func (r *Resolver) callerFiles(name string) []string {
	o := r.owner()

	o.mu.RLock()
	idx := o.callerIdx
	o.mu.RUnlock()

	if idx == nil {
		idx = o.buildCallerIndex()
	}

	key := strings.ToLower(name)

	o.mu.Lock()
	files := idx.byName[key]
	slices.Sort(files)
	files = slices.Compact(files)
	idx.byName[key] = files
	files = slices.Clone(files)
	o.mu.Unlock()

	return files
}

func (r *Resolver) buildCallerIndex() *callerIndex {
	idx := &callerIndex{byName: map[string][]string{}}

	for _, path := range r.InferArgsFiles {
		data, err := r.fs().ReadFile(path)
		if err != nil {
			continue
		}

		for name := range callerNames(string(data)) {
			idx.byName[name] = append(idx.byName[name], path)
		}
	}

	r.mu.Lock()
	if r.callerIdx == nil {
		r.callerIdx = idx
	}

	idx = r.callerIdx
	r.mu.Unlock()

	return idx
}

// argumentFromCallers is the component(s) the untyped argument variable names
// holds, from its callers, or "".
func (r *Resolver) argumentFromCallers(variable, caller string, pr *parser.ParseResult, ctx lookupCtx) string {
	o := r.owner()
	if len(o.InferArgsFiles) == 0 || pr == nil || caller == "" || ctx.depth >= maxAssignedDepth {
		return ""
	}

	name := variable
	if rest, ok := strings.CutPrefix(strings.ToLower(variable), "arguments."); ok {
		name = variable[len(variable)-len(rest):]
	}

	if name == "" || strings.ContainsAny(name, ".[(") {
		return ""
	}

	fi := slices.IndexFunc(pr.Funcs, func(f parser.FunctionDef) bool { return strings.EqualFold(f.Name, caller) })
	if fi < 0 {
		return ""
	}

	fd := &pr.Funcs[fi]

	pos := slices.IndexFunc(fd.Arguments, func(a parser.Argument) bool { return strings.EqualFold(a.Name, name) })
	if pos < 0 {
		return ""
	}

	switch strings.ToLower(fd.Arguments[pos].Type) {
	case "", "any", "component", "object":
	default:
		return ""
	}

	key := pr.URI.Path() + "\x00" + strconv.Itoa(int(fd.Line)) + "\x00" + strings.ToLower(name) + "\x00" + strconv.Itoa(ctx.depth)

	o.mu.RLock()
	cached, ok := o.argCache[key]
	o.mu.RUnlock()

	if ok {
		return cached
	}

	answer := r.inferArgument(fd, pos, pr.URI.Path(), ctx)

	o.mu.Lock()
	if o.argCache == nil || len(o.argCache) >= 8192 {
		o.argCache = map[string]string{}
	}

	o.argCache[key] = answer
	o.mu.Unlock()

	return answer
}

func (r *Resolver) inferArgument(fd *parser.FunctionDef, pos int, file string, ctx lookupCtx) string {
	files := r.callerFiles(fd.Name)
	if len(files) == 0 || len(files) > maxArgCallerFiles {
		return ""
	}

	argName := fd.Arguments[pos].Name

	var (
		comps []string
		sites int
	)

	for _, path := range files {
		hpr := r.handlerParse(path)
		if hpr == nil {
			return ""
		}

		var tokens []parser.Token

		calls := hpr.AllCalls()

		for ci := range calls {
			call := &calls[ci]
			if !strings.EqualFold(call.FuncName, fd.Name) {
				continue
			}

			dir := filepath.Dir(path)

			if ours, known := r.callIsTo(call, hpr, path, fd, file, dir, ctx); !ours {
				if known {
					continue
				}

				return ""
			}

			if tokens == nil {
				tokens = allTokens(hpr.Content)
			}

			expr, passed := callArgument(tokens, call.FuncName, int(call.Line), pos, argName)
			if !passed {
				continue
			}

			sites++

			if sites > maxArgCallSites {
				return ""
			}

			comp := r.argumentExprComponent(expr, call.Line, call.Caller, hpr, dir, ctx)
			if comp == "" {
				return ""
			}

			for alt := range strings.SplitSeq(r.pathsOf(comp, dir), "|") {
				if !containsFold(comps, alt) {
					comps = append(comps, alt)
				}
			}
		}
	}

	if sites == 0 {
		return ""
	}

	slices.Sort(comps)

	return strings.Join(comps, "|")
}

// callIsTo reports whether call, made in the file at path, is a call of fd:
// a bare or this./variables. call in fd's file or a component extending it, or
// a call on a receiver that resolves to a component whose function of that
// name is fd. known is false when the receiver cannot be placed at all, so
// the call may or may not be fd's.
func (r *Resolver) callIsTo(call *parser.CallSite, hpr *parser.ParseResult, path string, fd *parser.FunctionDef, file, dir string, ctx lookupCtx) (ours, known bool) {
	if call.Variable == "" || strings.EqualFold(call.Variable, "this") || strings.EqualFold(call.Variable, "variables") || call.This {
		return cfpath.SamePath(path, file) || r.descendsFrom(path, file), true
	}

	// super.f() is a call of an ancestor's f: fd's, when fd is in one.
	if strings.EqualFold(call.Variable, "super") {
		return r.descendsFrom(path, file), true
	}

	comp, _ := r.receiverComponentD(call.Variable, call.Line, call.Caller, call.FuncName, hpr, dir, nil, lookupCtx{depth: ctx.depth + 1})
	if comp == "" || strings.HasPrefix(comp, "$") {
		return false, false
	}

	for alt := range strings.SplitSeq(comp, "|") {
		if other := r.ResolveFunc(alt, call.FuncName, dir); other != nil && other.URI.Path() == fd.URI.Path() && other.Line == fd.Line {
			return true, true
		}
	}

	return false, true
}

// callArgument is the tokens of the argument a call of name on line passes at
// position pos, or by name argName, read from the file's tokens so a call split
// over lines reads whole. passed is false when the call does not pass it.
func callArgument(tokens []parser.Token, name string, line, pos int, argName string) (expr []parser.Token, passed bool) {
	for i := 0; i+1 < len(tokens); i++ {
		if tokens[i].Line != line || tokens[i].Kind != parser.TokIdent || !strings.EqualFold(tokens[i].Value, name) || tokens[i+1].Kind != parser.TokLParen {
			continue
		}

		end := producerGroupEnd(tokens, i+1, parser.TokLParen, parser.TokRParen)
		if end < 0 {
			return nil, false
		}

		positional := 0

		for _, piece := range producerSplit(tokens[i+2 : end]) {
			if len(piece) > 2 && piece[0].Kind == parser.TokIdent && (piece[1].Kind == parser.TokEquals || piece[1].Kind == parser.TokColon) {
				if strings.EqualFold(piece[0].Value, argName) {
					return piece[2:], true
				}

				continue
			}

			if positional == pos {
				return piece, len(piece) > 0
			}

			positional++
		}

		return nil, false
	}

	return nil, false
}

// argumentExprComponent types the expression a caller passes: `new X( … )`, a
// variable or dotted name, or one call. "" for anything else.
func (r *Resolver) argumentExprComponent(expr []parser.Token, line uint32, caller string, pr *parser.ParseResult, dir string, ctx lookupCtx) string {
	expr = producerUnwrap(expr)
	if len(expr) == 0 {
		return ""
	}

	next := lookupCtx{depth: ctx.depth + 1}

	if path := producerPath(expr); path != "" {
		name := strings.ReplaceAll(producerText(expr), " ", "")

		comp, _ := r.receiverComponentD(name, line, caller, "", pr, dir, nil, next)
		if comp == "" {
			// A configured resolver types a name too (prc.oCurrentSite).
			comp, _, _ = parser.ResolveFromCallMatch(name, r.Resolvers)
		}

		if comp == "" {
			// An argument the caller passes on is typed by its own callers.
			comp = r.argumentFromCallers(name, caller, pr, next)
		}

		if comp == "" {
			comp = r.argumentAliasComponent(name, line, caller, pr, dir, next)
		}

		if comp == "" || strings.HasPrefix(comp, "$") {
			return ""
		}

		return comp
	}

	if strings.EqualFold(expr[0].Value, "new") {
		end := slices.IndexFunc(expr, func(t parser.Token) bool { return t.Kind == parser.TokLParen })
		if end < 2 {
			return ""
		}

		dotted := strings.ReplaceAll(producerText(expr[1:end]), " ", "")
		if p := r.ComponentPath(dotted, dir); p != "" {
			return p
		}

		return dotted
	}

	text := strings.ReplaceAll(strings.ReplaceAll(producerText(expr), " . ", "."), " (", "(")

	return r.typeCallExpr(text, line, caller, pr, dir, next)
}

// argumentAliasComponent follows a plain local passed by a caller to the last
// straight-line assignment before that call. Requiring equal lexical brace
// depth keeps assignments in a conditional or nested closure from becoming a
// reaching definition; an assignment after the call is excluded by the
// bounded lookup itself.
func (r *Resolver) argumentAliasComponent(name string, line uint32, caller string, pr *parser.ParseResult, dir string, ctx lookupCtx) string {
	if pr == nil || ctx.depth >= maxAssignedDepth {
		return ""
	}

	lower := strings.ToLower(name)

	lower = strings.TrimPrefix(lower, "local.")
	if lower == "" || strings.Contains(lower, ".") || isScopeWord(lower) {
		return ""
	}

	start := 0
	if scope, ok := enclosingScope(pr, line); ok {
		start = scope.Start
	}

	rhs, assignedLine, ok := localAssignmentAt(pr.Content, lower, start, int(line))
	if !ok || strings.EqualFold(strings.TrimSpace(rhs), name) || producerBraceDepth(pr.Content, assignedLine) != producerBraceDepth(pr.Content, int(line)) {
		return ""
	}

	return r.argumentExprComponent(producerTokens(rhs), conv.Uint32(assignedLine), caller, pr, dir, ctx)
}

func producerBraceDepth(content string, line int) int {
	depth := 0

	for _, token := range producerTokens(content) {
		if token.Line >= line {
			break
		}

		switch token.Kind {
		case parser.TokLBrace:
			depth++
		case parser.TokRBrace:
			depth--
		default:
		}
	}

	return depth
}
