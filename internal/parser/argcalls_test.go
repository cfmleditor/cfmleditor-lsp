package parser

import (
	"slices"
	"strings"
	"testing"
)

// A call written inside another call's argument list used to be invisible: the
// argument list was consumed a token at a time and never examined. Nothing else
// lost calls that way — a condition, a return, an assignment's RHS, a string
// concatenation, a struct or array literal and a ternary all extracted
// correctly — so an argument list was the one place a call could hide.
//
// What that cost was not completeness for its own sake. A method called only
// from inside an argument list had no edge into it, so internal/codemap read it
// as unreachable and the `unresolved` scan never checked it: a broken call
// there was reported nowhere.

// callsIn returns "receiver.method" for every call the parser found, with "?"
// for a bare call that names no receiver.
func callsIn(t *testing.T, body string) []string {
	t.Helper()

	src := "component {\n\tfunction go() {\n\t\t" + body + "\n\t}\n}"

	pr := ParseWithOptions(testURI, src, ParseOptions{ExtractCalls: true})

	out := make([]string, 0, 4)

	for _, c := range pr.AllCalls() {
		recv := c.Variable
		if recv == "" {
			recv = "?"
		}

		out = append(out, recv+"."+c.FuncName)
	}

	return out
}

func assertCalls(t *testing.T, body string, want []string) {
	t.Helper()

	got := callsIn(t, body)
	if len(got) != len(want) {
		t.Fatalf("%s\n  got  %v\n  want %v", body, got, want)
	}

	for _, w := range want {
		found := false

		for _, g := range got {
			if g == w {
				found = true

				break
			}
		}

		if !found {
			t.Errorf("%s\n  got  %v\n  want %v (missing %q)", body, got, want, w)
		}
	}
}

func TestCallsInsideAnArgumentListAreFound(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []string
	}{
		// The bare-call and the dotted-call paths consume their argument lists
		// through different code, and only one of them was fixed first — so
		// both shapes are here deliberately.
		{"bare call's argument", `writeOutput(svc.getName());`, []string{"?.writeOutput", "svc.getName"}},
		{"dotted call's argument", `log.info(svc.getName());`, []string{"log.info", "svc.getName"}},
		{"second argument", `arrayAppend(rows, dao.load(id));`, []string{"?.arrayAppend", "dao.load"}},
		{"two levels down", `outer(mid(svc.inner()));`, []string{"?.outer", "?.mid", "svc.inner"}},
		{"both arguments", `merge(a.one(), b.two());`, []string{"?.merge", "a.one", "b.two"}},

		// A closure passed as an argument needs no case of its own: its body is
		// more tokens inside the same group.
		{"closure argument", `arr.each(function(i) { dao.save(i); });`, []string{"arr.each", "dao.save"}},
		{"arrow argument", `arr.map((i) => dao.wrap(i));`, []string{"arr.map", "dao.wrap"}},

		// A bracket-indexed receiver keeps the "[]" poison marker that stops it
		// resolving as if the bracket were not there.
		{"bracket receiver", `outer(REQUEST[key].run());`, []string{"?.outer", "REQUEST[].run"}},

		// Places that already worked, so the change is shown not to have moved
		// them.
		{"sequential statements", `svc.a(); svc.b();`, []string{"svc.a", "svc.b"}},
		{"condition", `if (svc.isValid()) { other.x(); }`, []string{"svc.isValid", "other.x"}},
		{"return", `return svc.build();`, []string{"svc.build"}},
		{"assignment rhs", `var x = svc.build();`, []string{"svc.build"}},
		{"ternary", `var x = cond ? svc.a() : svc.b();`, []string{"svc.a", "svc.b"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertCalls(t, tc.body, tc.want)
		})
	}
}

// A keyword met inside an argument list is not a receiver. `function` opens a
// closure and `new` an instantiation, and both are handled by what follows
// them — dispatching either as a call would invent one.
func TestKeywordsInAnArgumentListAreNotCalls(t *testing.T) {
	assertCalls(t, `register(new models.User());`, []string{"?.register"})
	assertCalls(t, `apply(function() { return 1; });`, []string{"?.apply"})
}

// The scan recurses, and Go cannot recover from stack exhaustion — it is a
// fatal runtime error, not a panic, so the recover() guarding every parse entry
// point would not catch it. Past maxArgNesting the innermost calls are skipped,
// which is what happened everywhere before.
func TestDeeplyNestedArgumentsDoNotExhaustTheStack(t *testing.T) {
	const depth = 2000

	body := strings.Repeat("f(", depth) + "svc.x()" + strings.Repeat(")", depth) + ";"

	got := callsIn(t, body)
	if len(got) == 0 {
		t.Fatal("no calls found at all — the scan bailed rather than capping")
	}

	if len(got) > maxArgNesting+2 {
		t.Errorf("found %d calls at %d deep — the nesting cap is not holding", len(got), depth)
	}
}

// An instantiation inside an argument list is not a call on its path's
// namespace, but its constructor arguments are still an argument list.
func TestInstantiationInAnArgumentList(t *testing.T) {
	assertCalls(t, `register(new models.User());`, []string{"?.register"})
	assertCalls(t, `register(new User());`, []string{"?.register"})
	assertCalls(t, `register(new "models.User"());`, []string{"?.register"})
	assertCalls(t, `register(new models.User(dao.seed()));`, []string{"?.register", "dao.seed"})
}

// A chained hop's arguments are a third argument list, and they had a third
// copy of the paren loop scanning them. `a().b(svc.c())` lost svc.c long after
// `a(svc.c())` and `x.a().b(svc.c())` were found.
func TestCallsInsideAChainedHopsArgumentsAreFound(t *testing.T) {
	assertCalls(t, `a().b(svc.c());`, []string{"?.a", "?.b", "svc.c"})
}

// A constructor's argument list is an argument list, and the four paths that
// read a `new` expression still discarded theirs a token at a time — the loop
// `skipParenBody` replaced everywhere else, left behind in `parseNewRef`,
// `parseStandaloneNew`, `checkReturnComponent` and `scanChainedCalls` as a
// `skipBalancedParens` of their own.
//
// `new Query( datasource = getDatasource() )` is how ContentBox's migrations
// reach a datasource, and the call was recorded nowhere: 118 sites on the
// corpus under that one method name. The instantiation itself stays a
// ComponentRef rather than a CallSite — the deliberate difference
// `createObject` already has — so only the arguments change.
func TestAConstructorsArgumentListIsScannedForCalls(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []string
	}{
		{"var decl", `var c = new X( svc.f() );`, []string{"svc.f"}},
		{"named argument", `var c = new X( a = svc.f() );`, []string{"svc.f"}},
		{"bare statement", `new X( svc.f() );`, []string{"svc.f"}},
		{"return", `return new X( svc.f() );`, []string{"svc.f"}},
		{"this-scoped assignment", `this.p = new X( svc.f() );`, []string{"svc.f"}},
		{"nested", `var c = new X( f( svc.g() ) );`, []string{"?.f", "svc.g"}},

		// A call chained onto the instantiation has an argument list of its
		// own, and it went through the same discarding loop.
		{"chained hop's arguments", `var c = new X().g( svc.f() );`, []string{"?.g", "svc.f"}},

		// Already correct before this, because it reaches `new` through
		// scanNestedCall rather than through one of the four paths above. It is
		// here so a regression in either direction shows up in one test.
		{"inside another argument list", `a( new X( svc.f() ) );`, []string{"?.a", "svc.f"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertCalls(t, c.body, c.want)
		})
	}
}

// Scanning a constructor's arguments must not turn its named arguments into
// variables — the trap the `addCall` gate exists for, and the reason
// `svc.save( force = true )` once declared `force`.
func TestConstructorNamedArgumentsDoNotDeclareVariables(t *testing.T) {
	src := "component {\n\tfunction go() {\n" +
		"\t\tvar c = new X( datasource = getDS(), table = \"t\" );\n" +
		"\t}\n}"

	pr := ParseWithOptions(testURI, src, ParseOptions{ExtractCalls: true})

	got := pr.FuncVars(pr.Scopes[0].Start, pr.Scopes[0].End)
	if want := []string{"c"}; !slices.Equal(got, want) {
		t.Errorf("FuncVars: got %v want %v", got, want)
	}
}
