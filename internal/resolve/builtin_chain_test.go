package resolve

import (
	"path/filepath"
	"testing"
)

// TestAChainOnABuiltinIsDynamic: getPageContext().getRequest() is a call on
// what the engine returns, not on a function the file lost track of. Lucee's
// test suite has 97 of them and 69 on CreateDateTime(); they were hidden
// while its TestBox base did not resolve. A function nobody declares, and a
// file's own function named like a built-in, are still what they were.
func TestAChainOnABuiltinIsDynamic(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Page.cfc": `component {
	function f() {
		getPageContext().getRequest();
		CreateDateTime( 2020, 1, 1 ).format( "yyyy" );
		nowhere().go();
		now().notAMethodOfThis();
	}
	function now() { return this; }
}`,
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "Page.cfc"), map[string]string{
		"getPageContext.getRequest": "",
		"CreateDateTime.format":     "",
		"nowhere.go":                "chained on 'nowhere', which is not found (calling 'go')",
		"now.notAMethodOfThis":      "method 'notAMethodOfThis' not found in Page",
	})
}

// TestAnInferredBareReturnIsBesideTheDeclaringFile: `return new
// Expectation( a )` in lib/BaseSpec.cfc names lib/Expectation.cfc, whatever
// directory the call is made from. Read from the caller's, it named nothing,
// and every expect( x ).toBe() in cfwheels' CLI specs — 3,099 of them — was a
// call on a component that does not exist. A dotted relative path is the
// same: ColdBox's AsyncManager returns `new tasks.Future()`.
func TestAnInferredBareReturnIsBesideTheDeclaringFile(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"lib/Expectation.cfc": `component { function toBe( v ) {} }`,
		// Nearer the caller, and not what the base returns: a file-name
		// search from the caller's directory takes this one.
		"specs/Expectation.cfc": `component { function other() {} }`,
		"lib/BaseSpec.cfc": `component {
	function expect( a ) { return new Expectation( a ); }
	Future function later() {
		return new tasks.Future();
	}
}`,
		"lib/tasks/Future.cfc": `component { function get() {} }`,
		// A relative path read from the caller's directory would take this.
		"specs/deep/tasks/Future.cfc": `component { function other() {} }`,
		"specs/deep/MySpec.cfc": `component extends="lib.BaseSpec" {
	function run() {
		expect( 1 ).toBe( 1 );
		expect( 1 ).notAMatcher();
		var base = new lib.BaseSpec();
		base.later().get();
	}
}`,
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "specs/deep/MySpec.cfc"), map[string]string{
		"expect.toBe":        "",
		"expect.notAMatcher": "method 'notAMatcher' not found in Expectation",
		"base.later.get":     "",
	})
}

// TestAComponentHasTheFunctionsOfWhatItIncludes: Wheels' Global.cfc is
// little but `include "global/functions.cfm"`, which includes the rest, and a
// qualified call on one — application.wo.$simpleLock() — looked only at the
// component's own functions. A template it does not include is still not
// its.
func TestAComponentHasTheFunctionsOfWhatItIncludes(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"wheels/Global.cfc":           `component { include "global/functions.cfm"; function own() {} }`,
		"wheels/global/functions.cfm": `<cfscript> include "locks.cfm"; function viaFunctions() {} </cfscript>`,
		"wheels/global/locks.cfm":     `<cfscript> function $simpleLock() {} </cfscript>`,
		"wheels/global/unused.cfm":    `<cfscript> function notIncluded() {} </cfscript>`,
		"Page.cfc": `component {
	function f() {
		var wo = new wheels.Global();
		wo.own();
		wo.viaFunctions();
		wo.$simpleLock();
		wo.notIncluded();
	}
}`,
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "Page.cfc"), map[string]string{
		"wo.own":          "",
		"wo.viaFunctions": "",
		"wo.$simpleLock":  "",
		"wo.notIncluded":  "method 'notIncluded' not found in wheels.Global",
	})
}

// TestARelativeReturnIsTheDeclaringFiles: ReturnComponentOf answers every
// question about what a function returns, and a relative path it hands on is
// read from wherever the question was asked. ColdBox's AsyncManager declares
// `Future function newFuture()` returning `new tasks.Future()`, and a spec in
// tests/specs/async resolved tasks.Future there: calls on a
// component that does not exist. The answer is the declaring file's.
func TestARelativeReturnIsTheDeclaringFiles(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"lib/Async.cfc":        "component {\n\tFuture function later() {\n\t\treturn new tasks.Future();\n\t}\n\ttasks.Future function declared() {}\n}",
		"lib/tasks/Future.cfc": `component { function get() {} }`,
	})

	r := &Resolver{}
	_ = reasonsWith(t, r, dir, "lib/Async.cfc")
	want := filepath.Join(dir, "lib", "tasks", "Future.cfc")

	for _, fn := range []string{"later", "declared"} {
		d := r.LookupFuncWithExtends(filepath.Join(dir, "lib", "Async.cfc"), fn)
		if d == nil {
			t.Fatalf("no %s", fn)
		}

		if got := r.ReturnComponentOf(d); got != want {
			t.Errorf("%s returns %q, want %q", fn, got, want)
		}
	}
}
