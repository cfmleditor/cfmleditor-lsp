package resolve

import "testing"

// TestAWheelsModelsFindersAndAssociations: wheels.Model declares its finders
// `any`, since they return whichever model they are called on — findByKey()
// on User is a User. An association in config() adds methods named after it
// (Wheels' hasMany, belongsTo and hasOne docs): author() is the Author model,
// newComment() a Comment, commentCount() and hasComments() values. None of
// them is written in the model, and a name the associations do not give is
// still not a method of it. The fixture's own wheels.Model has no
// onMissingMethod, which in the real one accepts any call and would hide a
// wrong type.
func TestAWheelsModelsFindersAndAssociations(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"wheels/Model.cfc": `component { any function findByKey( key ) {} any function new() {} }`,
		"app/models/Post.cfc": `component extends="wheels.Model" {
	function config() {
		belongsTo( "author" );
		hasMany( name = "comments" );
		hasOne( "summary" );
	}
}`,
		"app/models/Author.cfc":  `component extends="wheels.Model" { function fullName() {} }`,
		"app/models/Comment.cfc": `component extends="wheels.Model" { function approve() {} }`,
		"app/models/Summary.cfc": `component extends="wheels.Model" { function text() {} }`,
		"app/controllers/Posts.cfc": `component {
	function show() {
		var post = new app.models.Post();
		post.findByKey( 1 ).author().fullName();
		post.findByKey( 1 ).author().notAnAuthorMethod();
		post.newComment().approve();
		post.commentCount();
		post.hasComments();
		post.summary().text();
		post.notAnAssociation();
	}
}`,
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "app/controllers/Posts.cfc"), map[string]string{
		"post.findByKey.author.fullName":          "",
		"post.findByKey.author.notAnAuthorMethod": "method 'notAnAuthorMethod' not found in Author",
		"post.notAnAssociation":                   "method 'notAnAssociation' not found in app.models.Post",
		"post.newComment.approve":                 "",
		"post.commentCount":                       "",
		"post.hasComments":                        "",
		"post.summary.text":                       "",
	})
}
