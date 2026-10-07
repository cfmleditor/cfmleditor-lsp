package resolve

import (
	"path/filepath"
	"testing"
)

// TestAnAliasIsTypedAsTheNameItCopies: Masa's form builder writes
// `var mmRBF = application.rbFactory`, a value a startup template assigns and
// only a lookup can type. The alias is typed as the name is at the line.
func TestAnAliasIsTypedAsTheNameItCopies(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"startup.cfm":   `<cfset application.rbFactory = new RbFactory()>`,
		"RbFactory.cfc": `component { function getKeyValue( rb, key ){ return key; } }`,
		"Manager.cfc": `component {
	function label(){
		var mmRBF = application.rbFactory;
		mmRBF.getKeyValue( "en", "x" );
		mmRBF.nope();
	}
}`,
	})

	r := &Resolver{StartupFiles: []string{filepath.Join(dir, "startup.cfm")}}

	expectReasons(t, reasonsWith(t, r, dir, "Manager.cfc"), map[string]string{
		"mmRBF.getKeyValue": "",
		"mmRBF.nope":        "method 'nope' not found in RbFactory",
	})
}
