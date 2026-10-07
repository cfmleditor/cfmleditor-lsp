package resolve

import "testing"

// TestAConstructorArgumentIsWhatEveryConstructionPasses: cfwheels' CLI builds
// `new services.Templates( helpers = new util.Helpers() )`, and Templates
// keeps the argument as variables.helpers; ColdBox's BoxLangProvider makes its
// stats with `new Stats( this )`. A component also made where its argument
// cannot be read (getInstance( "Opaque" )) stays untyped.
func TestAConstructorArgumentIsWhatEveryConstructionPasses(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"util/Helpers.cfc": `component { function pluralize( s ){ return s; } }`,
		"services/Templates.cfc": `component {
	function init( required any helpers ){
		variables.helpers = arguments.helpers;
		return this;
	}
	function run(){
		variables.helpers.pluralize( "x" );
		variables.helpers.nope();
	}
}`,
		"services/Stats.cfc": `component {
	function init( required provider ){
		variables.provider = arguments.provider;
		return this;
	}
	function ratio(){ return variables.provider.getHits(); }
}`,
		"services/Provider.cfc": `component {
	function getHits(){ return 1; }
	function stats(){ return new Stats( this ); }
}`,
		"services/Opaque.cfc": `component {
	function init( required helpers ){
		variables.helpers = arguments.helpers;
		return this;
	}
	function run(){ variables.helpers.pluralize( "x" ); }
}`,
		"Module.cfc": `component {
	function setup(){
		variables.t = new services.Templates( helpers = new util.Helpers() );
		variables.o = new services.Opaque( helpers = new util.Helpers() );
		variables.o2 = wirebox.getInstance( "Opaque" );
	}
}`,
	})

	r := &Resolver{InferArgsFiles: cfmlFilesIn(t, dir)}

	expectReasons(t, reasonsWith(t, r, dir, "services/Templates.cfc"), map[string]string{
		"variables.helpers.pluralize": "",
		"variables.helpers.nope":      "method 'nope' not found in Helpers",
	})
	expectReasons(t, reasonsWith(t, r, dir, "services/Stats.cfc"), map[string]string{
		"variables.provider.getHits": "",
	})
	expectReasons(t, reasonsWith(t, r, dir, "services/Opaque.cfc"), map[string]string{
		"variables.helpers.pluralize": "variable 'variables.helpers' has no component ref",
	})
}

// TestAGeneratedGetterReturnsTheInitArgumentItsSetterStored: ColdBox's
// BoxLangStats keeps its provider through accessors, `setCacheProvider(
// arguments.cacheProvider )` in init, and reads it as getCacheProvider().
// A setter called anywhere else leaves the getter untyped.
func TestAGeneratedGetterReturnsTheInitArgumentItsSetterStored(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Stats.cfc": `component accessors="true" {
	property name="cacheProvider";
	function init( required cacheProvider ){
		setCacheProvider( arguments.cacheProvider );
		return this;
	}
	function ratio(){ return getCacheProvider().getHits(); }
}`,
		"Loose.cfc": `component accessors="true" {
	property name="cacheProvider";
	function init( required cacheProvider ){
		setCacheProvider( arguments.cacheProvider );
		return this;
	}
	function swap( p ){ setCacheProvider( p ); }
	function ratio(){ return getCacheProvider().getHits(); }
}`,
		"Provider.cfc": `component {
	function getHits(){ return 1; }
	function stats(){ return new Stats( this ); }
	function loose(){ return new Loose( this ); }
}`,
		"lib/Hits.cfc": `component { function getHits(){ return 1; } }`,
		"Typed.cfc": `component accessors="true" {
	property name="cacheProvider";
	function init( required lib.Hits cacheProvider ){
		setCacheProvider( arguments.cacheProvider );
		return this;
	}
	function ratio(){ return getCacheProvider().getHits(); }
}`,
	})

	r := &Resolver{InferArgsFiles: cfmlFilesIn(t, dir)}

	expectReasons(t, reasonsWith(t, r, dir, "Stats.cfc"), map[string]string{
		"getCacheProvider.getHits": "",
	})
	// Nothing constructs Typed; its argument's declared type answers.
	expectReasons(t, reasonsWith(t, r, dir, "Typed.cfc"), map[string]string{
		"getCacheProvider.getHits": "",
	})
	expectReasons(t, reasonsWith(t, r, dir, "Loose.cfc"), map[string]string{
		"getCacheProvider.getHits": "method 'getCacheProvider' has no component return type (chain to 'getHits')",
	})
}
