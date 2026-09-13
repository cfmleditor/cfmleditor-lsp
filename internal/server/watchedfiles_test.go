package server

import (
	"os"
	"path/filepath"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func writeCFC(t *testing.T, path, body string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func fileEvent(path string, kind protocol.FileChangeType) protocol.FileEvent {
	return protocol.FileEvent{URI: uri.File(path), Type: kind}
}

// indexedNames reports the files the index holds a function of this name in.
func indexedCount(t *testing.T, s *Server, name string) int {
	t.Helper()

	return len(s.index.Lookup(name))
}

// TestWatchedFileCreateIsIndexed covers a component that did not exist when
// indexWorkspace ran — a branch switch that adds a file, or codegen output.
func TestWatchedFileCreateIsIndexed(t *testing.T) {
	dir := t.TempDir()

	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}

	path := filepath.Join(dir, "New.cfc")
	writeCFC(t, path, "component {\n\tfunction freshlyAdded() {}\n}\n")

	if got := indexedCount(t, srv, "freshlyAdded"); got != 0 {
		t.Fatalf("precondition: index already holds freshlyAdded (%d)", got)
	}

	srv.applyWatchedFileChanges([]protocol.FileEvent{fileEvent(path, protocol.FileChangeTypeCreated)})

	if got := indexedCount(t, srv, "freshlyAdded"); got != 1 {
		t.Errorf("after create event: freshlyAdded indexed %d times, want 1", got)
	}
}

// TestWatchedFileChangeReplacesTheOldEntries is the git-checkout case: the file
// on disk is not the file that was indexed at startup. The old function must go
// as well as the new one arriving — an index that only ever grows answers
// go-to-definition with a function that no longer exists.
func TestWatchedFileChangeReplacesTheOldEntries(t *testing.T) {
	dir := t.TempDir()

	path := filepath.Join(dir, "Svc.cfc")
	writeCFC(t, path, "component {\n\tfunction beforeCheckout() {}\n}\n")

	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}
	srv.indexWorkspace()

	if got := indexedCount(t, srv, "beforeCheckout"); got != 1 {
		t.Fatalf("precondition: beforeCheckout indexed %d times, want 1", got)
	}

	writeCFC(t, path, "component {\n\tfunction afterCheckout() {}\n}\n")
	srv.applyWatchedFileChanges([]protocol.FileEvent{fileEvent(path, protocol.FileChangeTypeChanged)})

	if got := indexedCount(t, srv, "afterCheckout"); got != 1 {
		t.Errorf("afterCheckout indexed %d times, want 1", got)
	}

	if got := indexedCount(t, srv, "beforeCheckout"); got != 0 {
		t.Errorf("beforeCheckout still indexed %d times after the file stopped declaring it", got)
	}
}

// TestWatchedFileDeleteRemovesFromIndex covers a file deleted on disk, and with
// it the ORM entity registration keyed by its name.
func TestWatchedFileDeleteRemovesFromIndex(t *testing.T) {
	dir := t.TempDir()

	writeCFC(t, filepath.Join(dir, "Application.cfc"), "component {\n\tthis.ormEnabled = true;\n}\n")

	path := filepath.Join(dir, "Gone.cfc")
	writeCFC(t, path, "component persistent=\"true\" {\n\tfunction willVanish() {}\n}\n")

	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}
	srv.indexWorkspace()

	if got := indexedCount(t, srv, "willVanish"); got != 1 {
		t.Fatalf("precondition: willVanish indexed %d times, want 1", got)
	}

	// Asserted rather than assumed: if the fixture stopped registering an
	// entity, the check below would pass for the wrong reason.
	if srv.index.LookupEntity("Gone") == "" {
		t.Fatal("precondition: Gone was not registered as an ORM entity")
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	srv.applyWatchedFileChanges([]protocol.FileEvent{fileEvent(path, protocol.FileChangeTypeDeleted)})

	if got := indexedCount(t, srv, "willVanish"); got != 0 {
		t.Errorf("willVanish still indexed %d times after its file was deleted", got)
	}

	if srv.index.LookupEntity("Gone") != "" {
		t.Error("ORM entity Gone still registered after its file was deleted")
	}
}

// TestWatchedFileChangeSkipsOpenDocuments: the editor's buffer outranks disk.
// A watcher event for an open file is either the editor's own save, which
// didSave already handled, or an external change the editor will reload and
// report itself — reading disk here can only file older content.
func TestWatchedFileChangeSkipsOpenDocuments(t *testing.T) {
	dir := t.TempDir()

	path := filepath.Join(dir, "Open.cfc")
	writeCFC(t, path, "component {\n\tfunction onDisk() {}\n}\n")

	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}

	docURI := uri.File(path)
	srv.setDocument(docURI, "component {\n\tfunction inTheBuffer() {}\n}\n")
	srv.reindexIfCFC(docURI, "component {\n\tfunction inTheBuffer() {}\n}\n")

	srv.applyWatchedFileChanges([]protocol.FileEvent{fileEvent(path, protocol.FileChangeTypeChanged)})

	if got := indexedCount(t, srv, "inTheBuffer"); got != 1 {
		t.Errorf("buffer function inTheBuffer indexed %d times, want 1 — disk overwrote the open buffer", got)
	}

	if got := indexedCount(t, srv, "onDisk"); got != 0 {
		t.Errorf("disk function onDisk indexed %d times, want 0 — an open document was read from disk", got)
	}
}

// TestWatchedFileDeleteAppliesToOpenDocuments is the deliberate exception to
// the rule above. Content is the editor's to report; existence is not. An entry
// pointing at a path that is gone resolves go-to-definition to nothing and has
// no way back, whereas re-saving the buffer re-indexes it through didSave.
func TestWatchedFileDeleteAppliesToOpenDocuments(t *testing.T) {
	dir := t.TempDir()

	path := filepath.Join(dir, "Doomed.cfc")
	src := "component {\n\tfunction stillOpen() {}\n}\n"
	writeCFC(t, path, src)

	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}

	docURI := uri.File(path)
	srv.setDocument(docURI, src)
	srv.reindexIfCFC(docURI, src)

	if got := indexedCount(t, srv, "stillOpen"); got != 1 {
		t.Fatalf("precondition: stillOpen indexed %d times, want 1", got)
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	srv.applyWatchedFileChanges([]protocol.FileEvent{fileEvent(path, protocol.FileChangeTypeDeleted)})

	if got := indexedCount(t, srv, "stillOpen"); got != 0 {
		t.Errorf("stillOpen indexed %d times after its file was deleted while open, want 0", got)
	}
}

// TestWatchedFileChangeIgnoresPathsOutsideTheIndexScope: a workspace that has
// narrowed indexing does not want a file indexed just because it changed.
func TestWatchedFileChangeIgnoresPathsOutsideTheIndexScope(t *testing.T) {
	dir := t.TempDir()

	inside := filepath.Join(dir, "in")
	outside := filepath.Join(dir, "out")

	for _, d := range []string{inside, outside} {
		if err := os.MkdirAll(d, 0o750); err != nil {
			t.Fatal(err)
		}
	}

	srv := newTestServer()
	srv.WorkspaceFolders = []string{inside}

	path := filepath.Join(outside, "Stray.cfc")
	writeCFC(t, path, "component {\n\tfunction outOfScope() {}\n}\n")

	srv.applyWatchedFileChanges([]protocol.FileEvent{fileEvent(path, protocol.FileChangeTypeCreated)})

	if got := indexedCount(t, srv, "outOfScope"); got != 0 {
		t.Errorf("outOfScope indexed %d times, want 0 — a file outside the workspace folders was indexed", got)
	}
}

// TestWatchedFileVanishedBeforeReadIsRemoved: a created-then-deleted file
// inside one batch, or one the watcher caught mid-write, must not leave the
// previous contents indexed as though they were current.
func TestWatchedFileVanishedBeforeReadIsRemoved(t *testing.T) {
	dir := t.TempDir()

	path := filepath.Join(dir, "Flicker.cfc")
	writeCFC(t, path, "component {\n\tfunction wasThere() {}\n}\n")

	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}
	srv.indexWorkspace()

	if got := indexedCount(t, srv, "wasThere"); got != 1 {
		t.Fatalf("precondition: wasThere indexed %d times, want 1", got)
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	// A *Changed* event, not a delete — the file is already gone by the time
	// the batch is applied.
	srv.applyWatchedFileChanges([]protocol.FileEvent{fileEvent(path, protocol.FileChangeTypeChanged)})

	if got := indexedCount(t, srv, "wasThere"); got != 0 {
		t.Errorf("wasThere indexed %d times after its file vanished, want 0", got)
	}
}

func TestClientWatchesFiles(t *testing.T) {
	yes, no := true, false

	cases := []struct {
		name string
		caps protocol.ClientCapabilities
		want bool
	}{
		{"no workspace capabilities", protocol.ClientCapabilities{}, false},
		{
			"workspace but no didChangeWatchedFiles",
			protocol.ClientCapabilities{Workspace: &protocol.WorkspaceClientCapabilities{}},
			false,
		},
		{
			"didChangeWatchedFiles with no dynamicRegistration",
			protocol.ClientCapabilities{Workspace: &protocol.WorkspaceClientCapabilities{
				DidChangeWatchedFiles: &protocol.DidChangeWatchedFilesClientCapabilities{},
			}},
			false,
		},
		{
			"dynamicRegistration false",
			protocol.ClientCapabilities{Workspace: &protocol.WorkspaceClientCapabilities{
				DidChangeWatchedFiles: &protocol.DidChangeWatchedFilesClientCapabilities{DynamicRegistration: &no},
			}},
			false,
		},
		{
			"dynamicRegistration true",
			protocol.ClientCapabilities{Workspace: &protocol.WorkspaceClientCapabilities{
				DidChangeWatchedFiles: &protocol.DidChangeWatchedFilesClientCapabilities{DynamicRegistration: &yes},
			}},
			true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := clientWatchesFiles(tc.caps); got != tc.want {
				t.Errorf("clientWatchesFiles = %v, want %v", got, tc.want)
			}
		})
	}
}
