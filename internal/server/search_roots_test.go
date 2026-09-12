package server

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// configlessWorkspace is a session as an editor with no .cfmleditor.json gives
// us one: workspace roots reported at initialize, and WorkspaceFolders — which
// only ever comes from `workspacePaths` — empty.
func configlessWorkspace(t *testing.T) (*Server, string) {
	t.Helper()

	_, thisFile, _, _ := runtime.Caller(0)
	src := filepath.Join(filepath.Dir(thisFile), "..", "..", "testdata", "refs")

	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()

	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filepath.Join(dir, e.Name()), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	srv := newTestServer()
	srv.workspaceRoots = []string{dir}

	return srv, dir
}

// TestSearchRootsFallsBackToEditorRoots is the rule. WorkspaceFolders is the
// configured set and is empty without a config file, so a search that reads it
// directly covers nothing.
func TestSearchRootsFallsBackToEditorRoots(t *testing.T) {
	srv := newTestServer()
	srv.workspaceRoots = []string{"/editor/root"}

	if got := srv.searchRoots(); len(got) != 1 || got[0] != "/editor/root" {
		t.Errorf("with no configured folders, searchRoots = %v, want the editor's roots", got)
	}

	srv.WorkspaceFolders = []string{"/configured"}

	if got := srv.searchRoots(); len(got) != 1 || got[0] != "/configured" {
		t.Errorf("with configured folders, searchRoots = %v, want those", got)
	}
}

// TestFindRefsSearchesWithoutAConfig is the defect, at the command that showed
// it. Same workspace, same function, three callers sitting next to it —
// cfmleditor.findRefs answered "0 match(es)" purely because there was no
// .cfmleditor.json to populate WorkspaceFolders, which reads exactly like a
// function nothing calls.
func TestFindRefsSearchesWithoutAConfig(t *testing.T) {
	srv, dir := configlessWorkspace(t)

	docURI := uri.File(filepath.Join(dir, "controller.cfc"))

	req := makeCall(t, protocol.MethodWorkspaceExecuteCommand, protocol.ExecuteCommandParams{
		Command:   "cfmleditor.findRefs",
		Arguments: lspAnyArgs("GetReport", string(docURI)),
	})

	res, err := srv.handleExecuteCommand(context.Background(), req)
	if err != nil {
		t.Fatalf("cfmleditor.findRefs: %v", err)
	}

	summary, _ := res.(string)
	if strings.Contains(summary, "0 match(es)") || summary == "" {
		t.Errorf("findRefs found nothing without a config file: %q", summary)
	}
}

// TestScanWorkspaceScansWithoutAConfig is the same for the other command that
// walks the workspace itself.
func TestScanWorkspaceScansWithoutAConfig(t *testing.T) {
	srv, _ := configlessWorkspace(t)

	if got := len(srv.scanFiles()); got == 0 {
		t.Error("scanWorkspace would have scanned no files without a config file")
	}
}
