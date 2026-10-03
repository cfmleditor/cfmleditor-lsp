package resolve

import (
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
	}

	if !ok {
		return ""
	}

	m := loopCallRe.FindStringSubmatch(rhs)
	if m == nil {
		return ""
	}

	receiver, method := m[1], m[2]
	if receiver == "" || strings.EqualFold(receiver, variable) || strings.EqualFold(strings.TrimPrefix(strings.ToLower(receiver), "variables."), strings.ToLower(name)) {
		return ""
	}

	comp, _ := r.receiverComponentD(receiver, line, caller, method, pr, baseDir, nil, lookupCtx{depth: depth + 1, leaf: ctx.leaf})
	if comp == "" || strings.HasPrefix(comp, "$") {
		return ""
	}

	var returns []string

	lookup := r.FuncLookup(baseDir)

	for alt := range strings.SplitSeq(comp, "|") {
		ret := lookup(alt, method)
		if ret == "" || strings.HasPrefix(ret, "$") {
			return ""
		}

		if !containsFold(returns, ret) {
			returns = append(returns, ret)
		}
	}

	answer := strings.Join(returns, "|")

	tr.addf("resolved %q to %q: what %s.%s() returns, the last assignment to it", variable, answer, receiver, method)

	return answer
}

func containsFold(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(v, s) {
			return true
		}
	}

	return false
}
