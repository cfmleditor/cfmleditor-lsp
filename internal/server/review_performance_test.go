package server

import (
	"os"
	"path/filepath"
	"testing"

	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"go.lsp.dev/protocol"
)

func TestReviewUnrelatedJSONRetainsDiscovery(t *testing.T) {
	dir := t.TempDir()
	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}
	r := srv.getResolver()
	srv.ensureBeansLoaded()

	for _, name := range []string{"package-lock.json", ".vscode/settings.json", "report.json"} {
		srv.applyWatchedFileChanges([]protocol.FileEvent{fileEvent(filepath.Join(dir, name), protocol.FileChangeTypeChanged)})
		srv.invalidateSourceFile(filepath.Join(dir, name))
	}

	if srv.getResolver() != r || srv.beansResolver != r {
		t.Fatal("unrelated JSON discarded discovery")
	}
}

func TestReviewSavesRetainBeansAndRefreshDependencies(t *testing.T) {
	dir := t.TempDir()
	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}
	srv.BeanPaths = map[string]string{"services": filepath.Join(dir, "services")}

	srv.StartupFiles = []string{filepath.Join(dir, "start.cfm")}
	for name, body := range map[string]string{"Application.cfc": `component {include "mappings.cfm";}`, "mappings.cfm": `<cfset this.mappings["/app"]="./app">`, "start.cfm": `<cfinclude template="registration.cfm">`, "registration.cfm": `<cfset x=1>`, "views/page.cfm": `<cfset x=1>`, "views/Plain.cfc": `component {}`, "services/User.cfc": `component {}`} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
			t.Fatal(err)
		}

		writeCFC(t, filepath.Join(dir, name), body)
	}

	r := srv.getResolver()
	srv.ensureBeansLoaded()
	srv.invalidateSourceFile(filepath.Join(dir, "views", "page.cfm"))
	srv.invalidateSourceFile(filepath.Join(dir, "views", "Plain.cfc"))
	srv.ensureBeansLoaded()

	if srv.getResolver() != r || srv.beansResolver != r {
		t.Fatal("ordinary save rebuilt beans/discovery")
	}

	for _, name := range []string{"registration.cfm", "mappings.cfm", "services/User.cfc", "Application.cfc"} {
		before := srv.getResolver()
		srv.invalidateSourceFile(filepath.Join(dir, name))

		if before == srv.getResolver() {
			t.Fatalf("%s did not invalidate discovery", name)
		}
	}

	cfpath.InvalidateAppMappingsCache()
}

func TestReviewSelectedNestedCFConfigIsWatched(t *testing.T) {
	dir := t.TempDir()
	srv := newTestServer()

	srv.WorkspaceFolders = []string{dir}
	if err := os.MkdirAll(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}

	writeCFC(t, filepath.Join(dir, "nested", "server.json"), `{"cfconfig":{"file":"config/selected.json"}}`)

	selected := filepath.Join(dir, "nested", "config", "selected.json")
	if !srv.isMappingJSON(selected) || srv.isMappingJSON(filepath.Join(dir, "nested", "config", "report.json")) {
		t.Fatal("wrong selected config filtering")
	}

	before := srv.getResolver()
	srv.applyWatchedFileChanges([]protocol.FileEvent{fileEvent(selected, protocol.FileChangeTypeDeleted)})

	if srv.getResolver() == before {
		t.Fatal("selected CFConfig deletion did not invalidate")
	}
}

func TestReviewMissingStartupIncludeRefreshesWhenCreated(t *testing.T) {
	dir := t.TempDir()
	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}
	srv.StartupFiles = []string{filepath.Join(dir, "start.cfm")}
	writeCFC(t, filepath.Join(dir, "start.cfm"), `<cfinclude template="future.cfm">`)

	before := srv.getResolver()
	srv.invalidateSourceFile(filepath.Join(dir, "view.cfm"))

	if before != srv.getResolver() {
		t.Fatal("unrelated source invalidated discovery")
	}

	writeCFC(t, filepath.Join(dir, "future.cfm"), `<cfset x=1>`)
	srv.invalidateSourceFile(filepath.Join(dir, "future.cfm"))

	if before == srv.getResolver() {
		t.Fatal("new startup include did not invalidate discovery")
	}
}
