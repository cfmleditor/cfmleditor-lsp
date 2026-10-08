package server

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cfmleditor/clif/internal/config"
)

// TestExportCFLintLintsTheFoldersOpen lays out a workspace like ~/tassdev: a
// root config listing the repos, and a tassweb config listing its siblings for
// resolution. exportCFLint linted every workspace folder, so a tassweb window
// linted prs too and kept only tassweb's findings.
func TestExportCFLintLintsTheFoldersOpen(t *testing.T) {
	ws, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	tassweb := filepath.Join(ws, "tassweb")
	prs := filepath.Join(ws, "prs")

	for _, d := range []string{tassweb, prs} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	write(t, filepath.Join(ws, ".cfmleditor.json"), `{"workspaceName":"tass","workspacePaths":["tassweb","prs"]}`)
	write(t, filepath.Join(tassweb, ".cfmleditor.json"), `{"workspaceName":"tass","workspacePaths":["../prs","../tassweb"]}`)
	write(t, filepath.Join(prs, "a.cfm"), "")

	session := func(open string, folders ...string) *Server {
		srv := newTestServer()
		srv.addWorkspaceRoot(open)
		srv.WorkspaceFolders = folders

		return srv
	}

	t.Run("a project whose config lists its siblings", func(t *testing.T) {
		srv := session(tassweb, prs, tassweb)

		roots, targets, err := srv.exportScope(config.GenerateCFLint)
		if err != nil {
			t.Fatal(err)
		}

		if want := []string{tassweb}; !slices.Equal(roots, want) {
			t.Errorf("linted %v, want %v", roots, want)
		}

		if want := []string{filepath.Join(tassweb, ".clif-cflint.txt")}; !slices.Equal(targets, want) {
			t.Errorf("writes %v, want %v", targets, want)
		}

		// The unresolved report still reads every folder: resolution needs them.
		if roots, _, _ := srv.exportScope(config.GenerateUnresolved); !slices.Equal(roots, []string{prs, tassweb}) {
			t.Errorf("unresolved scans %v, want every workspace folder", roots)
		}
	})

	t.Run("the workspace root", func(t *testing.T) {
		roots, targets, err := session(ws, tassweb, prs).exportScope(config.GenerateCFLint)
		if err != nil {
			t.Fatal(err)
		}

		if want := []string{tassweb, prs}; !slices.Equal(roots, want) {
			t.Errorf("linted %v, want %v", roots, want)
		}

		if want := []string{filepath.Join(ws, ".clif-cflint.txt")}; !slices.Equal(targets, want) {
			t.Errorf("writes %v, want %v", targets, want)
		}
	})

	t.Run("a project with no config of its own", func(t *testing.T) {
		// prs is governed by the workspace config, whose report covers tassweb
		// too: rewriting it from a prs lint would delete tassweb's entries.
		_, _, err := session(prs, tassweb, prs).exportScope(config.GenerateCFLint)
		if err == nil || !strings.Contains(err.Error(), filepath.Join(ws, ".clif-cflint.txt")) {
			t.Errorf("err = %v, want a refusal naming the workspace report", err)
		}
	})
}
