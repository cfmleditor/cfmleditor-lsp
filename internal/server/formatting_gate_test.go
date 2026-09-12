package server

import (
	"context"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func runFormatCommand(t *testing.T, srv *Server, docURI uri.URI) error {
	t.Helper()

	req := makeCall(t, protocol.MethodWorkspaceExecuteCommand, protocol.ExecuteCommandParams{
		Command:   "cfmleditor.format",
		Arguments: lspAnyArgs(string(docURI)),
	})

	_, err := srv.handleExecuteCommand(context.Background(), req)

	return err
}

// TestFormatCommandGatesOnEnabled pins the gate cfmleditor.format was missing.
// It hands s.Formatting straight to the formatter, and an unconfigured one is
// the zero value — every flag false, including WhitespaceOnly, the guard that
// stops the formatter writing back a file whose non-whitespace content it
// changed. textDocument/formatting has always returned early here; this
// command did not, which is what made a zero-valued s.Formatting dangerous
// rather than merely inert.
//
// The content below is one the guard rejects, so reaching the formatter at all
// surfaces as an error: with the gate, the command declines and there is none.
func TestFormatCommandGatesOnEnabled(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///t.cfm")
	srv.setDocument(docURI, "<CFOUTPUT>   <CFSET a=1>   </CFOUTPUT>\n")

	srv.Formatting = config.ResolvedFormatting{WhitespaceOnly: true}

	if err := runFormatCommand(t, srv, docURI); err != nil {
		t.Errorf("the command formatted with formatting disabled: %v", err)
	}
}

// TestFormatCommandStillRunsWhenEnabled is the other side of the gate.
func TestFormatCommandStillRunsWhenEnabled(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///t.cfm")
	srv.setDocument(docURI, "<cfoutput>\n<cfset a = 1 />\n</cfoutput>\n")

	srv.Formatting = config.DefaultResolvedFormatting()
	srv.Formatting.Enabled = true

	if err := runFormatCommand(t, srv, docURI); err != nil {
		t.Errorf("cfmleditor.format with formatting enabled: %v", err)
	}
}
