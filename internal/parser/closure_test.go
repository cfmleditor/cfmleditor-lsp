package parser

import (
	"slices"
	"strings"
	"testing"
)

// closureSrc is a TestBox spec's shape: each test a closure inside run(),
// arrow and function forms, a parameter of each, a closure-local `var`, an
// unscoped assignment, and a closure's own `return`.
const closureSrc = `component {
	function run() {
		describe( "a", function( ctx ) {
			it( "arrow", () => {
				var t = new models.Arrow();
				t.go();
			} );
			it( "fn", function() {
				var t = new models.Fn();
				period = new models.Period();
				return new models.Returned();
			} );
			var names = rows.map( ( row ) => row.getName() );
		} );
		t = new models.After();
	}
}`

func refNamed(refs []ComponentRef, name string) *ComponentRef {
	for i := range refs {
		if strings.EqualFold(refs[i].Variable, name) {
			return &refs[i]
		}
	}

	return nil
}

// TestAClosureDeclaresWhatItsBodyDeclares: a closure's body was scanned for
// calls alone, so every `var` in one declared nothing and typed nothing —
// most of a TestBox spec, where each test is a closure. Its locals are
// declared, each visible only inside its closure; its parameters are
// declared, the arrow's and the function's, block-bodied or not; and an
// unscoped assignment in one lands at file level, as it does anywhere in a
// function.
func TestAClosureDeclaresWhatItsBodyDeclares(t *testing.T) {
	pr := ParseWithOptions(testURI, closureSrc, &ParseOptions{ExtractCalls: true})
	scope := pr.Scopes[0]

	// Lines are 0-based: `t.go()` in the arrow is 5, in the function 9 (the
	// assignment on 8 and the call would be on 9 had it one).
	for _, tc := range []struct {
		line uint32
		want string
	}{
		{5, "models.Arrow"},
		{9, "models.Fn"},
	} {
		ref := refNamed(pr.FuncComponentRefsAt(scope.Start, scope.End, tc.line), "t")
		if ref == nil || ref.Component != tc.want {
			t.Errorf("t at line %d: got %+v, want %s", tc.line, ref, tc.want)
		}
	}

	if ref := refNamed(pr.FuncComponentRefs(scope.Start, scope.End), "t"); ref != nil {
		t.Errorf("a closure's t is visible to the whole function: %+v", ref)
	}

	if ref := refNamed(pr.ComponentRefs, "period"); ref == nil || ref.Component != "models.Period" {
		t.Errorf("unscoped assignment in a closure: got %+v, want a file-level models.Period", ref)
	}

	vars := pr.FuncVars(scope.Start, scope.End)
	for _, name := range []string{"ctx", "row", "t", "names"} {
		if !slices.ContainsFunc(vars, func(v string) bool { return strings.EqualFold(v, name) }) {
			t.Errorf("%s not declared; FuncVars = %v", name, vars)
		}
	}
}

// TestAClosureLeavesTheEnclosingFunctionAlone: what a closure declares or
// returns is its own. Its `var t` does not make the function's later
// unscoped `t = …` a local, and its `return new …` is not the function's
// return type.
func TestAClosureLeavesTheEnclosingFunctionAlone(t *testing.T) {
	pr := ParseWithOptions(testURI, closureSrc, &ParseOptions{ExtractCalls: true})

	if ref := refNamed(pr.ComponentRefs, "t"); ref == nil || ref.Component != "models.After" {
		t.Errorf("the function's own unscoped t: got %+v, want a file-level models.After", ref)
	}

	if rc := pr.Funcs[0].ReturnComponent; rc != "" {
		t.Errorf("run() returns %q, the closure's return type", rc)
	}
}

// TestAnInlineComponentIsDynamic: `new component { … }` names no file. It was
// read as a path, a component literally called "component", and every method
// called on it failed its check.
func TestAnInlineComponentIsDynamic(t *testing.T) {
	src := "component {\n\tfunction f() {\n\t\tvar comp = new component {\n\t\t\tfunction getId() { return 1; }\n\t\t};\n\t\tcomp.getId();\n\t}\n}"
	pr := Parse(testURI, src)
	scope := pr.Scopes[0]

	if ref := refNamed(pr.FuncComponentRefs(scope.Start, scope.End), "comp"); ref == nil || ref.Component != "$any" {
		t.Errorf("comp: got %+v, want $any", ref)
	}
}

// TestMocksAndUnstubbedJavaAreDynamic: a MockBox mock's methods are added at
// runtime, and a Java object has nothing to be checked against without a
// stub, so both are $any rather than untyped — every call on one was "no
// component ref". A configured resolver still answers first, so a
// javaStubsPath keeps typing Java objects.
func TestMocksAndUnstubbedJavaAreDynamic(t *testing.T) {
	src := `component {
	function f() {
		var a = createMock( "models.User" );
		var b = prepareMock( entityNew( "User" ) );
		var c = getMockBox().createEmptyMock( "models.User" );
		var d = createObject( "java", "java.io.File" ).init( p );
		var e = createObject( 'java', 'java.lang.System' );
		var g = createObject( "component", "models.User" );
		var h = mockUser();
	}
}`
	pr := ParseWithOptions(testURI, src, &ParseOptions{})
	refs := pr.FuncComponentRefs(pr.Scopes[0].Start, pr.Scopes[0].End)

	for name, want := range map[string]string{
		"a": "$any", "b": "$any", "c": "$any", "d": "$any", "e": "$any",
		"g": "models.User", "h": "",
	} {
		got := ""
		if ref := refNamed(refs, name); ref != nil {
			got = ref.Component
		}

		if got != want {
			t.Errorf("%s: got %q, want %q", name, got, want)
		}
	}

	stubs := []Resolver{{Match: `^createObject\("java",\s*"(.+)"\)`, Resolve: "stubs.$1", Prefix: "createObject"}}
	pr = ParseWithOptions(testURI, src, &ParseOptions{Resolvers: stubs})

	if ref := refNamed(pr.FuncComponentRefs(pr.Scopes[0].Start, pr.Scopes[0].End), "d"); ref == nil || ref.Component == "$any" {
		t.Errorf("with a stub resolver d is %+v, want the stub's type", ref)
	}
}

// TestASiblingClosuresLocalDoesNotStandForThisOne: each test in a spec
// declares its own `t`, and a `t` typed after the parse — from a call's
// return type — was skipped when any closure in the function already had a
// ref called t, so every test after the first stayed untyped.
func TestASiblingClosuresLocalDoesNotStandForThisOne(t *testing.T) {
	src := `component {
	models.Widget function make() { return new models.Widget(); }
	function run() {
		it( "a", function() {
			var t = createMock( "models.Other" );
		} );
		it( "b", function() {
			var t = make();
			t.go();
		} );
	}
}`
	pr := ParseWithOptions(testURI, src, &ParseOptions{})

	var run FuncScope

	for _, sc := range pr.Scopes {
		if sc.Name == "run" {
			run = sc
		}
	}

	if ref := refNamed(pr.FuncComponentRefsAt(run.Start, run.End, 8), "t"); ref == nil || ref.Component != "models.Widget" {
		t.Errorf("t in the second test: got %+v, want models.Widget", ref)
	}
}
