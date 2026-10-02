package parser

import "testing"

// A function returning a variable returns what the variable holds at the
// return, which is its latest assignment at or before that line, as
// funcScopedRef and fileLevelRef read a receiver. The first assignment was
// taken, so `var x = new A(); x = new B(); return x;` returned an A.
func TestReturnTypeIsTheAssignmentReachingTheReturn(t *testing.T) {
	cases := []struct {
		name, content string
	}{
		{"script reassigned", `component {
function f() {
	var x = new models.A();
	x = new models.B();
	return x;
}
}`},
		{"script try catch", `component {
function f() {
	try {
		var x = new models.A();
	} catch ( any e ) {
		x = new models.B();
	}
	return x;
}
}`},
		{"script no semicolons", `component {
function f() {
	var x = new models.A()
	x = new models.B()
	return x
}
}`},
		{"script deferred", `component {
function makeB() { return new models.B(); }
function f() {
	var x = new models.A();
	x = makeB();
	return x;
}
}`},
		{"script variables scope", `component {
function init() { variables.x = new models.A(); }
function f() {
	variables.x = new models.B();
	return variables.x;
}
}`},
		{"tag reassigned", `<cfcomponent>
<cffunction name="f">
	<cfset var x = createObject("component", "models.A")>
	<cfset x = createObject("component", "models.B")>
	<cfreturn x>
</cffunction>
</cfcomponent>`},
		{"tag deferred", `<cfcomponent>
<cffunction name="makeB"><cfreturn createObject("component", "models.B")></cffunction>
<cffunction name="f">
	<cfset var x = createObject("component", "models.A")>
	<cfset x = makeB()>
	<cfreturn x>
</cffunction>
</cfcomponent>`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pr := Parse(testURI, tc.content)

			var f *FunctionDef

			for i := range pr.Funcs {
				if pr.Funcs[i].Name == "f" {
					f = &pr.Funcs[i]
				}
			}

			if f == nil {
				t.Fatal("no function f")
			}

			if f.ReturnComponent != "models.B" {
				t.Errorf("f returns %q, want models.B", f.ReturnComponent)
			}
		})
	}
}

// An assignment after the return does not reach it.
func TestReturnTypeIgnoresAssignmentsAfterTheReturn(t *testing.T) {
	pr := Parse(testURI, `component {
function f() {
	var x = new models.A();
	if ( true ) {
		return x;
	}
	x = new models.B();
}
}`)

	if len(pr.Funcs) != 1 || pr.Funcs[0].ReturnComponent != "models.A" {
		t.Errorf("got %+v, want f returning models.A", pr.Funcs)
	}
}

// A function whose return settles only once a call has typed its variable
// still types the calls made on it. ColdBox's Injector assigns
// `variables.binder = buildBinder()`, and buildBinder's return waited on a
// call that the assignment had already been looked at before.
func TestACallOnALateSettledReturnIsTyped(t *testing.T) {
	pr := Parse(testURI, `component {
function init() { variables.binder = build(); }
function make() { return new models.B(); }
function build() {
	var x = new models.A();
	x = make();
	return x;
}
}`)

	ref := firstRefNamed(pr.ComponentRefs, "binder")
	if ref == nil || ref.Component != "models.B" {
		t.Errorf("binder is %+v, want models.B", ref)
	}
}

// An argument is not the component's variable of the same name.
func TestAnArgumentsReceiverIsNotTheComponentsVariable(t *testing.T) {
	pr := Parse(testURI, `component {
function init( binder ) {
	variables.binder = new models.A();
	var x = arguments.binder.make();
	return x;
}
}`)

	if got := pr.Funcs[0].ReturnComponent; got != "" {
		t.Errorf("init returns %q, want nothing", got)
	}
}

// A method called on a value is not the file's function of the same name.
func TestAMethodOnAValueIsNotTheFilesFunction(t *testing.T) {
	pr := Parse(testURI, `component {
function init() { return new models.A(); }
function f( binder ) {
	var x = arguments.binder.init();
	var y = this.init();
	return x;
}
}`)

	for _, want := range []struct {
		name string
		ok   bool
	}{{"x", false}, {"y", true}} {
		got := false

		for _, refs := range pr.funcRefsMap {
			for i := range refs {
				if refs[i].Variable == want.name && refs[i].Component == "models.A" {
					got = true
				}
			}
		}

		if got != want.ok {
			t.Errorf("%s typed models.A: %t, want %t", want.name, got, want.ok)
		}
	}
}

// An assignment rebinds its variable when the call on its right-hand side is
// made on that variable; an argument or a member of the same name is another.
func TestRebindsNamesTheVariableItself(t *testing.T) {
	for _, tc := range []struct {
		varName, recv string
		want          bool
	}{
		{"x", "x", true},
		{"x", "local.x", true},
		{"variables.x", "variables.x", true},
		{"x", "variables.X", true},
		{"x", "arguments.x", false},
		{"x", "y.x", false},
		{"x", "variables.x.y", false},
		{"x", "y", false},
		{"x", "", false},
	} {
		if got := rebinds(tc.varName, tc.recv); got != tc.want {
			t.Errorf("rebinds(%q, %q) = %t, want %t", tc.varName, tc.recv, got, tc.want)
		}
	}
}

// An unscoped argument or local is not the component's variable of the same
// name either, in either syntax.
func TestALocalReceiverIsNotTheComponentsVariable(t *testing.T) {
	for _, src := range []string{
		`component {
function setup() { svc = new models.A(); }
function f( svc ) {
	var x = svc.make();
	return x;
}
}`,
		`component {
function setup() { svc = new models.A(); }
function f() {
	var svc = arguments.other;
	var x = svc.make();
	return x;
}
}`,
		`<cfcomponent>
<cffunction name="setup"><cfset svc = createObject("component", "models.A")></cffunction>
<cffunction name="f">
	<cfargument name="svc">
	<cfset var x = svc.make()>
	<cfreturn x>
</cffunction>
</cfcomponent>`,
	} {
		pr := Parse(testURI, src)
		for i := range pr.Funcs {
			if f := &pr.Funcs[i]; f.Name == "f" && f.ReturnComponent != "" {
				t.Errorf("f returns %q, want nothing:\n%s", f.ReturnComponent, src)
			}
		}
	}
}

// A tag region after a <cfscript> block starts partway down the file, and the
// return's line is shifted with everything else the region recorded. It was
// not, so the return was read at a line above both assignments and the first
// one was taken.
func TestATagReturnAfterAScriptBlockReadsItsOwnLine(t *testing.T) {
	pr := Parse(testURI, `<cfcomponent>
<cfscript>
	variables.ready = true;
</cfscript>
<cffunction name="f">
	<cfset var x = createObject("component", "models.A")>
	<cfset x = createObject("component", "models.B")>
	<cfreturn x>
</cffunction>
</cfcomponent>`)

	for i := range pr.Funcs {
		if f := &pr.Funcs[i]; f.Name == "f" && f.ReturnComponent != "models.B" {
			t.Errorf("f returns %q, want models.B", f.ReturnComponent)
		}
	}
}
