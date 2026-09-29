package resolve

import "testing"

// TestAnEntityIsFoundByItsEntityName: ContentBox's Author.cfc declares
// `entityname="cbAuthor"`, and entityNew( "cbAuthor" ) names it;
// the corpus reported cbAuthor and cbEntry as components that do not exist. An
// entity without an entityname is still its file's name.
func TestAnEntityIsFoundByItsEntityName(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"models/Author.cfc": `component persistent="true" entityname="cbAuthor" table="cb_author" {
	property name="name";
}`,
		"models/Tag.cfc": `component persistent="true" { property name="label"; }`,
		"services/Page.cfc": `component {
	function f() {
		var a = entityNew( "cbAuthor" );
		a.getName();
		a.notAnAccessor();
		var t = entityNew( "Tag" );
		t.getLabel();
	}
}`,
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "services/Page.cfc"), map[string]string{
		"a.getName":       "",
		"a.notAnAccessor": "method 'notAnAccessor' not found in cbAuthor",
		"t.getLabel":      "",
	})
}

// TestAVirtualEntityServiceReturnsItsEntity: ContentBox's services extend
// cborm's VirtualEntityService and bind it with super.init( entityName ),
// after which new(), get() and findWhere() return that entity — declared
// `any` in cborm, so only the service's own source says which. newCriteria()
// is a CriteriaBuilder whatever the service: cborm documents it, and the
// bundled API declares it. 207 ContentBox entries were newCriteria() alone.
func TestAVirtualEntityServiceReturnsItsEntity(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"models/Author.cfc": `component persistent="true" entityname="cbAuthor" { property name="name"; }`,
		"models/AuthorService.cfc": `component extends="cborm.models.VirtualEntityService" singleton {
	function init() {
		super.init( entityName = "cbAuthor" );
		return this;
	}
	function build() {
		var c = newCriteria();
		c.isEq( "name", "x" ).list();
		return c;
	}
}`,
		"handlers/Authors.cfc": `component {
	property name="authorService" inject="AuthorService";
	function show() {
		var a = authorService.get( 1 );
		a.getName();
		a.notAnAccessor();
		authorService.new().getName();
		authorService.findWhere( { name : "x" } ).getName();
		authorService.newCriteria().isEq( "a", 1 ).list();
	}
}`,
	})

	r := &Resolver{}
	expectReasons(t, reasonsWith(t, r, dir, "handlers/Authors.cfc"), map[string]string{
		"a.getName":                           "",
		"a.notAnAccessor":                     "method 'notAnAccessor' not found in cbAuthor",
		"authorService.new.getName":           "",
		"authorService.findWhere.getName":     "",
		"authorService.newCriteria.isEq.list": "",
	})
	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "models/AuthorService.cfc"), map[string]string{
		"c.isEq.list": "",
	})
}
