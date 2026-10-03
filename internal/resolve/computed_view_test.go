package resolve

import "testing"

// TestAComputedViewAndKeyAreReadPerSubclass: a base handler renders
// "#variables.handler#/index" and hands the view results[ variables.entityPlural ],
// and each subclass sets both to its own literals and holds its own service.
// A view's loop holds what that subclass's service returns under that subclass's
// key, not what every subclass's does.
func TestAComputedViewAndKeyAreReadPerSubclass(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"models/Page.cfc":  `component persistent="true" entityname="cbPage" { function pageOnly(){} }`,
		"models/Entry.cfc": `component persistent="true" entityname="cbEntry" { function entryOnly(){} }`,
		"svc/PageSvc.cfc": `component extends="cborm.models.VirtualEntityService" {
function init(){ super.init( entityName = "cbPage" ); return this; }
struct function search(){
	var results = { "count": 0, "pages": [] };
	var c = newCriteria();
	results.pages = c.list( asQuery = false );
	return results;
}
}`,
		"svc/EntrySvc.cfc": `component extends="cborm.models.VirtualEntityService" {
function init(){ super.init( entityName = "cbEntry" ); return this; }
struct function search(){
	var results = { "count": 0, "items": [] };
	var c = newCriteria();
	results.items = c.list( asQuery = false );
	return results;
}
}`,
		"admin/handlers/baseThing.cfc": `component {
variables.handler = "";
variables.entityPlural = "";
function index( event, rc, prc ){
	var results = variables.svc.search();
	prc.content = results[ variables.entityPlural ];
	event.setView( "#variables.handler#/index" );
}
}`,
		"admin/handlers/pages.cfc": `component extends="baseThing" {
variables.handler = "pages";
variables.entityPlural = "pages";
function init(){ variables.svc = new svc.PageSvc(); return this; }
}`,
		"admin/handlers/entries.cfc": `component extends="baseThing" {
variables.handler = "entries";
variables.entityPlural = "items";
function init(){ variables.svc = new svc.EntrySvc(); return this; }
}`,
		"admin/views/pages/index.cfm":   `<cfloop array="#prc.content#" index="p"><cfoutput>#p.pageOnly()# #p.entryOnly()#</cfoutput></cfloop>`,
		"admin/views/entries/index.cfm": `<cfloop array="#prc.content#" index="e"><cfoutput>#e.entryOnly()# #e.pageOnly()#</cfoutput></cfloop>`,
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "admin/views/pages/index.cfm"), map[string]string{
		"p.pageOnly":  "",
		"p.entryOnly": "method 'entryOnly' not found in Page",
	})
	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "admin/views/entries/index.cfm"), map[string]string{
		"e.entryOnly": "",
		"e.pageOnly":  "method 'pageOnly' not found in Entry",
	})
}
