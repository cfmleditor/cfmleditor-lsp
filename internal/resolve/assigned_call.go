package resolve

import (
	"regexp"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
)

// The parse types `x = svc.get( id )` from the refs its own file holds. When
// svc is a variable the file never types — an abstract handler's
// variables.ormService, which only its subclasses inject — the parse leaves x
// untyped, and every call on x and on prc.x in the view the action renders was
// "has no component ref".
//
// assignedFromCall reads the answer at lookup time instead: the last
// `x = receiver.method( … )` before the call, on one line, with the receiver
// typed as any receiver is (subclassComponent among the steps) and the
// method's return taken per alternative, each alternative of a receiver
// standing for the components its subclasses hold. Every alternative must
// return a component, or there is no answer.
//
// Only a bare or local. name and prc.name, and only a single call: a chain
// (`svc.get().x()`) or an expression is the parse's. It runs after the other
// steps and so never overrides one.

const maxAssignedDepth = 3

// lookupCtx is what a receiver lookup carries beyond the variable: how many
// assignments deep it is (assignedFromCall), and, when the view handoff is
// reading a handler action on behalf of one subclass, that leaf: the base's
// own receivers are then what that subclass holds, not what every one does.
type lookupCtx struct {
	depth int
	leaf  string
}

func (r *Resolver) assignedFromCall(variable string, line uint32, caller string, pr *parser.ParseResult, baseDir string, tr *callTrace, ctx lookupCtx) string {
	depth := ctx.depth
	if pr == nil || depth >= maxAssignedDepth || variable == "" || strings.ContainsAny(variable, "[(") {
		return ""
	}

	name, prc := variable, false

	switch {
	case strings.HasPrefix(strings.ToLower(variable), "prc."):
		name, prc = variable[4:], true
	case strings.HasPrefix(strings.ToLower(variable), "local."):
		name = variable[6:]
	}

	if name == "" || strings.Contains(name, ".") {
		return ""
	}

	start := 0
	if scope, ok := enclosingScope(pr, line); ok {
		start = scope.Start
	}

	var (
		rhs string
		ok  bool
	)

	if prc {
		lines := strings.Split(pr.Content, "\n")
		rhs, ok = lastPrcAssignment(strings.Join(lines[min(start, len(lines)):min(int(line), len(lines))], "\n"), name)
	} else {
		rhs, ok = localAssignment(pr.Content, name, start, int(line))
		if !ok && start > 0 && variable == name {
			rhs, ok = variablesAssignment(pr, name, caller, start, int(line))
		}
	}

	if !ok {
		return ""
	}

	if m := assignedCallRe.FindStringSubmatch(rhs); m == nil || m[1] == "" && isScopeWord(m[2]) || strings.EqualFold(m[1], variable) ||
		strings.EqualFold(strings.TrimPrefix(strings.ToLower(m[1]), "variables."), strings.ToLower(name)) {
		return ""
	}

	answer := r.typeCallExpr(rhs, line, caller, pr, baseDir, ctx)
	if answer != "" {
		tr.addf("resolved %q to %q: the last assignment to it is %s", variable, answer, rhs)
	}

	return answer
}

// typeCallExpr is the component(s) the single call rhs (`receiver.method( … )`)
// returns, written in pr at line, with the receiver typed as any receiver is
// and the method's return taken per alternative: every one must return a
// component, or there is none.
func (r *Resolver) typeCallExpr(rhs string, line uint32, caller string, pr *parser.ParseResult, baseDir string, ctx lookupCtx) string {
	m := assignedCallRe.FindStringSubmatch(rhs)
	if m == nil {
		return ""
	}

	receiver, method := m[1], m[2]
	if receiver == "" {
		return r.typeBareCallExpr(method, m[3], line, pr, baseDir)
	}

	comp, _ := r.receiverComponentD(receiver, line, caller, method, pr, baseDir, nil, lookupCtx{depth: ctx.depth + 1, leaf: ctx.leaf})
	if comp == "" {
		// A receiver only a configured resolver names (a preset's
		// application.wo), as ComponentOf reads it.
		comp, _, _ = parser.ResolveFromCallMatch(receiver, r.Resolvers)
	}

	if comp == "" || strings.HasPrefix(comp, "$") {
		return ""
	}

	var returns []string

	lookup := r.FuncLookup(baseDir)

	for alt := range strings.SplitSeq(comp, "|") {
		ret := lookup(alt, method)
		if ret == "" {
			// What some methods return depends on what they are handed
			// (Wheels' controller( "name" )); the parse asks the same way.
			ret = lookup(alt, parser.CallHop(method+"("+m[3]+")"))
		}

		if ret == "" || strings.HasPrefix(ret, "$") {
			return ""
		}

		if !containsFold(returns, ret) {
			returns = append(returns, ret)
		}
	}

	return strings.Join(returns, "|")
}

// typeBareCallExpr is what the unqualified call method( args ) returns, when
// only its arguments decide it (expressionReturn): a spec's
// `pluginObj = $pluginObj( config )`, whose wrapper hands config to Wheels'
// $createObjectFromRoot. A struct literal held in an argument's local is
// written into the call in its place (inlineStructArg).
func (r *Resolver) typeBareCallExpr(method, args string, line uint32, pr *parser.ParseResult, baseDir string) string {
	def := r.bareFunc(method, pr, baseDir)
	if def == nil {
		return ""
	}

	if ret := r.ReturnComponentOf(def); ret != "" {
		return ""
	}

	if name := strings.TrimSpace(args); identRe.MatchString(name) {
		start := 0
		if scope, ok := enclosingScope(pr, line); ok {
			start = scope.Start
		}

		if lit := inlineStructArg(pr.Content, name, start, int(line)); lit != "" {
			args = lit
		}
	}

	ret := r.expressionReturn(def, method+"("+args+")", baseDir, pr.URI.Path())
	if strings.HasPrefix(ret, "$") {
		return ""
	}

	return ret
}

var identRe = regexp.MustCompile(`^[A-Za-z_]\w*$`)

// assignedCallRe is loopCallRe allowing `$` in names, as CFML does: Wheels
// spells its internal methods `$pluginObj()` and `$createObjectFromRoot()`.
var assignedCallRe = regexp.MustCompile(`^(?:([\w.$]+)\.)?([\w$]+)\s*\((.*)\)$`)

// inlineStructArg is the literal struct name holds at line, its fields that
// are string literals, as `{a="x",b="y"}`: the last `name = { … }` between
// start and line, with every `name.key = …` after it applied (a field set to
// anything but a string literal is dropped). "" when there is none.
func inlineStructArg(content, name string, start, line int) string {
	lines := strings.Split(content, "\n")
	if start >= len(lines) {
		return ""
	}

	tokens := significantTokens(strings.Join(lines[start:min(line, len(lines))], "\n"))

	open := -1

	for i := 0; i+2 < len(tokens); i++ {
		if tokens[i].Kind == parser.TokIdent && strings.EqualFold(tokens[i].Value, name) &&
			(i == 0 || tokens[i-1].Kind != parser.TokDot) &&
			tokens[i+1].Kind == parser.TokEquals && tokens[i+2].Kind == parser.TokLBrace {
			open = i + 2
		}
	}

	if open < 0 {
		return ""
	}

	end := matchingBrace(tokens, open)
	if end < 0 {
		return ""
	}

	type field struct{ key, value string }

	var fields []field

	set := func(key, value string) {
		for i := range fields {
			if strings.EqualFold(fields[i].key, key) {
				fields = append(fields[:i], fields[i+1:]...)

				break
			}
		}

		if value != "" {
			fields = append(fields, field{key, value})
		}
	}

	depth := 0

	for i := open + 1; i < end; i++ {
		switch tokens[i].Kind {
		case parser.TokLBrace, parser.TokLParen, parser.TokLBracket:
			depth++
		case parser.TokRBrace, parser.TokRParen, parser.TokRBracket:
			depth--
		case parser.TokIdent:
			if depth == 0 && i+2 < end && (tokens[i+1].Kind == parser.TokEquals || tokens[i+1].Kind == parser.TokColon) {
				value := ""
				if tokens[i+2].Kind == parser.TokString && (i+3 == end || tokens[i+3].Kind == parser.TokComma) {
					value = tokens[i+2].Value
				}

				set(tokens[i].Value, value)
			}
		}
	}

	for i := end + 1; i+4 < len(tokens); i++ {
		if tokens[i].Kind == parser.TokIdent && strings.EqualFold(tokens[i].Value, name) &&
			tokens[i+1].Kind == parser.TokDot && tokens[i+2].Kind == parser.TokIdent && tokens[i+3].Kind == parser.TokEquals {
			value := ""
			if tokens[i+4].Kind == parser.TokString && (i+5 >= len(tokens) || tokens[i+5].Kind != parser.TokDot && tokens[i+5].Kind != parser.TokAmpersand) {
				value = tokens[i+4].Value
			}

			set(tokens[i+2].Value, value)
		}
	}

	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		parts = append(parts, f.key+"="+f.value)
	}

	return "{" + strings.Join(parts, ",") + "}"
}

func containsFold(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(v, s) {
			return true
		}
	}

	return false
}

// variablesAssignment is the last assignment to an unscoped name above the
// function the lookup is in, which CFML makes a variables-scope write:
// TestBox's `_controller = …` in beforeAll(), read by every spec in run().
// It is the parse's own rule for such a name (fileLevelRef: the nearest
// preceding assignment in the file). Not when the function declares name as
// a local or an argument, which hides the variable, nor when the assignment
// found is a `var` or local. one, which never left its function.
func variablesAssignment(pr *parser.ParseResult, name, caller string, start, line int) (string, bool) {
	lines := strings.Split(pr.Content, "\n")
	end := min(line, len(lines))

	declared := regexp.MustCompile(`(?i)(?:\bvar\s+|\blocal\.|<cfargument\s[^>]*name\s*=\s*["'])` + regexp.QuoteMeta(name) + `\b`)
	if argumentOf(pr, caller, name) != nil || declared.MatchString(strings.Join(lines[min(start, end):end], "\n")) {
		return "", false
	}

	rhs, at, ok := localAssignmentAt(pr.Content, name, 0, start)
	if !ok || declared.MatchString(lines[at]) {
		return "", false
	}

	return rhs, true
}
