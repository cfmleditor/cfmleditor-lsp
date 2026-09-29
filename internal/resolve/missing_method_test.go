package resolve

import "testing"

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
