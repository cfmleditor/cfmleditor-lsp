package server

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// exportDeps could not trace past the first hop: deps asked the index for the
// next hop's calls and the index has never stored call sites. This drives the
// real command, not the deps package, so the wiring is covered too.
func TestExportDepsTracesTransitively(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	src := filepath.Join(filepath.Dir(thisFile), "..", "..", "testdata", "deps")

	dir := t.TempDir()

	for _, name := range []string{"controller.cfc", "service.cfc", "persist.cfc"} {
		data, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}

	controller := filepath.Join(dir, "controller.cfc")

	data, err := os.ReadFile(controller)
	if err != nil {
		t.Fatal(err)
	}

	docURI := uri.File(controller)

	open := makeCall(t, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: docURI, Text: string(data)},
	})

	if _, err := srv.handleDidOpen(context.Background(), open); err != nil {
		t.Fatal(err)
	}

	req := makeCall(t, protocol.MethodWorkspaceExecuteCommand, protocol.ExecuteCommandParams{
		Command:   "cfmleditor.exportDeps",
		Arguments: lspAnyArgs(string(docURI), "BuildReport"),
	})

	res, err := srv.handleExecuteCommand(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	mermaid, ok := res.(string)
	if !ok {
		t.Fatalf("expected a mermaid string, got %T", res)
	}

	t.Logf("graph:\n%s", mermaid)

	for _, want := range []string{"service.cfc", "persist.cfc"} {
		if !strings.Contains(mermaid, want) {
			t.Errorf("graph never reached %s:\n%s", want, mermaid)
		}
	}
}

// A function-local `var repo = new modern()` shadows a file-level
// `variables.repo = new legacy()`, and resolve.CanResolveCall already checks
// function-scoped refs first. The deps loader handed them over the other way
// round — file-level first — and componentForReceiver takes the first name
// match, so the graph followed the shadowed component.
func TestExportDepsPrefersFunctionLocalRefs(t *testing.T) {
	dir := t.TempDir()

	write := func(name, body string) {
		t.Helper()

		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("A.cfc", "component {\n    variables.b = new B();\n\n    public void function Go() {\n        VARIABLES.b.Run();\n    }\n}\n")
	write("B.cfc", "component {\n    variables.repo = new legacy();\n\n    public void function Run() {\n        var repo = new modern();\n        repo.Save();\n    }\n}\n")
	write("legacy.cfc", "component {\n    public void function Save() {}\n}\n")
	write("modern.cfc", "component {\n    public void function Save() {}\n}\n")

	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}

	aPath := filepath.Join(dir, "A.cfc")

	data, err := os.ReadFile(aPath)
	if err != nil {
		t.Fatal(err)
	}

	docURI := uri.File(aPath)

	open := makeCall(t, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: docURI, Text: string(data)},
	})

	if _, err := srv.handleDidOpen(context.Background(), open); err != nil {
		t.Fatal(err)
	}

	req := makeCall(t, protocol.MethodWorkspaceExecuteCommand, protocol.ExecuteCommandParams{
		Command:   "cfmleditor.exportDeps",
		Arguments: lspAnyArgs(string(docURI), "Go"),
	})

	res, err := srv.handleExecuteCommand(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	mermaid, ok := res.(string)
	if !ok {
		t.Fatalf("expected a mermaid string, got %T", res)
	}

	t.Logf("graph:\n%s", mermaid)

	if strings.Contains(mermaid, "legacy") {
		t.Errorf("graph followed the shadowed file-level ref:\n%s", mermaid)
	}

	if !strings.Contains(mermaid, "modern") {
		t.Errorf("graph did not follow the function-local ref:\n%s", mermaid)
	}
}

// exportDeps read its calls from the document's cached ParseResult, and an
// edit outside a function reparses that shallowly, dropping every call site
// inside one. The graph then fell back to the index's component refs, which
// know which components a file names but not which of their functions it
// calls, so a comment added above the component changed the graph. The graph
// must come from the text the editor holds. The added line moves the call
// sites down one, so the graphs are compared with their line numbers dropped.
func TestExportDepsAfterAnEditOutsideAFunction(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	src := filepath.Join(filepath.Dir(thisFile), "..", "..", "testdata", "deps")

	dir := t.TempDir()

	for _, name := range []string{"controller.cfc", "service.cfc", "persist.cfc"} {
		data, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}

	controller := filepath.Join(dir, "controller.cfc")

	data, err := os.ReadFile(controller)
	if err != nil {
		t.Fatal(err)
	}

	docURI := uri.File(controller)

	open := makeCall(t, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: docURI, Text: string(data)},
	})

	if _, err := srv.handleDidOpen(context.Background(), open); err != nil {
		t.Fatal(err)
	}

	forms := [][]any{{string(docURI), "BuildReport"}, {string(docURI)}}

	graphs := func() []string {
		var out []string

		for _, args := range forms {
			req := makeCall(t, protocol.MethodWorkspaceExecuteCommand, protocol.ExecuteCommandParams{
				Command:   "cfmleditor.exportDeps",
				Arguments: lspAnyArgs(args...),
			})

			res, err := srv.handleExecuteCommand(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}

			mermaid, _ := res.(string)
			out = append(out, mermaid)
		}

		return out
	}

	before := graphs()

	edit := makeCall(t, protocol.MethodTextDocumentDidChange, protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: docURI},
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			&protocol.TextDocumentContentChangePartial{Range: protocol.Range{}, Text: "// added\n"},
		},
	})

	if _, err := srv.handleDidChange(context.Background(), edit); err != nil {
		t.Fatal(err)
	}

	lineNo := regexp.MustCompile(`( #40;line \d+#41;|__line_\d+_)`)

	for i, after := range graphs() {
		if lineNo.ReplaceAllString(after, "") != lineNo.ReplaceAllString(before[i], "") {
			t.Errorf("exportDeps %v changed after a comment was added:\nbefore:\n%s\nafter:\n%s", forms[i][1:], before[i], after)
		}
	}
}
