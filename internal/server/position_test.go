package server

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/language"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// é is two bytes and one UTF-16 unit; 😀 is four bytes and two units.
func TestColumnConversions(t *testing.T) {
	text := "aé😀b"

	cases := []struct {
		char uint32
		col  int
	}{
		{0, 0},
		{1, 1},
		{2, 3},
		{4, 7},
		{5, 8},
		{7, 10}, // past the end keeps its excess
	}

	for _, c := range cases {
		if got := lineByteCol(text, c.char); got != c.col {
			t.Errorf("lineByteCol(%d) = %d, want %d", c.char, got, c.col)
		}

		if got := lineCol(text, c.col); got != c.char {
			t.Errorf("lineCol(%d) = %d, want %d", c.col, got, c.char)
		}
	}

	content := "plain\nx = \"😀\"; y\nlast"
	m := newColMapper(content)

	if got := m.col(1, uint32(strings.Index("x = \"😀\"; y", "y"))); got != 10 {
		t.Errorf("colMapper on a non-ASCII line = %d, want 10", got)
	}

	if got := m.col(0, 3); got != 3 {
		t.Errorf("colMapper on an ASCII line = %d, want 3", got)
	}

	if got := newColMapper("all ascii\nhere").col(1, 2); got != 2 {
		t.Errorf("colMapper on an ASCII document = %d, want 2", got)
	}
}

// The line every handler test below works on: two emoji — eight bytes, four
// UTF-16 units — before the code the cursor is in, so a column read as bytes
// lands four bytes early.
const emojiPrefix = `<cfset s = "😀😀">`

// utf16Of is where needle starts on line, in UTF-16 units, plus delta.
func utf16Of(t *testing.T, line, needle string, delta int) uint32 {
	t.Helper()

	i := strings.Index(line, needle)
	if i < 0 {
		t.Fatalf("%q not in %q", needle, line)
	}

	return utf16Len(line[:i]) + uint32(delta) //nolint:gosec // test positions are small
}

func TestHoverReadsTheColumnInUTF16Units(t *testing.T) {
	srv := newTestServer()
	line := emojiPrefix + "<cfset n = len(s)>"
	srv.setDocument("file:///hover.cfm", line)

	req := makeCall(t, protocol.MethodTextDocumentHover, protocol.HoverParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: "file:///hover.cfm"},
		Position:     protocol.Position{Line: 0, Character: utf16Of(t, line, "len", 1)},
	})

	result, err := srv.handleHover(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	hover, ok := result.(*protocol.Hover)
	if !ok || !strings.Contains(strings.ToLower(markupContent(t, hover.Contents).Value), "len") {
		t.Errorf("hover on len after two emoji: got %#v", result)
	}
}

func TestOnTypeFormattingAnswersInUTF16Units(t *testing.T) {
	srv := newTestServer()
	// The user has typed the first '>' of `<b>  >`; the edit removes it, the
	// spaces and the original '>' and puts one back.
	line := emojiPrefix + "<b>  >"
	srv.setDocument("file:///ontype.cfm", line)

	typed := utf16Of(t, line, "<b>", 3)

	req := makeCall(t, protocol.MethodTextDocumentOnTypeFormatting, protocol.DocumentOnTypeFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: "file:///ontype.cfm"},
		Position:     protocol.Position{Line: 0, Character: typed},
		Ch:           ">",
	})

	result, err := srv.handleOnTypeFormatting(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	edits, _ := result.([]protocol.TextEdit)
	if len(edits) != 1 {
		t.Fatalf("got %d edits, want 1: %#v", len(edits), result)
	}

	want := protocol.Range{
		Start: protocol.Position{Line: 0, Character: typed - 1},
		End:   protocol.Position{Line: 0, Character: utf16Len(line)},
	}
	if edits[0].Range != want {
		t.Errorf("edit range %+v, want %+v", edits[0].Range, want)
	}
}

func TestDuplicateGtCompletionAnswersInUTF16Units(t *testing.T) {
	srv := newTestServer()
	// `<b>>` with the cursor after the second '>': the item deletes it.
	line := emojiPrefix + "<b>>"
	docURI := uri.URI("file:///dupgt.cfm")
	srv.setDocument(docURI, line)

	at := utf16Len(line)

	req := makeCall(t, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
			Position:     protocol.Position{Line: 0, Character: at},
		},
		Context: protocol.CompletionContext{
			TriggerKind:      protocol.CompletionTriggerKindTriggerCharacter,
			TriggerCharacter: new(">"),
		},
	})

	result, err := srv.handleCompletion(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	var items []protocol.CompletionItem

	switch r := result.(type) {
	case []protocol.CompletionItem:
		items = r
	case *protocol.CompletionList:
		items = r.Items
	}

	want := protocol.Range{Start: protocol.Position{Character: at - 1}, End: protocol.Position{Character: at}}

	for _, it := range items {
		if it.Label != ">" {
			continue
		}

		if te := textEdit(t, it.TextEdit); te != nil {
			if te.Range != want {
				t.Errorf("duplicate '>' edit range %+v, want %+v", te.Range, want)
			}

			return
		}
	}

	t.Fatalf("no duplicate '>' item among %d items", len(items))
}

func TestDocumentLinksAnswerInUTF16Units(t *testing.T) {
	srv := newTestServer()
	line := emojiPrefix + `<cfinclude template="inc/header.cfm">`
	docURI := uri.URI("file:///links.cfm")
	srv.setDocument(docURI, line)

	req := makeCall(t, protocol.MethodTextDocumentDocumentLink, protocol.DocumentLinkParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
	})

	result, err := srv.handleDocumentLink(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	links, _ := result.([]protocol.DocumentLink)
	if len(links) != 1 {
		t.Fatalf("got %d links, want 1: %#v", len(links), result)
	}

	start := utf16Of(t, line, "inc/header.cfm", 0)
	want := protocol.Range{
		Start: protocol.Position{Character: start},
		End:   protocol.Position{Character: start + uint32(len("inc/header.cfm"))},
	}

	if links[0].Range != want {
		t.Errorf("link range %+v, want %+v", links[0].Range, want)
	}
}

func TestParseErrorDiagnosticsAnswerInUTF16Units(t *testing.T) {
	src := []byte(emojiPrefix + "<cfif>unclosed")

	tree := language.Parse(language.CFML, src, nil)
	defer tree.Close()

	byteDiags := 0

	for _, d := range collectErrorDiagnostics(tree.RootNode(), src) {
		if d.Range.Start.Line != 0 {
			continue
		}

		if d.Range.Start.Character > utf16Len(string(src)) || d.Range.End.Character > utf16Len(string(src)) {
			byteDiags++
		}
	}

	if byteDiags > 0 {
		t.Errorf("%d diagnostics end past the line's UTF-16 length; columns are bytes", byteDiags)
	}
}

// Every column a request carries goes through byteCol before a handler reads
// the line with it. The behavioural tests above cover the handlers that were
// wrong; this covers the next one, which would otherwise be written as
// int(params.Position.Character) — the obvious spelling, and the bug.
//
// Only a request's own params are checked. didChange hands its edit ranges to
// parser.ApplyEdit as they arrived, and that is right: the parser converts
// UTF-16 units itself, since it applies the edit to text only it holds.
func TestNoHandlerReadsAClientColumnRaw(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}

	fset := token.NewFileSet()

	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}

		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}

		f, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatal(err)
		}

		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 1 {
				return true
			}

			fn, ok := call.Fun.(*ast.Ident)
			if !ok || fn.Name != "int" {
				return true
			}

			sel, ok := call.Args[0].(*ast.SelectorExpr)
			if ok && sel.Sel.Name == "Character" && rootIdent(sel) == "params" {
				t.Errorf("%s: %s converts a client column to a byte column by cast; use byteCol",
					fset.Position(call.Pos()), src[fset.Position(call.Pos()).Offset:fset.Position(call.End()).Offset])
			}

			return true
		})
	}
}

// rootIdent is the identifier a selector chain starts from: params for
// params.Position.Character.
func rootIdent(e ast.Expr) string {
	for {
		switch x := e.(type) {
		case *ast.SelectorExpr:
			e = x.X
		case *ast.Ident:
			return x.Name
		default:
			return ""
		}
	}
}

// cfmleditor.goToMatchingTag takes the editor's cursor as arguments and answers
// with a position the editor moves to, so both are LSP columns. Read as a byte
// column, the cursor after two emoji landed four bytes early, inside the
// string before the tag, and no tag was found.
func TestGoToMatchingTagCountsUTF16Units(t *testing.T) {
	srv := newTestServer()
	line := emojiPrefix + "<cfif a>b</cfif>"
	docURI := uri.URI("file:///match.cfm")
	srv.setDocument(docURI, line)

	req := makeCall(t, protocol.MethodWorkspaceExecuteCommand, protocol.ExecuteCommandParams{
		Command:   "cfmleditor.goToMatchingTag",
		Arguments: lspAnyArgs(string(docURI), 0, utf16Of(t, line, "<cfif", 1)),
	})

	result, err := srv.handleExecuteCommand(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	pos, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("no matching tag found: %#v", result)
	}

	want := utf16Of(t, line, "</cfif", 0)
	if got, _ := pos["character"].(uint32); pos["line"] != 0 || got != want {
		t.Errorf("matching tag at %v, want line 0 character %d", pos, want)
	}
}
