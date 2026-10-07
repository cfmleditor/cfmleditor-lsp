package resolve

import (
	"path/filepath"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

// TestEachNestedFW1AppHasItsOwnDI1Beans: FW/1's examples are applications side
// by side, each extending framework.one with DI/1 over its own model and
// controllers, and each with a model/services/user.cfc. A controller's
// `property userService;` is its own application's service, found though no
// Application.cfc sits at the workspace root and though a workspace bean map
// can name only one user.cfc.
func TestEachNestedFW1AppHasItsOwnDI1Beans(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{}

	for _, app := range []string{"appA", "appB"} {
		files[app+"/Application.cfc"] = `component extends="framework.one" { variables.framework = { trace = true }; }`
		files[app+"/model/services/user.cfc"] = `component { function get(){} }`
		files[app+"/controllers/main.cfc"] = `component accessors=true { property userService; }`
	}

	writeFiles(t, dir, files)

	r := &Resolver{FS: vfs.OS{}, Index: index.New(), WorkspaceFolders: []string{dir}}

	for _, app := range []string{"appA", "appB"} {
		lookup := r.InjectionPropertyLookup(filepath.Join(dir, app, "controllers", "main.cfc"))
		if lookup == nil {
			t.Fatalf("%s: no DI/1 policy", app)
		}

		want := filepath.Join(dir, app, "model", "services", "user.cfc")
		if got := lookup("userService", map[string]string{}); !cfpath.SamePath(got, want) {
			t.Errorf("%s: userService is %q, want %q", app, got, want)
		}
	}
}
