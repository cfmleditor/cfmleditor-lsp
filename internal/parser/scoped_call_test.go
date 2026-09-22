package parser

import (
	"slices"
	"testing"
)

// scopedCall returns "variable[component].method" for every call, in both parse
// loops, and fails if the two disagree — a statement reads through
// `scriptParser.parse()` at the top level of a file and through
// `handleBodyToken` inside a function, and every rule here has to hold twice.
func scopedCall(t *testing.T, stmt string) []string {
	t.Helper()

	render := func(src string) []string {
		pr := ParseWithOptions(testURI, src, ParseOptions{ExtractCalls: true})

		out := make([]string, 0, 2)

		for _, c := range pr.AllCalls() {
			v := c.Variable
			if v == "" {
				v = "?"
			}

			if c.Component != "" {
				v += "[" + c.Component + "]"
			}

			out = append(out, v+"."+c.FuncName)
		}

		slices.Sort(out)

		return out
	}

	top := render("<cfscript>\n" + stmt + "\n</cfscript>")
	inFunc := render("component {\n function go() {\n  " + stmt + "\n }\n}\n")

	if !slices.Equal(top, inFunc) {
		t.Errorf("the two parse loops disagree on %q:\n  top-level %v\n  in a body %v", stmt, top, inFunc)
	}

	return top
}

// A scope followed directly by a call is a member call on the scope. Both
// scoped-var handlers read `scope` `.` `name` and then looked only for `=` (an
// assignment) or `.` (a longer chain), so the shape where the statement *is*
// the call recorded nothing at all.
//
// `x = request.getRemote()` and `request.a.getRemote()` both worked, which is
// why this survived: only the bare statement form was lost, and that is how a
// component calls its own method with an explicit scope. 109 sites on the
// corpus under `getRemoteClients` alone.
func TestACallDirectlyOnAScopeIsRecorded(t *testing.T) {
	for _, c := range []struct {
		stmt string
		want []string
	}{
		// `this.` and `variables.` name a member of the component being
		// parsed, so the call is unqualified and resolves against the file's
		// own functions — including the ones parseFunctionValue files from
		// `this.helper = function(){}`.
		{`this.init();`, []string{"?.init"}},
		{`variables.buildCache();`, []string{"?.buildCache"}},

		// Every other scope holds a value put there at runtime. Recording one
		// of these unqualified would be a *wrong* answer rather than a missing
		// one: in a file that happens to declare `getRemote`, it is an edge to
		// a function the call never reaches. `$any` records the call site and
		// skips the method-exists check, as a literal receiver already does.
		{`request.getRemote();`, []string{"request[$any].getRemote"}},
		{`session.get();`, []string{"session[$any].get"}},
		{`application.getCache();`, []string{"application[$any].getCache"}},
		{`local.fn();`, []string{"local[$any].fn"}},
		{`arguments.callback();`, []string{"arguments[$any].callback"}},

		// The argument list is scanned, like any other.
		{`variables.helper( svc.load() );`, []string{"?.helper", "svc.load"}},

		// Shapes that already worked and must not change.
		{`x = request.getRemote();`, []string{"request.getRemote"}},
		{`request.a.getRemote();`, []string{"request.a.getRemote"}},
		{`variables.svc.save();`, []string{"variables.svc.save"}},
		{`this.x = 1;`, nil},
	} {
		t.Run(c.stmt, func(t *testing.T) {
			got := scopedCall(t, c.stmt)
			if len(got) == 0 && c.want == nil {
				return
			}

			if !slices.Equal(got, c.want) {
				t.Errorf("got %v want %v", got, c.want)
			}
		})
	}
}

// The scope is read from the token, not from the Scope value the dispatch
// passes: `request`, `session` and `application` are all dispatched as
// ScopeVariables so that an assignment through one keeps the component its
// right-hand side establishes. Testing the enum instead would record every one
// of them as a call to a function of that name in this file — which is exactly
// the made-up answer the `$any` arm exists to avoid.
func TestARequestScopedCallIsNotTakenForAFileLocalFunction(t *testing.T) {
	src := "component {\n" +
		" function getRemote() { return 1; }\n" +
		" function go() {\n" +
		"  request.getRemote();\n" +
		" }\n}\n"

	pr := ParseWithOptions(testURI, src, ParseOptions{ExtractCalls: true})

	for _, c := range pr.AllCalls() {
		if c.FuncName != "getRemote" {
			continue
		}

		if c.Component != "$any" {
			t.Errorf("request.getRemote() recorded with component %q, want %q — "+
				"the file declares a getRemote and this call does not reach it",
				c.Component, "$any")
		}

		return
	}

	t.Fatal("request.getRemote() was not recorded at all")
}
