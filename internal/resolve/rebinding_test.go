package resolve

import "testing"

// TestACallInAnAssignmentReadsTheValueBeforeIt: in `x = x.next()` the call
// is made on what x held before the line, and the ref the assignment makes
// is what x holds after it. Receivers are read from the latest ref at or
// before the call's line, which on that line is the assignment's own ref, so
// next() was looked for in what next() returns. Mura writes
// `pluginEvent = pluginEvent.init( data ).getEvent()` throughout.
func TestACallInAnAssignmentReadsTheValueBeforeIt(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"models/A.cfc": `component { function next(){ return new models.B(); } }`,
		"models/B.cfc": `component { function onlyB(){} }`,
		"Script.cfc": `component {
function local_(){
	var x = new models.A();
	x = x.next();
	x.onlyB();
}
function scoped(){
	var unrelated = new models.B();
	variables.y = new models.A();
	variables.y = variables.y.next();
	variables.y.onlyB();
}
// No local of its own, so FuncRefs parses the body itself.
function noLocals(){
	variables.w = new models.A();
	variables.w = variables.w.next();
	variables.w.onlyB();
}
}`,
		"Tag.cfc": `<cfcomponent>
<cffunction name="f">
	<cfset var z = createObject("component", "models.A")>
	<cfset z = z.next()>
	<cfset z.onlyB()>
</cffunction>
</cfcomponent>`,
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "Script.cfc"), map[string]string{
		"x.next":            "",
		"x.onlyB":           "",
		"variables.y.next":  "",
		"variables.y.onlyB": "",
		"variables.w.next":  "",
		"variables.w.onlyB": "",
	})
	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "Tag.cfc"), map[string]string{
		"z.next":  "",
		"z.onlyB": "",
	})
}
