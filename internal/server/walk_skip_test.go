package server

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"go.lsp.dev/uri"
)

// The server walks the workspace in three places — the startup index, a folder
// added mid-session, and scanWorkspace — and all three must skip what the code
// map skips: every dot-directory and the dependency folders.
//
// The startup index and the scan named .git, .svn, node_modules, target and
// vendor, and let every other dot-directory in; a git worktree under .claude/
// is a complete second copy of the codebase, so every component was indexed
// twice. indexRoot skipped nothing at all. A dot-named root is still walked:
// the rule is about what sits inside the workspace, not where it lives.
func TestWorkspaceWalksSkipWhatTheCodeMapSkips(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".hidden-root")

	write := func(rel string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(p, []byte("component { function f() {} }\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("models/Real.cfc")

	skipped := []string{
		".claude/worktrees/copy/models/Real.cfc",
		".cache/Stale.cfc",
		"node_modules/pkg/Dep.cfc",
		"bower_components/pkg/Dep.cfc",
		"vendor/Lib.cfc",
	}
	for _, rel := range skipped {
		write(rel)
	}

	want := []string{filepath.Join(root, "models", "Real.cfc")}

	srv := newTestServer()
	srv.WorkspaceFolders = []string{root}

	if got := srv.collectCFCFiles(root); !slices.Equal(got, want) {
		t.Errorf("startup index walked %v, want %v", got, want)
	}

	if got := srv.scanFiles(); !slices.Equal(got, want) {
		t.Errorf("scanWorkspace walked %v, want %v", got, want)
	}

	srv.indexRoot(root)

	for _, rel := range skipped {
		if srv.index.HasFile(uri.File(filepath.Join(root, rel))) {
			t.Errorf("indexRoot indexed %s", rel)
		}
	}

	if !srv.index.HasFile(uri.File(want[0])) {
		t.Errorf("indexRoot did not index %s", want[0])
	}
}
