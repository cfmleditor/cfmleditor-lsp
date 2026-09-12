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

func runFindRefs(t *testing.T, srv *Server, args ...any) {
	t.Helper()

	req := makeCall(t, protocol.MethodWorkspaceExecuteCommand, protocol.ExecuteCommandParams{
		Command:   "cfmleditor.findRefs",
		Arguments: lspAnyArgs(args...),
	})

	if _, err := srv.handleExecuteCommand(context.Background(), req); err != nil {
		t.Fatalf("cfmleditor.findRefs: %v", err)
	}
}

func reportFiles(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	var out []string

	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "refs-") {
			out = append(out, e.Name())
		}
	}

	return out
}

// TestFindRefsDoesNotWriteFilesByDefault is the fix. The command used to write
// its report unconditionally, and the caller that fires most often is a code
// action on an ordinary editor gesture — so asking "find all references" left
// two files beside the file being read, inside the user's source tree.
func TestFindRefsDoesNotWriteFilesByDefault(t *testing.T) {
	srv, dir, docURI := refsWorkspace(t, "controller.cfc")

	runFindRefs(t, srv, "GetReport", string(docURI))

	if got := reportFiles(t, dir); len(got) > 0 {
		t.Errorf("findRefs wrote %v without being asked to export", got)
	}
}

// TestFindRefsWritesFilesWhenAsked is the other half: the export still works,
// it just has to be asked for.
func TestFindRefsWritesFilesWhenAsked(t *testing.T) {
	srv, dir, docURI := refsWorkspace(t, "controller.cfc")

	runFindRefs(t, srv, "GetReport", string(docURI), true)

	got := reportFiles(t, dir)

	for _, want := range []string{"refs-GetReport.md", "refs-GetReport.dot"} {
		found := false

		for _, g := range got {
			if g == want {
				found = true

				break
			}
		}

		if !found {
			t.Errorf("export did not write %s (wrote %v)", want, got)
		}
	}

	data, err := os.ReadFile(filepath.Join(dir, "refs-GetReport.md"))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(data), "```mermaid") {
		t.Errorf("report is missing its mermaid graph:\n%s", data)
	}
}

// TestFindRefsExportFalseIsHonoured covers a caller that passes the argument
// and says no. The two-argument call every existing client sends is covered
// above; a third argument of false has to mean the same thing.
func TestFindRefsExportFalseIsHonoured(t *testing.T) {
	srv, dir, docURI := refsWorkspace(t, "controller.cfc")

	runFindRefs(t, srv, "GetReport", string(docURI), false)

	if got := reportFiles(t, dir); len(got) > 0 {
		t.Errorf("findRefs wrote %v for an explicit export:false", got)
	}
}

// TestCodeActionsSeparateFindingFromExporting pins which action carries the
// flag. The plain "find" actions must not, or the fix is undone by the caller
// that caused the problem in the first place.
func TestCodeActionsSeparateFindingFromExporting(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///test.cfc")
	srv.setDocument(docURI, "component {\n\tfunction f() {\n\t\tGetReport();\n\t}\n}\n")

	req := makeCall(t, protocol.MethodTextDocumentCodeAction, protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		Range:        protocol.Range{Start: protocol.Position{Line: 2, Character: 4}},
	})

	res, err := srv.handleCodeAction(context.Background(), req)
	if err != nil {
		t.Fatalf("handleCodeAction: %v", err)
	}

	actions, ok := res.([]protocol.CodeAction)
	if !ok {
		t.Fatalf("handleCodeAction returned %T", res)
	}

	var exports int

	for _, a := range actions {
		if a.Command.Command != "cfmleditor.findRefs" {
			continue
		}

		if argBool(a.Command.Arguments, 2) {
			exports++

			continue
		}

		if strings.Contains(strings.ToLower(a.Title), "export") {
			t.Errorf("%q offers an export but does not ask for one", a.Title)
		}
	}

	if exports != 1 {
		t.Errorf("want exactly one findRefs action that exports, got %d of %d actions", exports, len(actions))
	}
}
