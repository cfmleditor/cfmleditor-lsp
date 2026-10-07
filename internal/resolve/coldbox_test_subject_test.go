package resolve

import "testing"

// TestAColdBoxModelTestsModelIsItsAttributesClass: BaseModelTest mocks the
// class a test's model="…" attribute names into variables.model, and specs
// write `variables.pool = model.init( … )` in a beforeEach closure and read
// pool in an it(). A test that names no model gets nothing.
func TestAColdBoxModelTestsModelIsItsAttributesClass(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"coldbox/system/testing/BaseModelTest.cfc": `component {
	function setup(){ variables.model = mockBox.createMock( annotations.model ); }
}`,
		"models/Pool.cfc": `component { function init( name ){ return this; } function register(){} }`,
		"tests/PoolTest.cfc": `component extends="coldbox.system.testing.BaseModelTest" model="models.Pool" {
	function run(){
		describe( "pool", function(){
			beforeEach( function(){
				variables.pool = model.init( "x" );
			} );
			it( "registers", function(){
				pool.register();
				pool.nope();
				model.register();
			} );
		} );
	}
}`,
		"tests/BareTest.cfc": `component extends="coldbox.system.testing.BaseModelTest" {
	function run(){ model.register(); }
}`,
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "tests/PoolTest.cfc"), map[string]string{
		"pool.register":  "",
		"pool.nope":      "method 'nope' not found in Pool",
		"model.register": "",
	})
	// Without the attribute, model is the base's createMock(), which is $any.
	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "tests/BareTest.cfc"), map[string]string{
		"model.register": "",
	})
}
