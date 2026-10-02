package resolve

import "testing"

// TestALoopVariableHoldsTheCollectionsElement: a loop over a persistent
// entity's collection relationship, read through its getter or its property,
// or over a cborm service's getAll(), gives its variable the entity. A
// subclass of the entity is an element too. Outside the loop body, over
// properties-only results and over a collection nothing types, the variable
// stays untyped.
func TestALoopVariableHoldsTheCollectionsElement(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"orm/VirtualEntityService.cfc": `component { function init( entityName ){ return this; } function getAll( id, sortOrder, properties ){} }`,
		"models/Comment.cfc":           `component persistent="true" entityname="cbComment" { function getText(){} }`,
		"models/ReplyComment.cfc":      `component persistent="true" extends="Comment" { function getParentText(){} }`,
		"models/CommentService.cfc":    `component extends="orm.VirtualEntityService" { function init(){ super.init( entityName = "cbComment" ); return this; } }`,
		"models/Post.cfc": `component persistent="true" {
property name="comments" fieldtype="one-to-many" cfc="Comment";
function scriptLoops( svc ){
	for ( var c in getComments() ) {
		c.getText();
		c.nope();
		c.getParentText();
	}
	for ( var p in variables.comments ) {
		p.getText();
	}
	var all = getComments();
	for ( var a in all ) {
		a.getText();
	}
	var service = new CommentService();
	for ( var s in service.getAll() ) {
		s.getText();
	}
	for ( var q in service.getAll( properties = "text" ) ) {
		q.getText();
	}
	for ( var u in unknownThings ) {
		u.getText();
	}
	after.getText();
}
}`,
		"views/list.cfm": `<cfset post = new models.Post()>
<cfloop array="#post.getComments()#" index="row">
	<cfoutput>#row.getText()#</cfoutput>
</cfloop>
<cfset row.later()>`,
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "models/Post.cfc"), map[string]string{
		"c.getText":       "",
		"c.nope":          "method 'nope' not found in Comment",
		"c.getParentText": "",
		"p.getText":       "",
		"a.getText":       "",
		"s.getText":       "",
		"q.getText":       "variable 'q' has no component ref",
		"u.getText":       "variable 'u' has no component ref",
		"after.getText":   "variable 'after' has no component ref",
	})
	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "views/list.cfm"), map[string]string{
		"row.getText": "",
		"row.later":   "variable 'row' has no component ref",
	})
}
