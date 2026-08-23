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
