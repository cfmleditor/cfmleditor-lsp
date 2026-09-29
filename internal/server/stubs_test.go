package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
	"github.com/cfmleditor/cfmleditor-lsp/internal/frameworkapi"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

const stubHandler = `component {
	function index( event, rc, prc ) {
		event.getValue( "x" );
		event.
	}
}`

// stubServer opens a ColdBox handler in a workspace that has no ColdBox, with
// the coldbox preset named, so everything about event comes from the stubs.
func stubServer(t *testing.T) (*Server, uri.URI) {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "handlers", "Main.cfc")

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(stubHandler), 0o600); err != nil {
		t.Fatal(err)
	}

	s := newTestServer()
	s.Frameworks = []string{"coldbox"}
	s.ComponentResolvers = config.FrameworkResolvers(s.Frameworks)

	docURI := uri.File(path)

	open, err := json.Marshal(protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: docURI, LanguageID: "cfml", Version: 1, Text: stubHandler},
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.handleDidOpen(context.Background(), open); err != nil {
		t.Fatal(err)
	}

	return s, docURI
}

func stubPos(t *testing.T, line int, needle string, offset int) protocol.Position {
	t.Helper()

	text := strings.Split(stubHandler, "\n")[line]

	col := strings.Index(text, needle)
	if col < 0 {
		t.Fatalf("%q not on line %d", needle, line)
	}

	return protocol.Position{Line: uint32(line), Character: lineCol(text, col+offset)}
}

func stubParams(t *testing.T, docURI uri.URI, pos protocol.Position) []byte {
	t.Helper()

	b, err := json.Marshal(protocol.TextDocumentPositionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI}, Position: pos,
	})
	if err != nil {
		t.Fatal(err)
	}

	return b
}

// TestStubsAreDocumentedButNeverJumpedTo: with ColdBox not checked out, a
// method on event hovers with ColdBox's own doc comment, completes with it and
// shows it in signature help, and go-to-definition — which found the stub —
// answers nothing rather than send the editor to a file that does not exist.
func TestStubsAreDocumentedButNeverJumpedTo(t *testing.T) {
	s, docURI := stubServer(t)
	ctx := context.Background()
	onGetValue := stubParams(t, docURI, stubPos(t, 2, "getValue", 1))

	hover, err := s.handleHover(ctx, onGetValue)
	if err != nil || hover == nil {
		t.Fatalf("no hover: %v", err)
	}

	h, _ := hover.(*protocol.Hover)
	if h == nil {
		t.Fatalf("hover is %T", hover)
	}

	if v := markupContent(t, h.Contents).Value; !strings.Contains(v, "Get a value from the public or private request collection") ||
		!strings.Contains(v, "`name` — The key name") {
		t.Errorf("hover lacks ColdBox's docs:\n%s", v)
	}

	found, err := s.definitionAnswer(ctx, onGetValue)
	if err != nil {
		t.Fatal(err)
	}

	loc, _ := found.(protocol.Location)
	if !frameworkapi.IsStubURI(string(loc.URI)) {
		t.Fatalf("definition should have reached the stub before being filtered, got %#v", found)
	}

	if def, err := s.handleDefinition(ctx, onGetValue); err != nil || def != nil {
		t.Errorf("definition answered %#v, %v; want nothing for a stub", def, err)
	}

	sig, err := s.handleSignatureHelp(ctx, stubParams(t, docURI, stubPos(t, 2, `"x"`, 0)))
	if err != nil {
		t.Fatal(err)
	}

	help, _ := sig.(*protocol.SignatureHelp)
	if help == nil || len(help.Signatures) == 0 {
		t.Fatalf("no signature help: %#v", sig)
	}

	if doc, _ := help.Signatures[0].Documentation.(*protocol.MarkupContent); doc == nil || !strings.Contains(doc.Value, "Get a value") {
		t.Errorf("signature lacks the doc: %#v", help.Signatures[0].Documentation)
	}

	if len(help.Signatures[0].Parameters) == 0 || help.Signatures[0].Parameters[0].Documentation == nil {
		t.Errorf("the first argument is undocumented: %#v", help.Signatures[0].Parameters)
	}

	req, err := json.Marshal(protocol.CompletionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		Position:     stubPos(t, 3, "event.", len("event.")),
	})
	if err != nil {
		t.Fatal(err)
	}

	res, err := s.handleCompletion(ctx, req)
	if err != nil {
		t.Fatal(err)
	}

	list, _ := res.(*protocol.CompletionList)
	if list == nil {
		t.Fatalf("completion is %T", res)
	}

	var item *protocol.CompletionItem

	for i := range list.Items {
		if list.Items[i].Label == "getValue" {
			item = &list.Items[i]
		}
	}

	if item == nil {
		t.Fatalf("getValue not offered after event.: %d items", len(list.Items))
	}

	if doc, _ := item.Documentation.(*protocol.MarkupContent); doc == nil || !strings.Contains(doc.Value, "Get a value") {
		t.Errorf("getValue's item lacks the doc: %#v", item.Documentation)
	}

	symbols, err := s.handleWorkspaceSymbol(ctx, []byte(`{"query":"getValue"}`))
	if err != nil {
		t.Fatal(err)
	}

	list2, ok := symbols.([]protocol.SymbolInformation)
	if !ok {
		t.Fatalf("workspace symbols are %T", symbols)
	}

	for _, sym := range list2 {
		if frameworkapi.IsStubURI(string(sym.Location.URI)) {
			t.Errorf("workspace symbols list the stub's %s", sym.Name)
		}
	}
}
