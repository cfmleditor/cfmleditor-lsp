package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/language"
	"go.lsp.dev/protocol"
)

func TestExecuteCommandScanWorkspace(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "good.cfm"), []byte("<cfoutput>hello</cfoutput>"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "bad.cfm"), []byte("<cfoutput><cfif>unclosed"), 0o644)

	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}

	req := makeCall(t, protocol.MethodWorkspaceExecuteCommand, protocol.ExecuteCommandParams{
		Command: "cfmleditor.scanWorkspace",
	})

	if _, err := srv.handleExecuteCommand(context.Background(), req); err != nil {
		t.Fatal(err)
	}
}

func TestCollectErrorDiagnostics(t *testing.T) {
	src := []byte("<cfoutput><cfif>unclosed")

	tree := language.Parse(language.CFML, src, nil)
	defer tree.Close()

	if !tree.RootNode().HasError() {
		t.Skip("test source does not produce parse errors")
	}

	diags := collectErrorDiagnostics(tree.RootNode(), src)
	if len(diags) == 0 {
		t.Fatal("expected at least one diagnostic")
	}

	for _, d := range diags {
		if d.Severity != protocol.DiagnosticSeverityError {
			t.Errorf("expected error severity, got %v", d.Severity)
		}

		if src, _ := d.Source.Get(); src != "cfmleditor" {
			t.Errorf("expected source 'cfmleditor', got %q", src)
		}
	}
}
