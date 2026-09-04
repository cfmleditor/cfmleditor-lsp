package server

import (
	"context"
	"net"
	"testing"
	"time"

	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// TestExecuteCommandFormatDoesNotWedgeConnection pins the fix for a deadlock
// that took the whole session down, not just one request.
//
// jsonrpc2 runs handlers inline on the read goroutine, so a handler that makes
// a server->client request and waits for the reply is waiting on a read only it
// could perform. cfmleditor.format does exactly that: it sends
// workspace/applyEdit and blocks. The edit reached the editor and was applied,
// so formatting looked like it worked — and then the server never answered
// anything again.
//
// The assertion is therefore not just that the command returns, but that a
// later request is still served afterwards, which is what actually broke.
func TestExecuteCommandFormatDoesNotWedgeConnection(t *testing.T) {
	t.Parallel()

	a, b := net.Pipe()
	ctx := context.Background()

	srvConn := jsonrpc2.NewConn(jsonrpc2.NewStream(a))
	cliConn := jsonrpc2.NewConn(jsonrpc2.NewStream(b))

	s := NewServer(srvConn, cflog.NewLogger(false))
	s.Formatting.Enabled = true

	docURI := uri.URI("file:///deadlock_probe.cfc")
	s.setDocument(docURI, "<cfcomponent>\n<cfset x=1>\n</cfcomponent>\n")

	srvConn.Go(ctx, s.Handler())

	// A client that answers workspace/applyEdit, as a real editor does. Without
	// the Async release this reply is never read.
	cliConn.Go(ctx, func(_ context.Context, req *jsonrpc2.Request) (any, error) {
		if req.Method() == protocol.MethodWorkspaceApplyEdit {
			return protocol.ApplyWorkspaceEditResult{Applied: true}, nil
		}

		return nil, nil
	})

	cmdDone := make(chan struct{})

	go func() {
		defer close(cmdDone)

		var res any

		_, _ = cliConn.Call(ctx, protocol.MethodWorkspaceExecuteCommand, &protocol.ExecuteCommandParams{
			Command:   "cfmleditor.format",
			Arguments: lspAnyArgs(string(docURI)),
		}, &res)
	}()

	select {
	case <-cmdDone:
	case <-time.After(10 * time.Second):
		t.Fatal("cfmleditor.format never returned: the handler is blocked on the client's workspace/applyEdit reply, which the read goroutine it occupies would have to deliver")
	}

	// The command returning is not enough — the connection has to still work.
	followUp := make(chan struct{})

	go func() {
		defer close(followUp)

		var res any

		_, _ = cliConn.Call(ctx, protocol.MethodTextDocumentDocumentSymbol, &protocol.DocumentSymbolParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		}, &res)
	}()

	select {
	case <-followUp:
	case <-time.After(10 * time.Second):
		t.Fatal("connection wedged: a request issued after cfmleditor.format was never answered")
	}
}
