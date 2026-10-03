package resolve

import "testing"

// TestAStructFieldHoldsTheCriteriaListItsFunctionAssignsIt: a service's
// search() returns a struct whose collection field is a cborm criteria list()
// over the service's entity, and the handler hands that field to a view's loop.
// Each negative removes one thing the answer rests on.
func TestAStructFieldHoldsTheCriteriaListItsFunctionAssignsIt(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"orm/VirtualEntityService.cfc": `component { function init( entityName ){ return this; } function newCriteria(){} function getAll(){} }`,
		"models/Comment.cfc":           `component persistent="true" entityname="cbComment" { function getText(){} }`,
		"models/CommentService.cfc": `component extends="orm.VirtualEntityService" {
function init(){ super.init( entityName = "cbComment" ); return this; }
struct function search( boolean asQuery = false ){
	var results = { "count": 0, "comments": [], "queried": [], "twice": [], "dyn": [] };
	var c = newCriteria();
	results.count = c.count();
	results.comments = c.list( offset = 0, max = 10, asQuery = false );
	results.queried = c.list( asQuery = true );
	results.twice = c.list( asQuery = false );
	results.twice = getOther();
	results.dyn = c.resultTransformer( c.DISTINCT_ROOT_ENTITY ).list( asQuery = arguments.asQuery );
	return results;
}
struct function chained(){
	var results = {};
	var c = newCriteria().isEq( "a", 1 ).createAlias( "b", "b" );
	results.rows = c.list( asQuery = false );
	return results;
}
struct function chainedOther(){
	var results = {};
	var c = newCriteria().whatever( 1 );
	results.rows = c.list( asQuery = false );
	return results;
}
struct function omitted(){
	var results = { "comments": [] };
	var c = newCriteria();
	results.comments = c.list( max = 10 );
	return results;
}
struct function escapes(){
	var results = { "comments": [] };
	var c = newCriteria();
	results.comments = c.list( asQuery = false );
	log( results );
	return results;
}
}`,
		"admin/handlers/comments.cfc": `component {
function index( event, rc, prc ){
	var svc = new models.CommentService();
	var found = svc.search();
	prc.comments = found.comments;
	prc.queried = found.queried;
	prc.twice = found.twice;
	prc.dyn = found.dyn;
	prc.count = found.count;
	event.setView( "comments/index" );
}
function chained( event, rc, prc ){
	var svc = new models.CommentService();
	var found = svc.chained(
		1,
		2
	);
	prc.rows = found.rows;
	event.setView( "comments/chained" );
}
function chainedOther( event, rc, prc ){
	var svc = new models.CommentService();
	var found = svc.chainedOther();
	prc.rows = found.rows;
	event.setView( "comments/chainedOther" );
}
function forced( event, rc, prc ){
	var svc = new models.CommentService();
	var found = svc.search( asQuery = true );
	prc.dyn = found.dyn;
	event.setView( "comments/forced" );
}
function omitted( event, rc, prc ){
	var svc = new models.CommentService();
	var found = svc.omitted();
	prc.comments = found.comments;
	event.setView( "comments/omitted" );
}
function escapes( event, rc, prc ){
	var svc = new models.CommentService();
	var found = svc.escapes();
	prc.comments = found.comments;
	event.setView( "comments/escapes" );
}
}`,
		"admin/views/comments/index.cfm": `<cfloop array="#prc.comments#" index="a"><cfoutput>#a.getText()# #a.nope()#</cfoutput></cfloop>
<cfloop array="#prc.queried#" index="b"><cfoutput>#b.getText()#</cfoutput></cfloop>
<cfloop array="#prc.twice#" index="c"><cfoutput>#c.getText()#</cfoutput></cfloop>
<cfloop array="#prc.dyn#" index="d"><cfoutput>#d.getText()#</cfoutput></cfloop>`,
		"admin/views/comments/chained.cfm":      `<cfloop array="#prc.rows#" index="g"><cfoutput>#g.getText()#</cfoutput></cfloop>`,
		"admin/views/comments/chainedOther.cfm": `<cfloop array="#prc.rows#" index="h"><cfoutput>#h.getText()#</cfoutput></cfloop>`,
		"admin/views/comments/forced.cfm":       `<cfloop array="#prc.dyn#" index="d"><cfoutput>#d.getText()#</cfoutput></cfloop>`,
		"admin/views/comments/omitted.cfm":      `<cfloop array="#prc.comments#" index="e"><cfoutput>#e.getText()#</cfoutput></cfloop>`,
		"admin/views/comments/escapes.cfm":      `<cfloop array="#prc.comments#" index="f"><cfoutput>#f.getText()#</cfoutput></cfloop>`,
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "admin/views/comments/index.cfm"), map[string]string{
		"a.getText": "",
		"a.nope":    "method 'nope' not found in Comment",
		"b.getText": "variable 'b' has no component ref",
		"c.getText": "variable 'c' has no component ref",
		"d.getText": "", // asQuery is a parameter defaulting to false, and search() leaves it
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "admin/views/comments/chained.cfm"), map[string]string{"g.getText": ""})
	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "admin/views/comments/chainedOther.cfm"), map[string]string{
		"h.getText": "variable 'h' has no component ref",
	})

	for page, variable := range map[string]string{"forced": "d", "omitted": "e", "escapes": "f"} {
		expectReasons(t, reasonsWith(t, &Resolver{}, dir, "admin/views/comments/"+page+".cfm"), map[string]string{
			variable + ".getText": "variable '" + variable + "' has no component ref",
		})
	}
}
