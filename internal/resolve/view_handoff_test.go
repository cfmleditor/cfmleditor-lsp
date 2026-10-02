package resolve

import "testing"

// TestAViewReadsThePrcItsHandlerActionAssigns: a ColdBox view's prc.X is what
// the handler actions rendering it (event.setView with the view's name)
// assign it before rendering, typed in the handler, element types included.
// Actions that disagree, an action that does not assign it, and a view no
// setView names leave it untyped.
func TestAViewReadsThePrcItsHandlerActionAssigns(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"models/Thing.cfc":  `component { function go(){} }`,
		"models/Other.cfc":  `component { function other(){} }`,
		"models/Item.cfc":   `component persistent="true" { function getName(){} }`,
		"models/Holder.cfc": `component persistent="true" { property name="items" fieldtype="one-to-many" cfc="Item"; }`,
		"admin/handlers/things.cfc": `component {
function index( event, rc, prc ){
	prc.thing = new models.Thing();
	var holder = new models.Holder();
	prc.items = holder.getItems();
	event.setView( "things/index" );
}
function first( event, rc, prc ){
	prc.mixed = new models.Thing();
	event.setView( "things/mixed" );
}
function second( event, rc, prc ){
	prc.mixed = new models.Other();
	event.setView( view = "things/mixed", layout = "ajax" );
}
// unset.cfm is also rendered by an action that never assigns prc.thing:
// a pre-handler or interceptor may, and the view cannot tell.
function alsoUnset( event, rc, prc ){
	prc.thing = new models.Thing();
	event.setView( "things/unset" );
}
function unset( event, rc, prc ){
	event.setView( "things/unset" );
}
}`,
		"admin/views/things/index.cfm": `<cfoutput>#prc.thing.go()# #prc.thing.nope()#</cfoutput>
<cfloop array="#prc.items#" index="item"><cfoutput>#item.getName()#</cfoutput></cfloop>`,
		"admin/views/things/mixed.cfm":   `<cfoutput>#prc.mixed.go()#</cfoutput>`,
		"admin/views/things/unset.cfm":   `<cfoutput>#prc.thing.go()#</cfoutput>`,
		"admin/views/things/partial.cfm": `<cfoutput>#prc.thing.go()#</cfoutput>`,
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "admin/views/things/index.cfm"), map[string]string{
		"prc.thing.go":   "",
		"prc.thing.nope": "method 'nope' not found in Thing",
		"item.getName":   "",
	})
	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "admin/views/things/mixed.cfm"), map[string]string{
		"prc.mixed.go": "variable 'prc.mixed' has no component ref",
	})
	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "admin/views/things/unset.cfm"), map[string]string{
		"prc.thing.go": "variable 'prc.thing' has no component ref",
	})
	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "admin/views/things/partial.cfm"), map[string]string{
		"prc.thing.go": "variable 'prc.thing' has no component ref",
	})
}

// TestAPartialReadsThePrcOfWhatRendersIt: view() and renderView() render a
// view as setView does, from a handler action that returns a viewlet or from a
// view rendering a partial, and a partial's prc is its renderer's. A parent
// view that assigns the member itself, and a view() naming another module,
// do not hand it on.
func TestAPartialReadsThePrcOfWhatRendersIt(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"models/Thing.cfc": `component { function go(){} }`,
		"admin/handlers/things.cfc": `component {
function pager( event, rc, prc ){
	prc.pagerThing = new models.Thing();
	return view( view = "things/pager", module = "admin" );
}
function edit( event, rc, prc ){
	prc.thing = new models.Thing();
	event.setView( "things/edit" );
}
function elsewhere( event, rc, prc ){
	prc.other = new models.Thing();
	return view( view = "things/foreign", module = "otherModule" );
}
}`,
		"admin/views/things/pager.cfm":   `<cfoutput>#prc.pagerThing.go()#</cfoutput>`,
		"admin/views/things/edit.cfm":    `<cfoutput>#view( "things/sidebar" )# #renderView( view = "things/reset" )#</cfoutput>`,
		"admin/views/things/sidebar.cfm": `<cfoutput>#prc.thing.go()#</cfoutput>`,
		"admin/views/things/reset.cfm":   `<cfoutput>#prc.thing.go()#</cfoutput>`,
		"admin/views/things/foreign.cfm": `<cfoutput>#prc.other.go()#</cfoutput>`,
	})

	// edit.cfm reassigns nothing, so both partials read the action's prc.thing;
	// a second parent that reassigns it makes reset.cfm unknown.
	writeFiles(t, dir, map[string]string{
		"admin/views/things/resetParent.cfm": `<cfset prc.thing = "plain"><cfoutput>#view( "things/reset" )#</cfoutput>`,
		"admin/handlers/more.cfc": `component {
function resetParent( event, rc, prc ){
	prc.thing = new models.Thing();
	event.setView( "things/resetParent" );
}
}`,
	})

	for page, want := range map[string]map[string]string{
		"admin/views/things/pager.cfm":   {"prc.pagerThing.go": ""},
		"admin/views/things/sidebar.cfm": {"prc.thing.go": ""},
		"admin/views/things/reset.cfm":   {"prc.thing.go": "variable 'prc.thing' has no component ref"},
		"admin/views/things/foreign.cfm": {"prc.other.go": "variable 'prc.other' has no component ref"},
	} {
		expectReasons(t, reasonsWith(t, &Resolver{}, dir, page), want)
	}
}
