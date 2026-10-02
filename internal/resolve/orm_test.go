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

func TestEntityLoadFunctionsFindAnEntityByItsEntityName(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"models/Author.cfc": `component persistent="true" entityname="cbAuthor" {
	property name="name";
}`,
		"services/Page.cfc": `component {
	function f() {
		var loaded = entityLoad( "cbAuthor", 1, true );
		loaded.getName();
		var byPK = entityLoadByPK( "cbAuthor", 1 );
		byPK.getName();
	}
}`,
		"services/TagPage.cfc": `<cfcomponent>
	<cffunction name="f">
		<cfset loaded = entityLoad("cbAuthor", 1, true)>
		<cfset loaded.getName()>
		<cfset byPK = entityLoadByPK("cbAuthor", 1)>
		<cfset byPK.getName()>
	</cffunction>
</cfcomponent>`,
	})

	for _, page := range []string{"services/Page.cfc", "services/TagPage.cfc"} {
		expectReasons(t, reasonsWith(t, &Resolver{}, dir, page), map[string]string{
			"loaded.getName": "",
			"byPK.getName":   "",
		})
	}
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

// TestAServiceBoundThroughItsInitArgumentDefault: ContentBox's ContentService
// declares `init( entityName = "cbContent" )` and passes it on as
// `super.init( entityName = arguments.entityName )`, so the service is bound
// to the default unless a subclass passes its own, as EntryService does.
func TestAServiceBoundThroughItsInitArgumentDefault(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"models/Content.cfc": `component persistent="true" entityname="cbContent" { property name="title"; }`,
		"models/Entry.cfc":   `component persistent="true" entityname="cbEntry" extends="Content" { property name="excerpt"; }`,
		"models/ContentService.cfc": `component extends="cborm.models.VirtualEntityService" singleton {
	ContentService function init( entityName = "cbContent" ) {
		super.init( entityName = arguments.entityName, useQueryCaching = true );
		return this;
	}
}`,
		"models/EntryService.cfc": `component extends="ContentService" singleton {
	EntryService function init() {
		super.init( entityName = "cbEntry", useQueryCaching = true );
		return this;
	}
}`,
		"models/TagService.cfc": `<cfcomponent extends="cborm.models.VirtualEntityService">
	<cffunction name="init">
		<cfargument name="entityName" default="cbContent">
		<cfset super.init( entityName = arguments.entityName )>
		<cfreturn this>
	</cffunction>
</cfcomponent>`,
		"models/OpenService.cfc": `component extends="cborm.models.VirtualEntityService" singleton {
	function init( required entityName ) {
		super.init( entityName = arguments.entityName );
		return this;
	}
}`,
		"handlers/Content.cfc": `component {
	property name="contentService" inject="ContentService";
	property name="entryService" inject="EntryService";
	property name="tagService" inject="TagService";
	property name="openService" inject="OpenService";
	function show( event, rc, prc ) {
		prc.content = contentService.get( 1 );
		prc.content.getTitle();
		prc.content.notAMethod();
		entryService.get( 1 ).getExcerpt();
		tagService.get( 1 ).notAMethod();
		openService.get( 1 ).anything();
	}
}`,
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "handlers/Content.cfc"), map[string]string{
		"prc.content.getTitle":        "",
		"prc.content.notAMethod":      "method 'notAMethod' not found in cbContent",
		"entryService.get.getExcerpt": "",
		"tagService.get.notAMethod":   "method 'notAMethod' not found in cbContent",
		"openService.get.anything":    "method 'get' in OpenService has no component return type (chain to 'anything')",
	})
}
