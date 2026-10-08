package resolve

import (
	"path/filepath"
	"testing"

	"github.com/cfmleditor/clif/internal/index"
	cfpath "github.com/cfmleditor/clif/internal/path"
	"github.com/cfmleditor/clif/internal/vfs"
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

// TestFW1AutowiresItsControllersWhateverDILocationsSays: qBall sets
// diLocations = "./model/services", and FW/1 still injects its controllers
// from that bean factory. A property names a singleton, so of qBall's
// beans/question.cfc and services/question.cfc it is the service.
func TestFW1AutowiresItsControllersWhateverDILocationsSays(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Application.cfc":             `component extends="framework.one" { variables.framework = { diLocations = "./model/services,./model/beans" }; }`,
		"model/services/question.cfc": `component { function getQuestion(){} }`,
		"model/beans/question.cfc":    `component persistent="true" { property name="id"; }`,
		"controllers/question.cfc":    `component accessors=true { property question; }`,
		"views/main/default.cfm":      `<cfoutput></cfoutput>`,
	})

	r := &Resolver{FS: vfs.OS{}, Index: index.New(), WorkspaceFolders: []string{dir}}

	lookup := r.InjectionPropertyLookup(filepath.Join(dir, "controllers", "question.cfc"))
	if lookup == nil {
		t.Fatal("controllers are not injected")
	}

	want := filepath.Join(dir, "model", "services", "question.cfc")
	if got := lookup("question", map[string]string{}); !cfpath.SamePath(got, want) {
		t.Errorf("question is %q, want %q", got, want)
	}

	if r.InjectionPropertyLookup(filepath.Join(dir, "views", "main", "default.cfm")) != nil {
		t.Error("a view is not a component the factory injects")
	}
}
