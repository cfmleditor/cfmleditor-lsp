package parser

import "strings"

// A chained assignment, `a = b = rhs`, gives both names what rhs holds. Each
// parser handles the inner assignment as the statement it is, then hands the
// refs and pending calls it made to the outer name through these, so the
// outer name is filed by its own scope's rules: `var a = b = new X()` makes a
// local a and a variables-scope b.

// refsMadeFor is the refs made since the marks in global and local, renamed
// for name, one per component and line: an assignment inside a function may
// file one ref in both lists, and the caller refiles each under name's own
// rule.
func refsMadeFor(global []ComponentRef, fromGlobal int, local []ComponentRef, fromLocal int, name string) []ComponentRef {
	var out []ComponentRef

	add := func(refs []ComponentRef) {
		for i := range refs {
			r := refs[i]
			r.Variable = name

			dup := false

			for j := range out {
				if out[j].Component == r.Component && out[j].Line == r.Line && out[j].VisibleFrom == r.VisibleFrom {
					dup = true

					break
				}
			}

			if !dup {
				out = append(out, r)
			}
		}
	}

	add(global[fromGlobal:])

	if fromLocal <= len(local) {
		add(local[fromLocal:])
	}

	return out
}

// appendPendingFor appends a copy of each pending call made since from,
// made for name.
func appendPendingFor(calls []pendingCall, from int, name string, this bool) []pendingCall {
	for i, n := from, len(calls); i < n; i++ {
		c := calls[i]
		c.varName, c.refThis = name, this
		calls = append(calls, c)
	}

	return calls
}

// chainedSetTarget reports whether rhs, the right-hand side of a <cfset>, is
// itself an assignment: an optional this./variables./local. and a name,
// then a single `=`. `a == b` is a comparison, and `a.b.c = x` a write into a
// struct, which types nothing.
func chainedSetTarget(rhs string) bool {
	for _, scope := range []string{"this.", "variables.", "local."} {
		if hasPrefixFold(rhs, scope) {
			rhs = rhs[len(scope):]

			break
		}
	}

	name := extractIdent(rhs)
	if name == "" || isKeyword(name) {
		return false
	}

	rest := strings.TrimLeft(rhs[len(name):], " \t\r\n")

	return rest != "" && rest[0] == '=' && (len(rest) == 1 || rest[1] != '=')
}
