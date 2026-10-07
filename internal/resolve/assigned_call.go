package resolve

import (
	"regexp"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/conv"
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
// Only a bare or local. name, prc.name and FW/1's rc.name, and only a single call: a chain
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

	name, container, scoped := variable, "", false

	switch lower := strings.ToLower(variable); {
	case strings.HasPrefix(lower, "variables."):
		// The component's own variable, which any function may have set.
		name, scoped = variable[10:], true
	case strings.HasPrefix(lower, "prc."):
		name, container = variable[4:], "prc"
	case strings.HasPrefix(lower, "rc."):
		// FW/1's request context, as a controller fills it.
		name, container = variable[3:], "rc"
	case strings.HasPrefix(lower, "arguments.rc."):
		name, container = variable[13:], "rc"
	case strings.HasPrefix(lower, "local."):
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
		at  int // the assignment's line, -1 when not known
	)

	// Where the right-hand side is typed: the lookup's own line and function,
	// unless the assignment was made in another function.
	evalLine, evalCaller := line, caller

	if container != "" {
		lines := strings.Split(pr.Content, "\n")
		from := min(start, len(lines))

		var idx int

		rhs, idx, ok = lastMemberAssignmentAt(strings.Join(lines[from:min(int(line), len(lines))], "\n"), container, name)
		at = from + idx
	} else {
		rhs, at, ok = localAssignmentAt(pr.Content, name, start, int(line))
		if ok && scoped && localDeclRe.MatchString(lineOfContent(pr.Content, at)) {
			// `var x = …` is a local, not the variables.x asked about.
			ok = false
		}

		if !ok && start > 0 && (variable == name || scoped) {
			rhs, at, ok = variablesAssignment(pr, name, caller, start, int(line), scoped)
			if ok {
				// Typed where it was assigned: `variables.x = arguments.x` in
				// init() reads init's argument, not the caller's.
				evalLine = conv.Uint32(at)
				if scope, found := enclosingScope(pr, evalLine); found {
					evalCaller = scope.Name
				}
			}
		}
	}

	if !ok {
		return ""
	}

	// `var mmRBF = application.rbFactory`: an alias of a name, which is
	// typed as that name is at the line (Masa's form builder reads the
	// resource bundle factory a startup template assigns).
	if aliasRe.MatchString(rhs) && !strings.EqualFold(rhs, variable) && !strings.EqualFold(rhs, name) {
		comp, _ := r.receiverComponentD(rhs, evalLine, evalCaller, "", pr, baseDir, nil, lookupCtx{depth: depth + 1, leaf: ctx.leaf})
		if comp != "" && !strings.HasPrefix(comp, "$") {
			tr.addf("resolved %q to %q: it is assigned %s", variable, comp, rhs)

			return comp
		}

		return ""
	}

	recv, calls, isCall := callChain(rhs)
	if !isCall || recv == "" && isScopeWord(calls[0].name) {
		return ""
	}

	// `x = x.save()`: the call is made on what x held before the line, so
	// the receiver is read there. Without the assignment's line it cannot be.
	at32 := evalLine

	if selfAssigned(recv, variable, container, name) {
		if at < 0 {
			return ""
		}

		at32 = conv.Uint32(at)
	} else if strings.EqualFold(strings.TrimPrefix(strings.ToLower(recv), "variables."), strings.ToLower(name)) {
		return ""
	}

	answer := r.typeCallExpr(rhs, at32, evalCaller, pr, baseDir, ctx)
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
	receiver, calls, ok := callChain(rhs)
	if !ok {
		return ""
	}

	var comp string

	if receiver == "" {
		comp = r.typeBareCallExpr(calls[0].name, calls[0].args, line, pr, baseDir)
		calls = calls[1:]
	} else {
		comp, _ = r.receiverComponentD(receiver, line, caller, calls[0].name, pr, baseDir, nil, lookupCtx{depth: ctx.depth + 1, leaf: ctx.leaf})
		if comp == "" {
			// A receiver only a configured resolver names (a preset's
			// application.wo), as ComponentOf reads it.
			comp, _, _ = parser.ResolveFromCallMatch(receiver, r.Resolvers)
		}
	}

	lookup := r.FuncLookup(baseDir)

	// Each call in a chain is made on what the one before it returns.
	for _, c := range calls {
		if comp == "" || strings.HasPrefix(comp, "$") {
			return ""
		}

		comp = hopReturns(lookup, comp, c)
	}

	if strings.HasPrefix(comp, "$") {
		return ""
	}

	return comp
}

// hopReturns is what method c returns on each alternative of comp, or "" when
// any alternative returns no component.
func hopReturns(lookup func(string, string) string, comp string, c chainCall) string {
	var returns []string

	for alt := range strings.SplitSeq(comp, "|") {
		ret := lookup(alt, c.name)
		if ret == "" {
			// What some methods return depends on what they are handed
			// (Wheels' controller( "name" )); the parse asks the same way.
			ret = lookup(alt, parser.CallHop(c.name+"("+c.args+")"))
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

	// A declared return answers when the parse could not see the function:
	// a module's helper stub (cbvalidation's validate()) is found only here.
	if ret := r.ReturnComponentOf(def); ret != "" {
		if strings.HasPrefix(ret, "$") {
			return ""
		}

		return ret
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

// localDeclRe is a line declaring a local: `var x`, `local.x`.
var localDeclRe = regexp.MustCompile(`(?i)(?:^|[^\w.])(?:var\s+[\w$]|local\.)`)

// aliasRe is a right-hand side that is a dotted name and nothing else.
var aliasRe = regexp.MustCompile(`^[A-Za-z_$][\w$]*(?:\.[A-Za-z_$][\w$]*)+$`)

// chainCall is one call of a chain: its name and its argument text.
type chainCall struct{ name, args string }

// callChain splits rhs into the dotted name it starts from and the calls made
// on it, each on what the one before returns: `svc.get( id )` is svc and get,
// `getBean( "x" ).loadBy( id = 1 ).set( rc )` no receiver and three calls. It
// fails on anything else — a property read between calls, an operator, a
// bracket — since that is not a chain of calls. Names may hold `$`, as CFML's
// do: Wheels' internal methods are $-prefixed.
func callChain(rhs string) (receiver string, calls []chainCall, ok bool) {
	s := strings.TrimSpace(rhs)

	var segs []string

	for {
		n := 0
		for n < len(s) && (s[n] == '_' || s[n] == '$' || s[n] >= '0' && s[n] <= '9' || s[n] >= 'a' && s[n] <= 'z' || s[n] >= 'A' && s[n] <= 'Z') {
			n++
		}

		if n == 0 || s[0] >= '0' && s[0] <= '9' {
			return "", nil, false
		}

		word := s[:n]
		s = strings.TrimLeft(s[n:], " \t")

		switch {
		case strings.HasPrefix(s, "("):
			end := closingParen(s)
			if end < 0 {
				return "", nil, false
			}

			// Only the first call has a receiver: after it a bare word is a
			// property read, which the default case below refuses.
			if len(calls) == 0 {
				receiver = strings.Join(segs, ".")
			}

			calls = append(calls, chainCall{word, s[1:end]})
			s = strings.TrimLeft(s[end+1:], " \t")

			if s == "" {
				return receiver, calls, true
			}
		case len(calls) == 0:
			segs = append(segs, word)
		default:
			return "", nil, false
		}

		if !strings.HasPrefix(s, ".") {
			return "", nil, false
		}

		s = strings.TrimLeft(s[1:], " \t")
	}
}

// closingParen is the index of the ) closing the ( at s[0], stepping over
// quoted strings, or -1.
func closingParen(s string) int {
	depth := 0

	var quote byte

	for i := range len(s) {
		c := s[i]

		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '(':
			depth++
		case c == ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}

	return -1
}

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
		default:
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
// preceding assignment in the file). A variables.-scoped receiver reads the
// same assignment. Not when the function declares name as
// a local or an argument, which hides the variable, nor when the assignment
// found is a `var` or local. one, which never left its function.
func variablesAssignment(pr *parser.ParseResult, name, caller string, start, line int, scoped bool) (string, int, bool) {
	lines := strings.Split(pr.Content, "\n")
	end := min(line, len(lines))

	// A local or an argument of the same name hides an unscoped one; a
	// variables.-scoped receiver is the component's whatever the function
	// declares.
	declared := regexp.MustCompile(`(?i)(?:\bvar\s+|\blocal\.|<cfargument\s[^>]*name\s*=\s*["'])` + regexp.QuoteMeta(name) + `\b`)
	if !scoped && (argumentOf(pr, caller, name) != nil || declared.MatchString(strings.Join(lines[min(start, end):end], "\n"))) {
		return "", 0, false
	}

	rhs, at, ok := localAssignmentAt(pr.Content, name, 0, start)
	if !ok || declared.MatchString(lines[at]) {
		return "", 0, false
	}

	return rhs, at, true
}

// selfAssigned reports whether receiver, the receiver of the call assigned to
// variable, is variable itself, however its arguments. prefix is spelled.
func selfAssigned(receiver, variable, container, name string) bool {
	if strings.EqualFold(receiver, variable) {
		return true
	}

	return container != "" && strings.EqualFold(strings.TrimPrefix(strings.ToLower(receiver), "arguments."), container+"."+strings.ToLower(name))
}
