package resolve

import "testing"

// TestAMockBoxDecorationReturnsTheMock: `$()`, `$property()` and the other
// methods MockBox adds return the mock they are called on. A chain through
// them goes on from the mocked class, and an assignment through them holds
// the mock — dynamic when nothing here says what was mocked, as with
// ColdBox's BaseModelTest, whose `model` is createMock( annotations.model )
// in the base class.
func TestAMockBoxDecorationReturnsTheMock(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Svc.cfc":     `component { function go() {} function untyped() { return variables.x; } }`,
		"Factory.cfc": `component { Svc function make() { return new Svc(); } }`,
		"Spec.cfc": `component extends="vendor.missing.BaseModelTest" {
	function setup() {
		variables.iService = model.init( 1 ).$( "getCache", 2 ).$property( "x", "variables", 3 );
		variables.svc = createMock( "Svc" );
		variables.stubbed = svc.$( "go", 1 );
		variables.factory = new Factory();
		variables.made = factory.make().$( "go", 1 );
	}
	function f() {
		iService.anything();
		stubbed.go();
		stubbed.missing();
		svc.$( "go", 1 ).go();
		svc.$( "go", 1 ).missing();
		made.missing();
		svc.$( "go", 1 ).untyped().$( "a" );
		svc.untyped().go();
	}
}`,
	})

	expectReasons(t, reasonsIn(t, dir, "Spec.cfc"), map[string]string{
		"iService.anything": "",
		"stubbed.go":        "",
		"stubbed.missing":   "method 'missing' not found in Svc",
		"svc.$.go":          "",
		"svc.$.missing":     "method 'missing' not found in Svc",
		"made.missing":      "method 'missing' not found in Svc",
		// Only a mock has $(), so what untyped() returned is one.
		"svc.$.untyped.$": "",
		"svc.untyped.go":  "method 'untyped' in Svc has no component return type (chain to 'go')",
	})
}
