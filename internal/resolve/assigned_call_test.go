package resolve

import "testing"

// TestAVariableAssignedFromASubclassHeldReceiverIsTyped: an abstract handler
// writes `var o = variables.svc.get( 1 )` and `prc.item = variables.svc.get( 1 )`
// where only its subclasses hold svc. The variable, and prc.item in the view
// the action renders, hold what each subclass's service returns, as
// alternatives; one service returning no component leaves them untyped.
func TestAVariableAssignedFromASubclassHeldReceiverIsTyped(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"models/Item.cfc":  `component { function itemOnly(){} }`,
		"models/Other.cfc": `component { function otherOnly(){} }`,
		"svc/AService.cfc": `component { models.Item function get( id ){} }`,
		"svc/BService.cfc": `component { models.Other function get( id ){} }`,
		"svc/CService.cfc": `component { function get( id ){} }`,
		"handlers/base.cfc": `component {
function run(){
	var o = variables.svc.get( 1 );
	o.itemOnly();
	o.otherOnly();
	o.nope();
}
function quickLook( event, rc, prc ){
	prc.item = variables.svc.get( event.getValue( "id", 0 ) );
	event.setView( "content/quickLook" );
}
}`,
		"handlers/a.cfc":              `component extends="base" { function init(){ variables.svc = new svc.AService(); return this; } }`,
		"handlers/b.cfc":              `component extends="base" { function init(){ variables.svc = new svc.BService(); return this; } }`,
		"views/content/quickLook.cfm": `<cfoutput>#prc.item.itemOnly()# #prc.item.otherOnly()# #prc.item.nope()#</cfoutput>`,

		"mixed/mixedBase.cfc": `component { function run(){
	var o = variables.svc.get( 1 );
	o.itemOnly();
} }`,
		"mixed/mixedA.cfc": `component extends="mixedBase" { function init(){ variables.svc = new svc.AService(); return this; } }`,
		"mixed/mixedC.cfc": `component extends="mixedBase" { function init(){ variables.svc = new svc.CService(); return this; } }`,
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "handlers/base.cfc"), map[string]string{
		"o.itemOnly":  "",
		"o.otherOnly": "",
		"o.nope":      "method 'nope' not found in models.Item|models.Other",
	})
	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "views/content/quickLook.cfm"), map[string]string{
		"prc.item.itemOnly":  "",
		"prc.item.otherOnly": "",
		"prc.item.nope":      "method 'nope' not found in Other",
	})
	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "mixed/mixedBase.cfc"), map[string]string{
		"o.itemOnly": "variable 'o' has no component ref",
	})
}
