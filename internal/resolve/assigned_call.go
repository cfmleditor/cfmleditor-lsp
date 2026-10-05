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

	if m := loopCallRe.FindStringSubmatch(rhs); m == nil || m[1] == "" || strings.EqualFold(m[1], variable) ||
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
	m := loopCallRe.FindStringSubmatch(rhs)
	if m == nil {
		return ""
	}

	receiver, method := m[1], m[2]
	if receiver == "" {
		return ""
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
