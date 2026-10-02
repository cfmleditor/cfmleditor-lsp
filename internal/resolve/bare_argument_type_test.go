package resolve

import "testing"

// TestABareArgumentTypeNamesAComponentBesideTheFile: `required Author a` is
// the Author beside the declaring file, whether the argument is read as
// arguments.a or a, and inside a closure the function holds. A CFML type
// name, a word naming no file there, and an untyped argument stay untyped.
func TestABareArgumentTypeNamesAComponentBesideTheFile(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Author.cfc": `component {function fetchProfile(){}}`,
		// A struct argument is a native struct, whatever sits beside it.
		"Struct.cfc": `component {function fetchProfile(){}}`,
		"Service.cfc": `component {
function scoped(required Author a){ arguments.a.fetchProfile(); arguments.a.nope(); }
function unscoped(Author b){ b.fetchProfile(); }
function inClosure(required Author c){ return [ 1 ].map( function( x ) { return c.fetchProfile(); } ); }
function cfmlType(required struct d){ d.fetchProfile(); }
function noFile(required Editor e){ e.fetchProfile(); }
function untyped(f){ f.fetchProfile(); }
}`,
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "Service.cfc"), map[string]string{
		"arguments.a.fetchProfile": "",
		"arguments.a.nope":         "method 'nope' not found in Author",
		"b.fetchProfile":           "",
		"c.fetchProfile":           "",
		"d.fetchProfile":           "variable 'd' has no component ref",
		"e.fetchProfile":           "variable 'e' has no component ref",
		"f.fetchProfile":           "variable 'f' has no component ref",
	})
}
