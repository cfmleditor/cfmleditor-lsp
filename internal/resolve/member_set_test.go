package resolve

import "testing"

// TestAMethodAssignedOntoAnObjectIsDynamic: fw1's CircularTest gives two
// beans a method at run time — `a.getVariables = getVariables;` — and calls
// it. The call is to what was stored there, not to a method of a's
// component. Only an assignment earlier in the same function counts, and
// only to that member of that variable.
func TestAMethodAssignedOntoAnObjectIsDynamic(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Bean.cfc": `component { function own() {} }`,
		"Spec.cfc": `component {
	function f() {
		var a = new Bean();
		var b = new Bean();
		b.early();
		var d = new Bean();
		d.late();
		d.late = getVariables;
		a.getVariables = getVariables;
		b.getVariables == getVariables;
		a.getVariables();
		b.getVariables();
		a.missing();
	}
	function g() {
		var c = new Bean();
		c.getVariables = getVariables;
	}
	function h() {
		var c = new Bean();
		c.getVariables();
	}
	private function getVariables() { return variables; }
}`,
	})

	got := reasonsIn(t, dir, "Spec.cfc")
	expectReasons(t, got, map[string]string{
		"a.getVariables": "",
		"b.getVariables": "method 'getVariables' not found in Bean",
		"b.early":        "method 'early' not found in Bean",
		"a.missing":      "method 'missing' not found in Bean",
		"d.late":         "method 'late' not found in Bean",
		// Assigned in g(), called in h(): another variable.
		"c.getVariables": "method 'getVariables' not found in Bean",
	})
}

// TestAMethodAssignedOntoAnObjectByCFSetIsDynamic keeps tag syntax equivalent
// to cfscript: the expression parser sees the assignment even though it has no
// call of its own, and a comparison or a later assignment does not hide a
// missing method.
func TestAMethodAssignedOntoAnObjectByCFSetIsDynamic(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Bean.cfc": `<cfcomponent><cffunction name="own"></cffunction></cfcomponent>`,
		"Spec.cfc": `<cfcomponent>
<cffunction name="f">
	<cfset a = new Bean()>
	<cfset b = new Bean()>
	<cfset c = new Bean()>
	<cfset a.getVariables = getVariables>
	<cfset b.getVariables == getVariables>
	<cfset a.getVariables()>
	<cfset b.getVariables()>
	<cfset c.missing()>
	<cfset c.missing = getVariables>
</cffunction>
<cffunction name="getVariables" access="private"><cfreturn variables></cffunction>
</cfcomponent>`,
	})

	expectReasons(t, reasonsIn(t, dir, "Spec.cfc"), map[string]string{
		"a.getVariables": "",
		"b.getVariables": "method 'getVariables' not found in Bean",
		"c.missing":      "method 'missing' not found in Bean",
	})
}
