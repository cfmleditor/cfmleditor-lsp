package server

import (
	"context"
	"slices"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// TestCodeActionsOfferTheWorkspaceReports covers the only way a client such as
// Zed, which cannot run a server command itself, reaches the report exports:
// they are offered with the cursor on a word and on blank space alike.
func TestCodeActionsOfferTheWorkspaceReports(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///test.cfc")
	srv.setDocument(docURI, "component {\n\n\tfunction f() {\n\t\tGetReport();\n\t}\n}\n")

	for _, pos := range []protocol.Position{{Line: 3, Character: 4}, {Line: 1, Character: 0}} {
		req := makeCall(t, protocol.MethodTextDocumentCodeAction, protocol.CodeActionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
			Range:        protocol.Range{Start: pos},
		})

		res, err := srv.handleCodeAction(context.Background(), req)
		if err != nil {
			t.Fatalf("handleCodeAction: %v", err)
		}

		actions, _ := res.([]protocol.CodeAction)

		var commands []string
		for _, a := range actions {
			commands = append(commands, a.Command.Command)
		}

		for _, want := range []string{"cfmleditor.exportUnresolved", "cfmleditor.exportCFLint"} {
			if !slices.Contains(commands, want) {
				t.Errorf("line %d: %s not offered; got %v", pos.Line, want, commands)
			}
		}
	}
}
