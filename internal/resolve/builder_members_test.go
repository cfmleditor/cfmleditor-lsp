package resolve

import "testing"

// TestACriteriaBuildersClosureAndRestrictionsAreTyped: cborm's when() hands its
// closure the current builder, and a builder's restrictions is a Restrictions.
// The closure rule needs the closure to be when()'s argument on a builder chain.
func TestACriteriaBuildersClosureAndRestrictionsAreTyped(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"svc/Other.cfc": `component { function run(){} }`,
		"svc/Svc.cfc": `component extends="cborm.models.VirtualEntityService" {
function init(){ super.init( entityName = "x" ); return this; }
function onChain(){
	return newCriteria().isTrue( "p" ).when( true, function( c ){
		c.isEq( "a", 1 );
		c.restrictions.like( "a", "b" );
	} );
}
function onLocal(){
	var q = newCriteria();
	q.when( true, function( e ){ e.isEq( "a", 1 ); } );
}
function notABuilder(){
	return thing().when( true, function( d ){ d.isEq( "a", 1 ); } );
}
function notWhen(){
	return newCriteria().list( function( f ){ f.isEq( "a", 1 ); } );
}
function brokenChain(){
	return newCriteria().list().when( true, function( g ){ g.isEq( "a", 1 ); } );
}
function plainRestrictions( x ){
	x.restrictions.like( "a", "b" );
}
function typedButNotABuilder(){
	var o = new svc.Other();
	o.restrictions.like( "a", "b" );
}
}`,
	})

	got := reasonsWith(t, &Resolver{}, dir, "svc/Svc.cfc")

	for _, k := range []string{"c.isEq", "c.restrictions.like", "e.isEq"} {
		if got[k] != "" {
			t.Errorf("%s: %q, want it resolved", k, got[k])
		}
	}

	for k, want := range map[string]string{
		"d.isEq":              "variable 'd' has no component ref",
		"f.isEq":              "variable 'f' has no component ref",
		"g.isEq":              "variable 'g' has no component ref",
		"x.restrictions.like": "variable 'x.restrictions' has no component ref",
		"o.restrictions.like": "variable 'o.restrictions' has no component ref",
	} {
		if got[k] != want {
			t.Errorf("%s: %q, want %q", k, got[k], want)
		}
	}
}
