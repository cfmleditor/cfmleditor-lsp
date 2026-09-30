package resolve

import "testing"

// TestAReturnedScopedVariableTypesTheFunction: `return variables.print;`
// returns what variables.print holds, so a chain on the function's result is
// checked against that component. `this.x` and `variables.x` are separate
// stores, and each reads its own.
func TestAReturnedScopedVariableTypesTheFunction(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"PrintBuffer.cfc": `component { function line() {} }`,
		"Other.cfc":       `component { function go() {} }`,
		"Service.cfc": `component {
	function init() {
		variables.print = new PrintBuffer();
		this.other = new Other();
		return this;
	}
	function getPrint() { return variables.print; }
	function getOther() { return this.other; }
	function getWrong() { return variables.other; }
}`,
		"Page.cfc": `component {
	function f() {
		var s = new Service();
		s.getPrint().line();
		s.getPrint().missing();
		s.getOther().go();
		s.getOther().missing();
		s.getWrong().go();
	}
}`,
	})

	expectReasons(t, reasonsIn(t, dir, "Page.cfc"), map[string]string{
		"s.getPrint.line":    "",
		"s.getPrint.missing": "method 'missing' not found in PrintBuffer",
		"s.getOther.go":      "",
		"s.getOther.missing": "method 'missing' not found in Other",
		// variables.other is not this.other.
		"s.getWrong.go": "method 'getWrong' in Service has no component return type (chain to 'go')",
	})
}

// TestAReturnedInjectedPropertyTypesTheFunction is the case the gap was found
// on, cfwheels' DetailOutputService: `print` is an injected property, not an
// assignment, and the getter is declared before anything else in the file.
// The same holds in tag syntax.
func TestAReturnedInjectedPropertyTypesTheFunction(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"PrintBuffer.cfc": `component { function line() {} }`,
		"Service.cfc": `component {
	property name="print" inject="PrintBuffer";
	function getPrint() { return variables.print; }
	function getThisPrint() { return this.print; }
}`,
		"TagService.cfc": `<cfcomponent>
	<cfproperty name="print" inject="PrintBuffer">
	<cffunction name="getPrint">
		<cfreturn variables.print>
	</cffunction>
	<cffunction name="getLater">
		<cfreturn this.later />
	</cffunction>
	<cffunction name="init">
		<cfset this.later = new PrintBuffer()>
		<cfreturn this>
	</cffunction>
</cfcomponent>`,
		"Page.cfc": `component {
	function f() {
		var s = new Service();
		s.getPrint().line();
		s.getPrint().missing();
		s.getThisPrint().line();
		var t = new TagService();
		t.getPrint().line();
		t.getPrint().missing();
		t.getLater().missing();
	}
}`,
	})

	expectReasons(t, reasonsIn(t, dir, "Page.cfc"), map[string]string{
		"s.getPrint.line":    "",
		"s.getPrint.missing": "method 'missing' not found in PrintBuffer",
		// An injected property is in variables scope; this.print is
		// another variable, and nothing says what it holds.
		"s.getThisPrint.line": "method 'getThisPrint' in Service has no component return type (chain to 'line')",
		"t.getPrint.line":     "",
		"t.getPrint.missing":  "method 'missing' not found in PrintBuffer",
		"t.getLater.missing":  "method 'missing' not found in PrintBuffer",
	})
}

// TestATagMemberReturnIsNotTheReceiver checks the conservative answer for a
// member whose own type is unavailable. The tag parser used to read
// `<cfreturn parent.child>` as `<cfreturn parent>`, so `getChild().run()` was
// incorrectly checked against Parent.
func TestATagMemberReturnIsNotTheReceiver(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Parent.cfc": `component { function parentOnly() {} }`,
		"Service.cfc": `<cfcomponent>
	<cffunction name="getChild">
		<cfset var parent = new Parent()>
		<cfreturn parent.child>
	</cffunction>
</cfcomponent>`,
		"Page.cfc": `component {
	function f() {
		var service = new Service();
		service.getChild().run();
	}
}`,
	})

	expectReasons(t, reasonsIn(t, dir, "Page.cfc"), map[string]string{
		"service.getChild.run": "method 'getChild' in Service has no component return type (chain to 'run')",
	})
}
