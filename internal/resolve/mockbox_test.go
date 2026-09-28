package resolve

import "testing"

// TestMockBoxDecorationsAreAcceptedOnAnyComponent: a test mocks a real
// component in place and then calls what MockBox added to it — $(),
// $results(), $never() — so the component's own methods are the wrong place
// to look for them. A $-name MockBox does not add is still checked.
func TestMockBoxDecorationsAreAcceptedOnAnyComponent(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Svc.cfc": `component { function go() {} }`,
		"Spec.cfc": `component {
	function run() {
		var s = new Svc();
		s.$( "go" ).$results( 1 );
		s.$never( "go" );
		s.$nope();
	}
}`,
	})

	expectReasons(t, reasonsIn(t, dir, "Spec.cfc"), map[string]string{
		"s.$.$results": "",
		"s.$never":     "",
		"s.$nope":      "method '$nope' not found in Svc",
	})
}
