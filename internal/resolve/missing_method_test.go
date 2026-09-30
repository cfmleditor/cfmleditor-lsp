package resolve

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

// TestAThisCallReachesOnMissingMethod: cborm's services call their dynamic
// finders through this — `this.findBySlug( slug )` — and BaseORMService
// answers them in onMissingMethod. CFML hands a method missing on the object
// to onMissingMethod; an unscoped call is a function lookup, which it never
// answers, and a component with no onMissingMethod answers neither.
func TestAThisCallReachesOnMissingMethod(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"orm/Base.cfc":  `component { function onMissingMethod( name, args ) {} }`,
		"orm/Plain.cfc": `component { function own() {} }`,
		"Service.cfc": `component extends="orm.Base" {
	function f() {
		var x = this.findBySlug( "a" );
		var y = findByName( "b" );
		this.countWhere ( a = 1 );
	}
}`,
		"Other.cfc": `component extends="orm.Plain" {
	function f() {
		this.findBySlug( "a" );
	}
}`,
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "Service.cfc"), map[string]string{
		"findBySlug": "",
		"findByName": "not found in extends chain",
		"countWhere": "",
	})
	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "Other.cfc"), map[string]string{
		"findBySlug": "not found in extends chain",
	})
}

// TestAFluentBaseMethodReturnsTheSubclass: cborm's BaseBuilder declares
// `BaseBuilder function add()`, which returns this, and on a CriteriaBuilder
// that is the CriteriaBuilder — whose onMissingMethod answers isEq(). Read as
// the declared BaseBuilder, ContentBox's criteria calls were "not found". A method whose
// declared class is some other one keeps it.
func TestAFluentBaseMethodReturnsTheSubclass(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"crit/BaseBuilder.cfc": `component {
	BaseBuilder function add( c ) { return this; }
	Other function other() {}
}`,
		"crit/Other.cfc":           `component { function own() {} }`,
		"crit/CriteriaBuilder.cfc": `component extends="BaseBuilder" { function onMissingMethod( n, a ) { return this; } function list() {} }`,
		"Service.cfc": `component {
	function f() {
		var c = new crit.CriteriaBuilder();
		c.add( 1 ).isEq( "a", 1 );
		var d = c.add( 2 );
		d.list();
		c.other().isEq( "a", 1 );
	}
}`,
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "Service.cfc"), map[string]string{
		"c.add.isEq":   "",
		"d.list":       "",
		"c.other.isEq": "method 'isEq' not found in Other",
	})
}

// TestGeneratedAccessorsReturnWhatTheyHold: a setter CFML generates for a
// property returns the object, so ColdBox's REST handlers chain
// `event.getResponse().setError( true ).setStatusCode( 401 )` on Response's
// accessors; a getter returns the property, so it holds what the property
// was typed as — cborm's getWireBox() is the injector it was given. A getter
// of a property with no component returns nothing known.
func TestGeneratedAccessorsReturnWhatTheyHold(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Helper.cfc": `component { function help() {} }`,
		"Response.cfc": `component accessors="true" {
	property name="error" type="boolean";
	property name="statusCode";
	property name="helper" inject="Helper";
	function addMessage( m ) {}
}`,
		"Handler.cfc": `component {
	function f() {
		var r = new Response();
		r.setError( true ).setStatusCode( 401 ).addMessage( "x" );
		r.getError().addMessage( "x" );
		r.getHelper().help();
		r.getHelper().nope();
	}
}`,
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "Handler.cfc"), map[string]string{
		"r.setError.setStatusCode.addMessage": "",
		"r.getError.addMessage":               "method 'getError' in Response has no component return type (chain to 'addMessage')",
		"r.getHelper.help":                    "",
		"r.getHelper.nope":                    "method 'nope' not found in Helper",
	})
}

// TestThisCallsAreKnownWhereverTheyAreWritten: the parser records whether a
// call was written `this.f()` (CallSite.This), in every form a call is
// recorded, where the resolver used to search the call's source line for
// `this.f(` — which also accepted a bare f() on a line holding a this.f().
func TestThisCallsAreKnownWhereverTheyAreWritten(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"orm/Base.cfc": `component { function onMissingMethod( name, args ) {} }`,
		"Service.cfc": `component extends="orm.Base" {
	function f() {
		g( this.findA() );
		this.findF( findF() );
		return this.findB();
	}
}`,
		"TagService.cfc": `<cfcomponent extends="orm.Base">
	<cffunction name="f">
		<cfset this.findC()>
		<cfreturn this.findD()>
	</cffunction>
</cfcomponent>`,
	})

	got := reasonsWith(t, &Resolver{}, dir, "Service.cfc")
	expectReasons(t, got, map[string]string{
		"findA": "",
		"findB": "",
	})

	// Two calls named findF: the this. one is answered, the bare one is not.
	if n := countReasons(t, dir, "Service.cfc", "findF"); n != 1 {
		t.Errorf("findF: %d unanswered calls, want 1 (the bare one)", n)
	}

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "TagService.cfc"), map[string]string{
		"findC": "",
		"findD": "",
	})
}

// countReasons is how many calls named name in page are reported, where
// reasonsIn keeps one answer per name.
func countReasons(t *testing.T, dir, page, name string) int {
	t.Helper()

	r := &Resolver{FS: vfs.OS{}, WorkspaceFolders: []string{dir}, Index: index.New()}
	file := filepath.Join(dir, filepath.FromSlash(page))

	err := filepath.WalkDir(dir, func(p string, _ os.DirEntry, err error) error {
		if err != nil || !strings.HasSuffix(p, ".cfc") {
			return err
		}

		data, err := os.ReadFile(p)
		if err == nil {
			r.Index.IndexFile(cfpath.ToURI(p), string(data))
		}

		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}

	pr := parser.ParseWithOptions(cfpath.ToURI(file), string(data), &parser.ParseOptions{ExtractCalls: true})
	calls := pr.AllCalls()
	n := 0

	for i := range calls {
		if c := &calls[i]; c.FuncName == name && r.CanResolveCall(c, pr, filepath.Dir(file)) != "" {
			n++
		}
	}

	return n
}
