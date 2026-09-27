package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// filesUnder lists every file beneath dir whose name starts with prefix.
func filesUnder(t *testing.T, dir, prefix string) []string {
	t.Helper()

	var out []string

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && strings.HasPrefix(d.Name(), prefix) {
			out = append(out, path)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	return out
}

// A function name comes from the command's arguments, and the report path was
// built with filepath.Join, which cleans it: "x/../../escaped" made
// refs-x/../../escaped.md, which is escaped.md one directory above the source.
func TestFindRefsReportNameCannotEscape(t *testing.T) {
	srv, dir, docURI := refsWorkspace(t, "controller.cfc")

	// The workspace sits one level below a directory the test owns, so a file
	// written through "../" lands somewhere this test can see.
	parent := filepath.Dir(dir)

	runFindRefs(t, srv, "x/../../escaped", string(docURI), true)

	if got := filesUnder(t, parent, "refs-"); len(got) > 0 {
		t.Errorf("a report was written for a name holding ../: %v", got)
	}

	if got := filesUnder(t, parent, "escaped"); len(got) > 0 {
		t.Errorf("a report escaped the workspace: %v", got)
	}
}

// A document URI comes from the command's arguments too. One outside every
// workspace root put a deps report beside it, wherever it was.
func TestExportDepsWillNotWriteOutsideTheWorkspace(t *testing.T) {
	workspace := t.TempDir()
	outside := t.TempDir()

	page := filepath.Join(outside, "page.cfm")
	if err := os.WriteFile(page, []byte("<cfset x = 1>\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	srv := newTestServer()
	srv.WorkspaceFolders = []string{workspace}

	req := makeCall(t, protocol.MethodWorkspaceExecuteCommand, protocol.ExecuteCommandParams{
		Command:   "cfmleditor.exportDeps",
		Arguments: lspAnyArgs(string(uri.File(page))),
	})

	if _, err := srv.handleExecuteCommand(context.Background(), req); err != nil {
		t.Fatal(err)
	}

	if got := filesUnder(t, outside, "deps-"); len(got) > 0 {
		t.Errorf("exportDeps wrote outside the workspace: %v", got)
	}
}

func TestReportPath(t *testing.T) {
	root := t.TempDir()
	srv := newTestServer()
	srv.WorkspaceFolders = []string{root}

	cases := []struct {
		dir, base string
		ok        bool
	}{
		{root, "refs-getUser.md", true},
		{filepath.Join(root, "sub", "dir"), "deps-page.md", true},
		{root, "refs-x/../../x.md", false},
		{root, `refs-..\x.md`, false},
		{root, "..", false},
		{filepath.Dir(root), "refs-x.md", false},
		{filepath.Join(root, "..", "elsewhere"), "refs-x.md", false},
	}

	for _, c := range cases {
		_, err := srv.reportPath(c.dir, c.base)
		if (err == nil) != c.ok {
			t.Errorf("reportPath(%q, %q): error %v, want ok=%v", c.dir, c.base, err, c.ok)
		}
	}
}
