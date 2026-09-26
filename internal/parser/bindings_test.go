package parser

import (
	"slices"
	"testing"
)

// A component may expose a method it builds rather than declares. The parser
// recorded only a variable, so Funcs held none of them — no completion, no
// signature help, no go-to-definition and nothing in the index.
func TestFunctionValuedAssignmentsDeclareAMethod(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
		args int
	}{
		{"this scope", `this.helper = function(required string a) { return a; };`, "helper", 1},
		{"variables scope", `variables.helper = function(a, b) { return a; };`, "helper", 2},
		{"unscoped", `helper = function(a) { return a; };`, "helper", 1},
		{"arrow", `this.helper = (a) => a;`, "helper", 1},
		{"no arguments", `this.helper = function() { return 1; };`, "helper", 0},
		{"named function expression", `this.helper = function helper(a) { return a; };`, "helper", 1},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pr := Parse(testURI, "component {\n"+c.body+"\n}\n")

			if len(pr.Funcs) != 1 {
				t.Fatalf("expected one function, got %+v", pr.Funcs)
			}

			if pr.Funcs[0].Name != c.want {
				t.Errorf("name: got %q want %q", pr.Funcs[0].Name, c.want)
			}

			if len(pr.Funcs[0].Arguments) != c.args {
				t.Errorf("arguments: got %d want %d (%+v)",
					len(pr.Funcs[0].Arguments), c.args, pr.Funcs[0].Arguments)
			}

			if len(pr.Scopes) != 1 {
				t.Errorf("expected one scope, got %+v", pr.Scopes)
			}
		})
	}
}

// A local closure is a value, not a method. Declaring one would put a helper
// private to a single function into every caller's completion list.
func TestLocalClosuresAreNotMethods(t *testing.T) {
	cases := []string{
		"component {\nvar helper = function(a) { return a; };\n}\n",
		"component {\nfunction go() {\n var helper = function(a) { return a; };\n}\n}\n",
		"component {\nfunction go() {\n local.helper = function(a) { return a; };\n}\n}\n",
	}

	for _, src := range cases {
		pr := Parse(testURI, src)
		for _, f := range pr.Funcs {
			if f.Name == "helper" {
				t.Errorf("%s\n  declared a method for a local closure", src)
			}
		}
	}
}

// An expression that merely starts with a paren is not an arrow function.
func TestParenthesisedExpressionsAreNotArrowFunctions(t *testing.T) {
	pr := Parse(testURI, "component {\nthis.total = (a + b) * c;\n}\n")
	if len(pr.Funcs) != 0 {
		t.Errorf("expected no functions, got %+v", pr.Funcs)
	}
}

// `for (var row in qry)` binds through `in`, and the var-decl parsers only ever
// looked for `=` — so the loop variable of every for-in loop was undeclared.
// `catch (any e)` was the same, in every catch block there is.
func TestLoopAndCatchVariablesAreDeclared(t *testing.T) {
	cases := []struct {
		body string
		want string
	}{
		{`for (var row in qry) { writeOutput(row); }`, "row"},
		{`for (var k in data) { writeOutput(k); }`, "k"},
		{`try { x(); } catch (any e) { writeOutput(e); }`, "e"},
		{`try { x(); } catch (e) { writeOutput(e); }`, "e"},
		{`try { x(); } catch (org.Foo err) { writeOutput(err); }`, "err"},
		{`for (var i = 1; i <= 10; i++) { writeOutput(i); }`, "i"},
	}

	for _, c := range cases {
		t.Run(c.body, func(t *testing.T) {
			pr := Parse(testURI, "component {\n function go() {\n  "+c.body+"\n }\n}\n")

			scope := pr.Scopes[0]

			got := pr.FuncVars(scope.Start, scope.End)
			if !slices.Contains(got, c.want) {
				t.Errorf("expected %q in FuncVars, got %v", c.want, got)
			}
		})
	}
}

// Unscoped, the loop variable is a variables-scope binding — CFML's rule for
// any unscoped assignment, and the reason the `var` form is the one to write.
func TestUnscopedLoopVariableIsVariablesScoped(t *testing.T) {
	pr := Parse(testURI, "component {\n function go() {\n  for (row in qry) { writeOutput(row); }\n }\n}\n")

	var found bool

	for _, v := range pr.AllVars() {
		if v.Name == "row" && v.Scope == ScopeVariables {
			found = true
		}
	}

	if !found {
		t.Errorf("expected a variables-scope 'row', got %+v", pr.AllVars())
	}
}

func callTargets(t *testing.T, body string) []string {
	t.Helper()

	pr := ParseWithOptions(testURI, "component {\n function go() {\n  "+body+"\n }\n}\n",
		&ParseOptions{ExtractCalls: true})
	scope := pr.Scopes[0]

	var got []string

	cs := pr.FuncCalls(scope.Start, scope.End)

	for i := range cs {
		c := &cs[i]

		switch {
		case c.Variable != "":
			got = append(got, c.Variable+"."+c.FuncName)
		case c.Component != "":
			got = append(got, c.Component+"::"+c.FuncName)
		default:
			got = append(got, "?."+c.FuncName)
		}
	}

	return got
}

// `?.` was two unrecognised tokens, so the receiver was dropped and the call
// was recorded as a bare one — an unqualified function call, which is a wrong
// answer rather than a missing one. The scanner folds it into a dot, so every
// chain walk sees it without a case of its own.
func TestSafeNavigationKeepsTheReceiver(t *testing.T) {
	cases := []struct {
		body string
		want []string
	}{
		{`svc?.save();`, []string{"svc.save"}},
		{`svc?.a()?.b();`, []string{"svc.a", "svc.b"}},
		{`svc?.save(force = true);`, []string{"svc.save"}},
		{`variables.svc?.save();`, []string{"variables.svc.save"}},
	}

	for _, c := range cases {
		t.Run(c.body, func(t *testing.T) {
			if got := callTargets(t, c.body); !slices.Equal(got, c.want) {
				t.Errorf("got %v want %v", got, c.want)
			}
		})
	}
}

// A ternary's question mark is not safe navigation. Adjacency is what tells
// them apart, so only `?.` folds.
func TestTernaryIsNotSafeNavigation(t *testing.T) {
	sc := NewScanner("a ? b : c")

	var kinds []TokenKind

	for {
		tok := sc.NextSkipComments()
		if tok.Kind == TokEOF {
			break
		}

		kinds = append(kinds, tok.Kind)
	}

	want := []TokenKind{TokIdent, TokQuestion, TokIdent, TokColon, TokIdent}
	if !slices.Equal(kinds, want) {
		t.Errorf("got %v want %v", kinds, want)
	}
}

// Static member access is qualified by a *component*, not by a variable holding
// one. Dropping the `::` reported `Foo::bar()` as `bar (no qualifier, not in
// file)`, which names the wrong problem, and left the call unresolvable even
// when the component was right there on disk.
//
// Every assignment form is listed because the parser walks a chain in five
// separate places and each one needs the case: three of the five were still
// reporting a bare `bar` when the first two were done.
func TestStaticCallCarriesItsComponent(t *testing.T) {
	cases := []struct {
		body string
		want []string
	}{
		{`Foo::bar();`, []string{"Foo::bar"}},
		{`models.Foo::bar();`, []string{"models.Foo::bar"}},
		{`x = Foo::bar();`, []string{"Foo::bar"}},
		{`var x = models.Foo::bar();`, []string{"models.Foo::bar"}},
		{`variables.x = Foo::bar();`, []string{"Foo::bar"}},
		{`local.x = models.Foo::bar();`, []string{"models.Foo::bar"}},
		{`this.x = Foo::bar();`, []string{"Foo::bar"}},

		// Controls: a single colon is not static access.
		{`svc.bar();`, []string{"svc.bar"}},
		{`x = cond ? a() : b();`, []string{"?.a", "?.b"}},
	}

	for _, c := range cases {
		t.Run(c.body, func(t *testing.T) {
			if got := callTargets(t, c.body); !slices.Equal(got, c.want) {
				t.Errorf("got %v want %v", got, c.want)
			}
		})
	}
}

// A component's or interface's attribute list is not a run of assignments.
// parsePlain's keyword guard runs before its tag-attribute check and both names
// are keywords, so `component extends="models.Base" accessors="true"` put
// `extends` and `accessors` into VariablesVars — which is what completion
// offers and what the index stores.
func TestComponentAttributesAreNotVariables(t *testing.T) {
	cases := []struct {
		src     string
		want    []string
		extends string
	}{
		{"component extends=\"models.Base\" accessors=\"true\" output=\"false\" {\n\tvariables.real = 1;\n}\n",
			[]string{"real"}, "models.Base"},
		{"interface extends=\"IBase\" {\n\tpublic string function getName();\n}\n", nil, "IBase"},
		{"component {\n\tvariables.real = 1;\n}\n", []string{"real"}, ""},
	}

	for _, c := range cases {
		t.Run(c.src, func(t *testing.T) {
			pr := Parse(testURI, c.src)
			if got := pr.VariablesVars(); !slices.Equal(got, c.want) {
				t.Errorf("VariablesVars: got %v want %v", got, c.want)
			}

			var names []string
			for _, v := range ParseVars(c.src) {
				names = append(names, v.Name)
			}

			if !slices.Equal(names, c.want) {
				t.Errorf("ParseVars: got %v want %v", names, c.want)
			}

			if pr.Extends != c.extends {
				t.Errorf("Extends: got %q want %q", pr.Extends, c.extends)
			}
		})
	}
}

// `throw` is the one CFScript keyword invoked like a function, and being a
// keyword it never reached the dispatch that consumes an argument list — so its
// named arguments were read as statements.
func TestThrowArgumentsAreNotDeclarations(t *testing.T) {
	cases := []struct {
		body string
		want []string
	}{
		{`throw(type = "x", message = "y");`, nil},
		{`throw(object = new errors.Bad());`, nil},
		{`throw(message = svc.describe());`, nil},

		// Controls: the other keywords that take parentheses hold an
		// expression, and the statement scan reads those correctly as is.
		{`if (svc.check()) { hits = 1; }`, []string{"hits"}},
		{`for (var i = 1; i <= 3; i++) { use(i); }`, []string{"i"}},
	}

	for _, c := range cases {
		t.Run(c.body, func(t *testing.T) {
			src := "component {\n function go() {\n  " + c.body + "\n }\n}\n"

			var got []string
			for _, v := range ParseVars(src) {
				got = append(got, v.Name)
			}

			if !slices.Equal(got, c.want) {
				t.Errorf("vars: got %v want %v", got, c.want)
			}

			pr := ParseWithOptions(testURI, src, &ParseOptions{ExtractCalls: true})
			for _, r := range pr.FuncComponentRefs(pr.Scopes[0].Start, pr.Scopes[0].End) {
				if r.Variable == "object" {
					t.Errorf("recorded a component ref for a named argument: %s -> %s", r.Variable, r.Component)
				}
			}
		})
	}
}

// A nested function's body used to be discarded, so an immediately-invoked
// function lost everything it called — while the same closure passed as an
// argument kept its calls, because that path counts parentheses instead.
func TestNestedFunctionBodiesAreScanned(t *testing.T) {
	cases := []struct {
		body string
		want []string
	}{
		{`return (function(){ return svc.x(); })();`, []string{"svc.x"}},
		{`var f = function() { return svc.y(); }; return f();`, []string{"?.f", "svc.y"}},
		{`arrayEach(list, function(i) { svc.use(i); });`, []string{"?.arrayEach", "svc.use"}},
		{`function inner(a = dao.make()) { return a; } return inner();`, []string{"?.inner", "dao.make"}},
	}

	for _, c := range cases {
		t.Run(c.body, func(t *testing.T) {
			got := interpCalls(t, "component {\n function go() {\n  "+c.body+"\n }\n}\n")
			if !slices.Equal(got, c.want) {
				t.Errorf("got %v want %v", got, c.want)
			}
		})
	}
}

// An argument's default runs on every call that omits it, so a call in one is
// a call.
func TestArgumentDefaultsAreScannedForCalls(t *testing.T) {
	got := interpCalls(t, "component {\n function load(id, dao = newDao()) { return dao; }\n}\n")
	if !slices.Equal(got, []string{"?.newDao"}) {
		t.Errorf("got %v want [?.newDao]", got)
	}
}

// `import models.User;` puts User in scope, and `new User()` then names
// models.User — not a component literally called User, which is what the
// resolver went looking for.
func TestImportQualifiesABareComponentName(t *testing.T) {
	cases := []struct {
		src  string
		want string
	}{
		{"import models.User;\ncomponent { function go() { var u = new User(); } }", "models.User"},
		{"import models.User;\ncomponent { function go() { var u = new cfml:User(); } }", "models.User"},

		// A dotted path is already qualified; a wildcard names a directory and
		// which component a bare name then means is a question about disk.
		{"import models.User;\ncomponent { function go() { var u = new other.User(); } }", "other.User"},
		{"import models.*;\ncomponent { function go() { var u = new User(); } }", "User"},
		{"component { function go() { var u = new User(); } }", "User"},
	}

	for _, c := range cases {
		t.Run(c.src, func(t *testing.T) {
			pr := Parse(testURI, c.src)

			var got string

			for _, sc := range pr.Scopes {
				for _, r := range pr.FuncComponentRefs(sc.Start, sc.End) {
					if r.Variable == "u" {
						got = r.Component
					}
				}
			}

			if got != c.want {
				t.Errorf("got %q want %q", got, c.want)
			}
		})
	}
}

// A *named* function declared inside another function's body is a declaration,
// not a value: CFML hoists it into the component's variables scope, which is
// what lets the enclosing function call it before the line it is written on.
// The parser recorded nothing, so there was no index entry, no completion and
// nowhere for go-to-definition to land — while the tag parser had always
// recorded the same code, so the two syntaxes disagreed.
func TestNamedNestedFunctionsAreDeclared(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []string
		args int
	}{
		{
			"called before it is written",
			"setup();\n  function setup(required string a) { return a; }",
			[]string{"run", "setup"}, 1,
		},
		{
			"inside a closure argument",
			"describe(\"x\", function() { function helper(b, c) { return b; } });",
			[]string{"run", "helper"}, 2,
		},
		{
			"with an access modifier",
			"private function helper(b) { return b; }",
			[]string{"run", "helper"}, 1,
		},

		// An anonymous function is a value, not a declaration — a var-scoped
		// closure is private to the one function, and declaring it would put it
		// in every caller's completion list.
		{"anonymous stays a value", "var f = function(a) { return a; };", []string{"run"}, 0},
		{"anonymous arrow stays a value", "var f = (a) => a;", []string{"run"}, 0},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pr := Parse(testURI, "component {\n function run() {\n  "+c.body+"\n }\n}\n")

			got := make([]string, 0, len(pr.Funcs))
			for _, f := range pr.Funcs {
				got = append(got, f.Name)
			}

			if !slices.Equal(got, c.want) {
				t.Fatalf("funcs: got %v want %v", got, c.want)
			}

			if len(c.want) > 1 {
				if n := len(pr.Funcs[1].Arguments); n != c.args {
					t.Errorf("arguments: got %d want %d", n, c.args)
				}
			}
		})
	}
}

// The nested function's scope has to end at its own closing brace, not at the
// enclosing function's — scanNestedFunctionBody reports that line because it is
// the only thing that consumed it.
func TestANestedFunctionScopeEndsAtItsOwnBrace(t *testing.T) {
	src := "component {\n" + // 0
		" function run() {\n" + // 1
		"  function setup() {\n" + // 2
		"   return 1;\n" + // 3
		"  }\n" + // 4
		"  return setup();\n" + // 5
		" }\n" + // 6
		"}\n" // 7

	pr := Parse(testURI, src)

	want := []FuncScope{{Name: "run", Start: 1, End: 6}, {Name: "setup", Start: 2, End: 4}}
	if !slices.Equal(pr.Scopes, want) {
		t.Errorf("got %v want %v", pr.Scopes, want)
	}
}
