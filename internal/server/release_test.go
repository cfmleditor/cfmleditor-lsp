package server

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// Formatting a 65,000-line component takes 1.5s, and handlers run inline on
// the read goroutine, so every message behind a format waited for it: hovers,
// keystrokes, cancellations. Formatting now releases the read loop once it has
// the text.
//
// The formatter is held on a channel rather than made slow, so this proves the
// ordering without depending on timing: while it is held, a didChange must be
// applied and a later request answered; and the format, released afterwards,
// must still answer for the text it was asked about.
func TestFormattingDoesNotHoldTheReadLoop(t *testing.T) {
	held, release := make(chan string, 1), make(chan struct{})

	orig := formatDocumentFn
	formatDocumentFn = func(content string, opts protocol.FormattingOptions, cfg *config.ResolvedFormatting) (string, error) {
		held <- content

		<-release

		return orig(content, opts, cfg)
	}

	t.Cleanup(func() { formatDocumentFn = orig })

	a, b := net.Pipe()
	ctx := context.Background()

	srvConn := jsonrpc2.NewConn(jsonrpc2.NewStream(a))
	cliConn := jsonrpc2.NewConn(jsonrpc2.NewStream(b))

	s := NewServer(srvConn, cflog.NewLogger(false))
	s.Formatting = config.DefaultResolvedFormatting()
	s.Formatting.Enabled = true

	docURI := uri.URI("file:///release_probe.cfc")
	before := "component {\nfunction a() {\nx = 1;\n}\n}\n"
	s.setDocument(docURI, before)

	srvConn.Go(ctx, s.Handler())
	cliConn.Go(ctx, func(context.Context, *jsonrpc2.Request) (any, error) { return nil, nil })

	type result struct {
		edits []protocol.TextEdit
		err   error
	}

	formatted := make(chan result, 1)

	go func() {
		var edits []protocol.TextEdit

		_, err := cliConn.Call(ctx, protocol.MethodTextDocumentFormatting, &protocol.DocumentFormattingParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
			Options:      protocol.FormattingOptions{TabSize: 4, InsertSpaces: true},
		}, &edits)
		formatted <- result{edits, err}
	}()

	select {
	case got := <-held:
		if got != before {
			t.Fatalf("formatter was given %q", got)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("formatting never reached the formatter")
	}

	// The formatter is held. A keystroke and a request sent now must get
	// through. Both are sent from one goroutine so a held read loop shows up
	// as a timeout here: the pipe is unbuffered, so even the send blocks.
	answered := make(chan error, 1)

	go func() {
		err := cliConn.Notify(ctx, protocol.MethodTextDocumentDidChange, &protocol.DidChangeTextDocumentParams{
			TextDocument: protocol.VersionedTextDocumentIdentifier{TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: docURI}},
			ContentChanges: []protocol.TextDocumentContentChangeEvent{
				&protocol.TextDocumentContentChangePartial{Range: protocol.Range{}, Text: "// edited\n"},
			},
		})
		if err == nil {
			var res any

			_, err = cliConn.Call(ctx, protocol.MethodTextDocumentFoldingRange, &protocol.FoldingRangeParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
			}, &res)
		}

		answered <- err
	}()

	select {
	case err := <-answered:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("a message sent during formatting was not handled until it finished: the read loop is still held")
	}

	if got, _ := s.getDocument(docURI); got != "// edited\n"+before {
		t.Fatalf("the didChange sent during formatting was not applied: %q", got)
	}

	close(release)

	select {
	case r := <-formatted:
		if r.err != nil {
			t.Fatal(r.err)
		}

		// The edits are for the text formatting was asked about, which is
		// what the editor applies them against.
		if got := applyTextEdits(t, before, r.edits); got == before {
			t.Errorf("formatting changed nothing: %+v", r.edits)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("formatting never answered")
	}
}
