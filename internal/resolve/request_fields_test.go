package resolve

import (
	"path/filepath"
	"testing"

	"github.com/cfmleditor/clif/internal/index"
	"github.com/cfmleditor/clif/internal/vfs"
)

// TestALazyRequestFieldIsWhatItsOnlyWriterStores: Mura's getCurrentUser()
// creates request.currentUser when it is absent and returns it, and nothing
// else writes that key, so it returns the facade it creates. A second writer
// anywhere, or a scan that does not know every file, withholds the answer.
func TestALazyRequestFieldIsWhatItsOnlyWriterStores(t *testing.T) {
	files := map[string]string{
		"Facade.cfc": `component { function init(){ return this; } }`,
		"Obj.cfc": `component {
	public function getCurrentUser() {
		if ( !structKeyExists(request,"currentUser") ) {
			request.currentUser=createObject("component","Facade").init();
		}
		return request.currentUser;
	}
}`,
	}

	returns := func(files map[string]string, batch bool) string {
		dir := t.TempDir()
		writeFiles(t, dir, files)

		r := &Resolver{FS: vfs.OS{}, Index: index.New(), WorkspaceFolders: []string{dir}}
		if batch {
			r.InferArgsFiles = cfmlFilesIn(t, dir)
		}

		return r.ReturnComponentOf(r.ResolveFunc(filepath.Join(dir, "Obj.cfc"), "getCurrentUser", dir))
	}

	if got := returns(files, true); filepath.Base(got) != "Facade.cfc" {
		t.Errorf("only writer: %q, want Facade.cfc", got)
	}

	if got := returns(files, false); got != "" {
		t.Errorf("outside a batch scan: %q, want nothing", got)
	}

	files["page.cfm"] = `<cfset request.currentUser = "guest">`
	if got := returns(files, true); got != "" {
		t.Errorf("a second writer: %q, want nothing", got)
	}
}
