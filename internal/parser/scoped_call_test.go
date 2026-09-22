package parser

import (
	"slices"
	"strings"
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

// chainOf renders each call as "receiver[chain].method", so a test can assert
// what a hop was called *on* rather than only that it was recorded.
func chainOf(t *testing.T, stmt string) []string {
	t.Helper()

	pr := ParseWithOptions(testURI, "component {\n function go() {\n  "+stmt+"\n }\n}\n",
		ParseOptions{ExtractCalls: true})

	out := make([]string, 0, 4)

	for _, c := range pr.AllCalls() {
		v := c.Variable
		if v == "" {
			v = "?"
		}

		if len(c.Chain) > 0 {
			v += "[" + strings.Join(c.Chain, " ") + "]"
		}

		out = append(out, v+"."+c.FuncName)
	}

	slices.Sort(out)

	return out
}

// A hop chained onto a scope-prefixed call kept no receiver. Both scoped-var
// handlers recorded the first call and then merely skipped its argument list,
// so the `.c()` in `variables.a.b().c()` was left for the outer loop to
// rediscover as an orphaned *bare* call.
//
// That is a wrong answer rather than a missing one — in a component that
// declares a `c`, it is an edge the call never takes — and it is the exact
// thing `continueChainCalls` exists to stop on the unscoped path, which is why
// the fix routes the scoped path through the same helper rather than growing a
// second one.
//
// `make gapcheck` cannot see most of this: it compares line and method name and
// deliberately not the receiver, so a call recorded against the wrong receiver
// still counts as found. What the corpus did show is the line: 251 sites where
// the rediscovered bare call landed somewhere the grammar did not put it.
func TestAHopChainedOntoAScopedCallKeepsItsReceiver(t *testing.T) {
	for _, c := range []struct {
		stmt string
		want []string
	}{
		{`variables.a.b().c();`, []string{"variables.a.b", "variables.a[b].c"}},
		{`arguments.a.b().c();`, []string{"arguments.a.b", "arguments.a[b].c"}},
		{`local.a.b().c();`, []string{"local.a.b", "local.a[b].c"}},
		{`this.a.b().c();`, []string{"this.a.b", "this.a[b].c"}},
		{`variables.a.b().c().d();`, []string{"variables.a.b", "variables.a[b c].d", "variables.a[b].c"}},

		// The unscoped path, which already behaved this way and is what the
		// scoped one is now matched against.
		{`x.y().z();`, []string{"x.y", "x[y].z"}},
	} {
		t.Run(c.stmt, func(t *testing.T) {
			if got := chainOf(t, c.stmt); !slices.Equal(got, c.want) {
				t.Errorf("got %v want %v", got, c.want)
			}
		})
	}
}

// A hop chained onto a call made *directly* on a scope is the same wrong-answer
// class, and the receiver it deserves differs by scope for the same reason the
// first call's does. A member of this component walks the chain through its
// declared return type; a dynamic receiver stays dynamic all the way down, as a
// literal receiver's chain already does.
//
// Two corpus sites, so this is not here for the tally — it is here because
// `?.c` is an edge to a file-local `c` that the call never takes.
func TestAHopChainedOntoAScopeMemberCallKeepsItsReceiver(t *testing.T) {
	render := func(stmt string) []string {
		pr := ParseWithOptions(testURI, "component {\n function go() {\n  "+stmt+"\n }\n}\n",
			ParseOptions{ExtractCalls: true})

		out := make([]string, 0, 4)

		for _, c := range pr.AllCalls() {
			v := c.Variable
			if v == "" {
				v = "?"
			}

			if c.Component != "" {
				v += "{" + c.Component + "}"
			}

			if len(c.Chain) > 0 {
				v += "[" + strings.Join(c.Chain, " ") + "]"
			}

			out = append(out, v+"."+c.FuncName)
		}

		slices.Sort(out)

		return out
	}

	for _, c := range []struct {
		stmt string
		want []string
	}{
		{`variables.helper().c();`, []string{"?.helper", "?[helper].c"}},
		{`this.init().run();`, []string{"?.init", "?[init].run"}},
		{`variables.helper().c().d();`, []string{"?.helper", "?[helper c].d", "?[helper].c"}},
		{`request.get().c();`, []string{"request{$any}.c", "request{$any}.get"}},
		{`request.get().c().d();`, []string{"request{$any}.c", "request{$any}.d", "request{$any}.get"}},
		{`local.fn().c();`, []string{"local{$any}.c", "local{$any}.fn"}},
	} {
		t.Run(c.stmt, func(t *testing.T) {
			if got := render(c.stmt); !slices.Equal(got, c.want) {
				t.Errorf("got %v want %v", got, c.want)
			}
		})
	}
}
