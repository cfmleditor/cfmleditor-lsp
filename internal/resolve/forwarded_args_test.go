package resolve

import "testing"

// TestAForwardedArgumentIsWhatTheCallerHolds: Mura's contentRenderer sets
// `arguments.renderer = this` and calls its utility with
// `argumentCollection = arguments`, so the utility's renderer argument is the
// renderer. A forwarding caller whose own argument cannot be typed (an
// override handing its arguments to super) is skipped, as a caller that does
// not pass the argument is.
func TestAForwardedArgumentIsWhatTheCallerHolds(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Renderer.cfc": `component {
	variables.util = new Utility();
	function dspObject( object ){
		arguments.renderer = this;
		return variables.util.dspObject( argumentCollection = arguments );
	}
	function createHREF(){ return ""; }
}`,
		"Utility.cfc": `component {
	function dspObject( object, renderer ){
		arguments.renderer.createHREF();
		arguments.renderer.nope();
	}
}`,
		"SpyUtility.cfc": `component extends="Utility" {
	function dspObject( object, renderer ){
		return super.dspObject( argumentCollection = arguments );
	}
}`,
	})

	expectReasons(t, reasonsWith(t, &Resolver{InferArgsFiles: cfmlFilesIn(t, dir)}, dir, "Utility.cfc"), map[string]string{
		"arguments.renderer.createHREF": "",
		"arguments.renderer.nope":       "method 'nope' not found in Renderer",
	})
}
