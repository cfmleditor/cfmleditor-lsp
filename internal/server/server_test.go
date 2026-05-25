package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"github.com/cfmleditor/cfmleditor-lsp/internal/refs"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func newTestServer() *Server {
	return NewServer(nil, cflog.NewLogger(false))
}

func makeCall(t *testing.T, method string, params interface{}) jsonrpc2.Request {
	t.Helper()
	req, err := jsonrpc2.NewCall(jsonrpc2.NewNumberID(1), method, params)
	if err != nil {
		t.Fatal(err)
	}
	return req
}

func captureReply(t *testing.T) (jsonrpc2.Replier, *interface{}, *error) {
	t.Helper()
	var result interface{}
	var replyErr error
	replier := func(_ context.Context, res interface{}, err error) error {
		result = res
		replyErr = err
		return nil
	}
	return replier, &result, &replyErr
}

func TestHandleInitialize(t *testing.T) {
	srv := newTestServer()
	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodInitialize, protocol.InitializeParams{})

	if err := srv.handleInitialize(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}
	if !srv.initialized {
		t.Error("expected server to be initialized")
	}

	res, ok := (*result).(protocol.InitializeResult)
	if !ok {
		t.Fatalf("expected InitializeResult, got %T", *result)
	}
	if res.ServerInfo.Name != "cfmleditor-lsp" {
		t.Errorf("expected server name cfmleditor-lsp, got %s", res.ServerInfo.Name)
	}
	if res.Capabilities.CompletionProvider == nil {
		t.Error("expected completion provider to be set")
	}
}

func TestHandleDidOpen(t *testing.T) {
	srv := newTestServer()
	reply, _, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:  "file:///test.cfm",
			Text: "<cfoutput>hello</cfoutput>",
		},
	})

	if err := srv.handleDidOpen(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	content, ok := srv.getDocument(uri.URI("file:///test.cfm"))
	if !ok {
		t.Fatal("document not found")
	}
	if content != "<cfoutput>hello</cfoutput>" {
		t.Errorf("unexpected content: %s", content)
	}
}

func TestHandleDidChange(t *testing.T) {
	srv := newTestServer()
	srv.setDocument(uri.URI("file:///test.cfm"), "old content")

	reply, _, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDidChange, protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: "file:///test.cfm"},
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			{Text: "new content"},
		},
	})

	if err := srv.handleDidChange(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	content, _ := srv.getDocument(uri.URI("file:///test.cfm"))
	if content != "new content" {
		t.Errorf("expected 'new content', got '%s'", content)
	}
}

func TestHandleDidClose(t *testing.T) {
	srv := newTestServer()
	cfcURI := uri.URI("file:///test.cfc")
	cfcContent := "component {\nfunction hello() {}\n}"
	srv.setDocument(cfcURI, cfcContent)
	srv.index.IndexFile(cfcURI, cfcContent)

	reply, _, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDidClose, protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(cfcURI)},
	})

	if err := srv.handleDidClose(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	if _, ok := srv.getDocument(cfcURI); ok {
		t.Error("document should have been removed from open docs")
	}
	if defs := srv.index.Lookup("hello"); len(defs) != 1 {
		t.Error("index entry should be preserved after close")
	}
}

func TestCompletionTriggeredByTag(t *testing.T) {
	srv := newTestServer()
	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{
		Context: &protocol.CompletionContext{
			TriggerKind:      protocol.CompletionTriggerKindTriggerCharacter,
			TriggerCharacter: "<",
		},
	})

	if err := srv.handleCompletion(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	list := completionListFromResult(t, *result)
	if len(list.Items) == 0 {
		t.Fatal("expected tag completions")
	}
	for _, item := range list.Items {
		if item.Kind != protocol.CompletionItemKindKeyword {
			t.Errorf("expected Keyword kind for tag %s, got %v", item.Label, item.Kind)
		}
	}
	if list.Items[0].Label != "cfoutput" {
		// Order may vary with Lucee docs; just check cfoutput exists
		found := false
		for _, item := range list.Items {
			if strings.ToLower(item.Label) == "cfoutput" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected cfoutput in tag completions")
		}
	}
}

func TestCompletionTagWithDocContentInvoked(t *testing.T) {
	srv := newTestServer()
	srv.setDocument(uri.URI("file:///test.cfm"), "<cfoutput>hello</cfoutput>\n<")

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.cfm"},
			Position:     protocol.Position{Line: 1, Character: 1},
		},
		Context: &protocol.CompletionContext{
			TriggerKind: protocol.CompletionTriggerKindInvoked,
		},
	})

	if err := srv.handleCompletion(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	list := completionListFromResult(t, *result)
	if len(list.Items) == 0 {
		t.Fatal("expected tag completions when cursor is after < with Invoked trigger")
	}
	found := false
	for _, item := range list.Items {
		if strings.ToLower(item.Label) == "cfoutput" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected cfoutput in tag completions")
	}
}

func TestCompletionTriggeredByTyping(t *testing.T) {
	srv := newTestServer()
	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{
		Context: &protocol.CompletionContext{
			TriggerKind: protocol.CompletionTriggerKindInvoked,
		},
	})

	if err := srv.handleCompletion(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	list := completionListFromResult(t, *result)
	if len(list.Items) == 0 {
		t.Fatal("expected function completions")
	}
	for _, item := range list.Items {
		if item.Kind != protocol.CompletionItemKindFunction && item.Kind != protocol.CompletionItemKindKeyword {
			t.Errorf("expected Function or Keyword kind for %s, got %v", item.Label, item.Kind)
		}
	}
}

func TestCompletionWithNilContext(t *testing.T) {
	srv := newTestServer()
	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{})

	if err := srv.handleCompletion(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	list := completionListFromResult(t, *result)
	for _, item := range list.Items {
		if item.Kind != protocol.CompletionItemKindFunction && item.Kind != protocol.CompletionItemKindKeyword {
			t.Errorf("nil context should return functions, got kind %v for %s", item.Kind, item.Label)
		}
	}
}

func TestCompletionTagAttributes(t *testing.T) {
	srv := newTestServer()
	srv.setDocument(uri.URI("file:///test.cfm"), "<cfquery ")

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.cfm"},
			Position:     protocol.Position{Line: 0, Character: 9},
		},
		Context: &protocol.CompletionContext{
			TriggerKind:      protocol.CompletionTriggerKindTriggerCharacter,
			TriggerCharacter: " ",
		},
	})

	if err := srv.handleCompletion(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	list := completionListFromResult(t, *result)
	if len(list.Items) == 0 {
		t.Fatal("expected attribute completions for cfquery")
	}
	for _, item := range list.Items {
		if item.Kind != protocol.CompletionItemKindProperty {
			t.Errorf("expected Property kind for attribute %s, got %v", item.Label, item.Kind)
		}
	}
	found := false
	for _, item := range list.Items {
		if strings.EqualFold(item.Label, "datasource") || strings.EqualFold(item.Label, "dataSource") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected datasource/dataSource attribute in cfquery completions")
	}
}

func TestCompletionTagAttributesMultiline(t *testing.T) {
	srv := newTestServer()
	srv.setDocument(uri.URI("file:///test.cfm"), "<cfloop\n  ")

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.cfm"},
			Position:     protocol.Position{Line: 1, Character: 2},
		},
		Context: &protocol.CompletionContext{
			TriggerKind: protocol.CompletionTriggerKindInvoked,
		},
	})

	if err := srv.handleCompletion(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	list := completionListFromResult(t, *result)
	if len(list.Items) == 0 {
		t.Fatal("expected attribute completions for cfloop")
	}
	if list.Items[0].Kind != protocol.CompletionItemKindProperty {
		t.Errorf("expected Property kind, got %v", list.Items[0].Kind)
	}
}

func TestCompletionSpecialTagShowsFunctions(t *testing.T) {
	tags := []struct {
		doc string
		pos protocol.Position
	}{
		{"<cfset ", protocol.Position{Line: 0, Character: 7}},
		{"<cfif ", protocol.Position{Line: 0, Character: 6}},
		{"<cfelseif ", protocol.Position{Line: 0, Character: 10}},
	}
	for _, tc := range tags {
		srv := newTestServer()
		srv.setDocument(uri.URI("file:///test.cfm"), tc.doc)

		reply, result, replyErr := captureReply(t)
		req := makeCall(t, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.cfm"},
				Position:     tc.pos,
			},
			Context: &protocol.CompletionContext{TriggerKind: protocol.CompletionTriggerKindInvoked},
		})

		if err := srv.handleCompletion(context.Background(), reply, req); err != nil {
			t.Fatalf("%s: %v", tc.doc, err)
		}
		if *replyErr != nil {
			t.Fatalf("%s: %v", tc.doc, *replyErr)
		}

		list := completionListFromResult(t, *result)
		if len(list.Items) == 0 {
			t.Fatalf("%s: expected function completions", tc.doc)
		}
		for _, item := range list.Items {
			if item.Kind != protocol.CompletionItemKindFunction && item.Kind != protocol.CompletionItemKindKeyword {
				t.Errorf("%s: expected Function or Keyword kind, got %v for %s", tc.doc, item.Kind, item.Label)
			}
		}
	}
}

func TestCompletionCfElseOffersIf(t *testing.T) {
	srv := newTestServer()
	srv.setDocument(uri.URI("file:///test.cfm"), "<cfelse ")

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.cfm"},
			Position:     protocol.Position{Line: 0, Character: 8},
		},
		Context: &protocol.CompletionContext{TriggerKind: protocol.CompletionTriggerKindInvoked},
	})

	if err := srv.handleCompletion(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	list := completionListFromResult(t, *result)
	if len(list.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(list.Items))
	}
	item := list.Items[0]
	if item.Label != "if" {
		t.Errorf("expected label 'if', got %q", item.Label)
	}
	if item.TextEdit == nil {
		t.Fatal("expected TextEdit to be set")
	}
	if item.TextEdit.Range.Start.Character != 0 {
		t.Errorf("expected start char 0, got %d", item.TextEdit.Range.Start.Character)
	}
	if item.TextEdit.NewText != "<cfelseif $1" {
		t.Errorf("expected NewText '<cfelseif $1', got %q", item.TextEdit.NewText)
	}
}

func TestCompletionAfterClosedTag(t *testing.T) {
	srv := newTestServer()
	srv.setDocument(uri.URI("file:///test.cfm"), "<cfoutput>hello")

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.cfm"},
			Position:     protocol.Position{Line: 0, Character: 15},
		},
		Context: &protocol.CompletionContext{
			TriggerKind: protocol.CompletionTriggerKindInvoked,
		},
	})

	if err := srv.handleCompletion(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	list := completionListFromResult(t, *result)
	for _, item := range list.Items {
		if item.Kind != protocol.CompletionItemKindFunction && item.Kind != protocol.CompletionItemKindKeyword {
			t.Errorf("after closed tag should return functions, got kind %v for %s", item.Kind, item.Label)
		}
	}
}

func TestCompletionClosingTag(t *testing.T) {
	srv := newTestServer()
	srv.setDocument(uri.URI("file:///test.cfm"), "<cfoutput>hello</")

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.cfm"},
			Position:     protocol.Position{Line: 0, Character: 17},
		},
		Context: &protocol.CompletionContext{
			TriggerKind:      protocol.CompletionTriggerKindTriggerCharacter,
			TriggerCharacter: "/",
		},
	})

	if err := srv.handleCompletion(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	list := completionListFromResult(t, *result)
	if len(list.Items) != 1 {
		t.Fatalf("expected 1 closing tag, got %d", len(list.Items))
	}
	if list.Items[0].Label != "cfoutput" {
		t.Errorf("expected cfoutput, got %s", list.Items[0].Label)
	}
	if list.Items[0].InsertText != "cfoutput>" {
		t.Errorf("expected insert text 'cfoutput>', got %s", list.Items[0].InsertText)
	}
}

func TestCompletionClosingTagNested(t *testing.T) {
	srv := newTestServer()
	srv.setDocument(uri.URI("file:///test.cfm"), "<cfoutput><cfloop query=\"q\">hello</")

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.cfm"},
			Position:     protocol.Position{Line: 0, Character: 36},
		},
		Context: &protocol.CompletionContext{
			TriggerKind:      protocol.CompletionTriggerKindTriggerCharacter,
			TriggerCharacter: "/",
		},
	})

	if err := srv.handleCompletion(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	list := completionListFromResult(t, *result)
	if len(list.Items) != 2 {
		t.Fatalf("expected 2 closing tags, got %d", len(list.Items))
	}
	if list.Items[0].Label != "cfloop" {
		t.Errorf("expected most recent unclosed tag cfloop first, got %s", list.Items[0].Label)
	}
	if list.Items[1].Label != "cfoutput" {
		t.Errorf("expected cfoutput second, got %s", list.Items[1].Label)
	}
}

func TestCompletionClosingTagAlreadyClosed(t *testing.T) {
	srv := newTestServer()
	srv.setDocument(uri.URI("file:///test.cfm"), "<cfoutput>hello</cfoutput></")

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.cfm"},
			Position:     protocol.Position{Line: 0, Character: 28},
		},
		Context: &protocol.CompletionContext{
			TriggerKind:      protocol.CompletionTriggerKindTriggerCharacter,
			TriggerCharacter: "/",
		},
	})

	if err := srv.handleCompletion(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	list := completionListFromResult(t, *result)
	if len(list.Items) != 0 {
		t.Errorf("expected no closing tags, got %d", len(list.Items))
	}
}

func TestFindUnclosedTags(t *testing.T) {
	tests := []struct {
		name    string
		content string
		line    int
		char    int
		want    []string
	}{
		{"single open", "<cfoutput></", 0, 12, []string{"cfoutput"}},
		{"nested", "<cfoutput><cfloop query=\"q\"></", 0, 30, []string{"cfloop", "cfoutput"}},
		{"all closed", "<cfoutput></cfoutput></", 0, 22, nil},
		{"self closing", "<cfset value=\"1\" /></", 0, 21, nil},
		{"cfif is unclosed", "<cfif true></", 0, 13, []string{"cfif"}},
		{"cfif closed", "<cfif true></cfif></", 0, 19, nil},
		{"cfelse not in stack", "<cfif true><cfelse></", 0, 20, []string{"cfif"}},
		{"cfelseif not in stack", "<cfif true><cfelseif false></", 0, 29, []string{"cfif"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findUnclosedTags(tt.content, 0, tt.line, tt.char)
			if len(got) != len(tt.want) {
				t.Fatalf("findUnclosedTags() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("findUnclosedTags()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFindEnclosingTag(t *testing.T) {
	tests := []struct {
		name    string
		content string
		line    int
		char    int
		want    string
	}{
		{"inside cfquery", "<cfquery ", 0, 9, "cfquery"},
		{"inside cfloop multiline", "<cfloop\n  ", 1, 2, "cfloop"},
		{"after closed tag", "<cfoutput>hello", 0, 15, ""},
		{"still typing tag name", "<cfq", 0, 4, ""},
		{"closing tag", "</cfoutput>", 0, 5, ""},
		{"no tag", "hello world", 0, 5, ""},
		{"with existing attrs", `<cfquery name="q" `, 0, 18, "cfquery"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findEnclosingTag(tt.content, tt.line, tt.char)
			if got != tt.want {
				t.Errorf("findEnclosingTag() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseFunctionDefs(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{"tag-based", `<cffunction name="getUser">`, []string{"getUser"}},
		{"script public", "component {\npublic function getData() {\n}\n}", []string{"getData"}},
		{"script bare", "component {\nfunction doStuff() {\n}\n}", []string{"doStuff"}},
		{"script with return type", "component {\nprivate struct function buildQuery() {\n}\n}", []string{"buildQuery"}},
		{"mixed tag and script", "<cfcomponent>\n<cffunction name=\"a\">\n</cffunction>\n<cfscript>\nfunction b() {\n}\n</cfscript>\n</cfcomponent>", []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defs := parser.ParseFunctionDefs("file:///test.cfc", tt.content)
			if len(defs) != len(tt.want) {
				t.Fatalf("got %d defs, want %d", len(defs), len(tt.want))
			}
			for i, d := range defs {
				if d.Name != tt.want[i] {
					t.Errorf("def[%d].Name = %q, want %q", i, d.Name, tt.want[i])
				}
			}
		})
	}
}

func TestDefinitionLookup(t *testing.T) {
	dir := t.TempDir()
	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}

	cfcContent := `<cfcomponent>
<cffunction name="getUser">
	<cfreturn "user">
</cffunction>
</cfcomponent>`
	_ = os.WriteFile(filepath.Join(dir, "User.cfc"), []byte(cfcContent), 0o644)
	cfcURI := uri.URI("file://" + filepath.Join(dir, "User.cfc"))
	srv.index.IndexFile(cfcURI, cfcContent)

	callerContent := `<cfset userObj = new User()><cfset result = userObj.getUser()>`
	callerURI := uri.URI("file://" + filepath.Join(dir, "index.cfm"))
	srv.setDocument(callerURI, callerContent)
	srv.index.IndexFile(callerURI, callerContent)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDefinition, protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(callerURI)},
			Position:     protocol.Position{Line: 0, Character: 55},
		},
	})

	if err := srv.handleDefinition(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	loc, ok := (*result).(protocol.Location)
	if !ok {
		t.Fatalf("expected Location, got %T", *result)
	}
	if !strings.Contains(string(loc.URI), "User.cfc") {
		t.Errorf("expected User.cfc, got %s", loc.URI)
	}
	if loc.Range.Start.Line != 1 {
		t.Errorf("expected line 1, got %d", loc.Range.Start.Line)
	}
}

func TestDefinitionNotFound(t *testing.T) {
	srv := newTestServer()
	srv.setDocument(uri.URI("file:///test.cfm"), "<cfset x = noSuchFunc()>")

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDefinition, protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.cfm"},
			Position:     protocol.Position{Line: 0, Character: 14},
		},
	})

	if err := srv.handleDefinition(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}
	if *result != nil {
		t.Errorf("expected nil result for unknown function, got %v", *result)
	}
}

func TestWordAtPosition(t *testing.T) {
	tests := []struct {
		name    string
		content string
		line    int
		char    int
		want    string
	}{
		{"middle of word", "getUser()", 0, 3, "getUser"},
		{"start of word", "getUser()", 0, 0, "getUser"},
		{"on paren", "getUser()", 0, 7, "getUser"},
		{"multiline", "line1\ngetData()", 1, 3, "getData"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.WordAtPosition(tt.content, tt.line, tt.char)
			if got != tt.want {
				t.Errorf("parser.WordAtPosition() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIndexReindexOnChange(t *testing.T) {
	srv := newTestServer()
	cfcURI := uri.URI("file:///app/Service.cfc")

	srv.index.IndexFile(cfcURI, "component {\nfunction oldFunc() {}\n}")
	if defs := srv.index.Lookup("oldFunc"); len(defs) != 1 {
		t.Fatal("expected oldFunc indexed")
	}

	srv.index.IndexFile(cfcURI, "component {\nfunction newFunc() {}\n}")
	if defs := srv.index.Lookup("oldFunc"); len(defs) != 0 {
		t.Error("oldFunc should be removed after reindex")
	}
	if defs := srv.index.Lookup("newFunc"); len(defs) != 1 {
		t.Error("newFunc should be indexed")
	}
}

func TestDocumentSymbol(t *testing.T) {
	srv := newTestServer()
	content := `<cfcomponent>
<cffunction name="getUser">
</cffunction>
<cfscript>
function saveUser() {
}
</cfscript>
</cfcomponent>`
	srv.setDocument(uri.URI("file:///app/User.cfc"), content)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDocumentSymbol, protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: "file:///app/User.cfc"},
	})

	if err := srv.handleDocumentSymbol(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	symbols, ok := (*result).([]protocol.DocumentSymbol)
	if !ok {
		t.Fatalf("expected []DocumentSymbol, got %T", *result)
	}
	if len(symbols) != 2 {
		t.Fatalf("expected 2 symbols, got %d", len(symbols))
	}
	if symbols[0].Name != "getUser" {
		t.Errorf("expected getUser, got %s", symbols[0].Name)
	}
	if symbols[1].Name != "saveUser" {
		t.Errorf("expected saveUser, got %s", symbols[1].Name)
	}
}

func TestWorkspaceSymbol(t *testing.T) {
	srv := newTestServer()
	srv.index.IndexFile("file:///app/User.cfc", "component {\nfunction getUser() {}\nfunction deleteUser() {}\n}")
	srv.index.IndexFile("file:///app/Order.cfc", "component {\nfunction getOrder() {}\n}")

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodWorkspaceSymbol, protocol.WorkspaceSymbolParams{Query: "get"})

	if err := srv.handleWorkspaceSymbol(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	symbols, ok := (*result).([]protocol.SymbolInformation)
	if !ok {
		t.Fatalf("expected []SymbolInformation, got %T", *result)
	}
	if len(symbols) != 2 {
		t.Fatalf("expected 2 symbols matching 'get', got %d", len(symbols))
	}
	for _, s := range symbols {
		if !strings.Contains(strings.ToLower(s.Name), "get") {
			t.Errorf("symbol %s should contain 'get'", s.Name)
		}
	}
}

func TestWorkspaceSymbolEmptyQuery(t *testing.T) {
	srv := newTestServer()
	srv.index.IndexFile("file:///app/User.cfc", "component {\nfunction getUser() {}\nfunction deleteUser() {}\n}")

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodWorkspaceSymbol, protocol.WorkspaceSymbolParams{Query: ""})

	if err := srv.handleWorkspaceSymbol(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	symbols, ok := (*result).([]protocol.SymbolInformation)
	if !ok {
		t.Fatalf("expected []SymbolInformation, got %T", *result)
	}
	if len(symbols) != 2 {
		t.Fatalf("expected all 2 symbols for empty query, got %d", len(symbols))
	}
}

func TestHoverFunction(t *testing.T) {
	srv := newTestServer()
	srv.setDocument(uri.URI("file:///test.cfm"), "<cfset x = Len(y)>")

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentHover, protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.cfm"},
			Position:     protocol.Position{Line: 0, Character: 12},
		},
	})

	if err := srv.handleHover(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	hover, ok := (*result).(*protocol.Hover)
	if !ok {
		t.Fatalf("expected *Hover, got %T", *result)
	}
	if !strings.Contains(strings.ToLower(hover.Contents.Value), "len") {
		t.Errorf("expected hover to contain 'len', got %s", hover.Contents.Value)
	}
	if hover.Contents.Kind != protocol.Markdown {
		t.Errorf("expected markdown, got %s", hover.Contents.Kind)
	}
}

func TestHoverTag(t *testing.T) {
	srv := newTestServer()
	srv.setDocument(uri.URI("file:///test.cfm"), "<cfquery name=\"q\">")

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentHover, protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.cfm"},
			Position:     protocol.Position{Line: 0, Character: 3},
		},
	})

	if err := srv.handleHover(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	hover, ok := (*result).(*protocol.Hover)
	if !ok {
		t.Fatalf("expected *Hover, got %T", *result)
	}
	if !strings.Contains(hover.Contents.Value, "cfquery") {
		t.Errorf("expected hover to contain 'cfquery', got %s", hover.Contents.Value)
	}
}

func TestHoverUnknown(t *testing.T) {
	srv := newTestServer()
	srv.setDocument(uri.URI("file:///test.cfm"), "myCustomVar")

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentHover, protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.cfm"},
			Position:     protocol.Position{Line: 0, Character: 3},
		},
	})

	if err := srv.handleHover(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}
	if *result != nil {
		t.Errorf("expected nil for unknown word, got %v", *result)
	}
}

func completionListFromResult(t *testing.T, result interface{}) *protocol.CompletionList {
	t.Helper()
	// The reply captures the value as-is, but it may be a pointer
	if list, ok := result.(*protocol.CompletionList); ok {
		return list
	}
	// Fall back to re-marshal/unmarshal
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var list protocol.CompletionList
	if err := json.Unmarshal(data, &list); err != nil {
		t.Fatal(err)
	}
	return &list
}

func TestWorkspaceFoldersAreIndexed(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "Shared.cfc"), []byte("component {\nfunction sharedHelper() {}\n}"), 0o644)

	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}
	srv.indexWorkspace()

	if defs := srv.index.Lookup("sharedHelper"); len(defs) != 1 {
		t.Errorf("expected sharedHelper indexed, got %d defs", len(defs))
	}
}

func TestWorkspaceFoldersSkipsWorkspaceRoots(t *testing.T) {
	wsDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(wsDir, "Local.cfc"), []byte("component {\nfunction localFunc() {}\n}"), 0o644)

	folderDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(folderDir, "Extra.cfc"), []byte("component {\nfunction extraFunc() {}\n}"), 0o644)

	srv := newTestServer()
	srv.workspaceRoots = []string{wsDir}
	srv.WorkspaceFolders = []string{folderDir}
	srv.indexWorkspace()

	if defs := srv.index.Lookup("localFunc"); len(defs) != 0 {
		t.Errorf("workspace root should be skipped when WorkspaceFolders set, got %d defs", len(defs))
	}
	if defs := srv.index.Lookup("extraFunc"); len(defs) != 1 {
		t.Errorf("expected extraFunc indexed, got %d defs", len(defs))
	}
}

func TestIndexGlobsFilterFiles(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "Wanted.cfc"), []byte("component {\nfunction wantedFunc() {}\n}"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "Unwanted.cfc"), []byte("component {\nfunction unwantedFunc() {}\n}"), 0o644)

	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}
	srv.IndexGlobs = []string{dir + "/Wanted.cfc"}
	srv.indexWorkspace()

	if defs := srv.index.Lookup("wantedFunc"); len(defs) != 1 {
		t.Errorf("expected wantedFunc indexed, got %d defs", len(defs))
	}
	if defs := srv.index.Lookup("unwantedFunc"); len(defs) != 0 {
		t.Errorf("unwantedFunc should not be indexed, got %d defs", len(defs))
	}
}

func TestReindexWithGlobsFilter(t *testing.T) {
	srv := newTestServer()
	srv.WorkspaceFolders = []string{"/project"}
	srv.IndexGlobs = []string{"/project/**/*.cfc"}

	srv.reindexIfCFC("file:///project/Service.cfc", "component {\nfunction allowedFunc() {}\n}")
	if defs := srv.index.Lookup("allowedFunc"); len(defs) != 1 {
		t.Errorf("expected allowedFunc indexed, got %d defs", len(defs))
	}

	srv.reindexIfCFC("file:///project/sub/Deep.cfc", "component {\nfunction deepFunc() {}\n}")
	if defs := srv.index.Lookup("deepFunc"); len(defs) != 1 {
		t.Errorf("expected deepFunc indexed, got %d defs", len(defs))
	}

	srv.reindexIfCFC("file:///other/Rogue.cfc", "component {\nfunction rogueFunc() {}\n}")
	if defs := srv.index.Lookup("rogueFunc"); len(defs) != 0 {
		t.Errorf("rogueFunc should not be indexed, got %d defs", len(defs))
	}
}

func TestReindexFoldersWithoutGlobs(t *testing.T) {
	srv := newTestServer()
	srv.WorkspaceFolders = []string{"/project"}

	srv.reindexIfCFC("file:///project/Any.cfc", "component {\nfunction anyFunc() {}\n}")
	if defs := srv.index.Lookup("anyFunc"); len(defs) != 1 {
		t.Errorf("expected anyFunc indexed under workspace folder, got %d defs", len(defs))
	}

	srv.reindexIfCFC("file:///outside/Rogue.cfc", "component {\nfunction rogueFunc() {}\n}")
	if defs := srv.index.Lookup("rogueFunc"); len(defs) != 0 {
		t.Errorf("rogueFunc outside workspace folders should not be indexed, got %d defs", len(defs))
	}
}

func TestReindexNoFilterWithoutFolders(t *testing.T) {
	srv := newTestServer()
	srv.reindexIfCFC("file:///anywhere/Thing.cfc", "component {\nfunction anyFunc() {}\n}")
	if defs := srv.index.Lookup("anyFunc"); len(defs) != 1 {
		t.Errorf("expected anyFunc indexed without WorkspaceFolders, got %d defs", len(defs))
	}
}

func TestDidChangeWorkspaceFoldersAdd(t *testing.T) {
	dir := t.TempDir()
	cfcPath := filepath.Join(dir, "Added.cfc")
	_ = os.WriteFile(cfcPath, []byte("component {\nfunction addedFunc() {}\n}"), 0o644)

	srv := newTestServer()
	reply, _, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodWorkspaceDidChangeWorkspaceFolders, protocol.DidChangeWorkspaceFoldersParams{
		Event: protocol.WorkspaceFoldersChangeEvent{
			Added: []protocol.WorkspaceFolder{{URI: "file://" + dir, Name: "added"}},
		},
	})

	if err := srv.handleDidChangeWorkspaceFolders(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	if defs := srv.index.Lookup("addedFunc"); len(defs) != 1 {
		t.Errorf("expected addedFunc to be indexed, got %d defs", len(defs))
	}
}

func TestDidChangeWorkspaceFoldersRemove(t *testing.T) {
	srv := newTestServer()
	srv.index.IndexFile("file:///workspace/A/Service.cfc", "component {\nfunction svcFunc() {}\n}")
	srv.index.IndexFile("file:///workspace/B/Other.cfc", "component {\nfunction otherFunc() {}\n}")
	srv.workspaceRoots = []string{"/workspace/A", "/workspace/B"}

	reply, _, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodWorkspaceDidChangeWorkspaceFolders, protocol.DidChangeWorkspaceFoldersParams{
		Event: protocol.WorkspaceFoldersChangeEvent{
			Removed: []protocol.WorkspaceFolder{{URI: "file:///workspace/A", Name: "A"}},
		},
	})

	if err := srv.handleDidChangeWorkspaceFolders(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	if defs := srv.index.Lookup("svcFunc"); len(defs) != 0 {
		t.Error("svcFunc should have been removed from index")
	}
	if defs := srv.index.Lookup("otherFunc"); len(defs) != 1 {
		t.Error("otherFunc should still be in index")
	}
	if len(srv.workspaceRoots) != 1 || srv.workspaceRoots[0] != "/workspace/B" {
		t.Errorf("expected workspaceRoots [/workspace/B], got %v", srv.workspaceRoots)
	}
}

func TestRemoveFilesUnder(t *testing.T) {
	idx := index.New()
	idx.IndexFile("file:///project/a/One.cfc", "component {\nfunction oneFunc() {}\n}")
	idx.IndexFile("file:///project/b/Two.cfc", "component {\nfunction twoFunc() {}\n}")

	idx.RemoveFilesUnder("file:///project/a")

	if defs := idx.Lookup("oneFunc"); len(defs) != 0 {
		t.Error("oneFunc should have been removed")
	}
	if defs := idx.Lookup("twoFunc"); len(defs) != 1 {
		t.Error("twoFunc should still exist")
	}
}

func TestDidChangeWorkspaceFoldersRemoveProtectsWorkspaceFolders(t *testing.T) {
	srv := newTestServer()
	srv.WorkspaceFolders = []string{"/shared/lib"}
	srv.index.IndexFile("file:///shared/lib/Utils.cfc", "component {\nfunction sharedUtil() {}\n}")
	srv.index.IndexFile("file:///workspace/App.cfc", "component {\nfunction appFunc() {}\n}")
	srv.workspaceRoots = []string{"/shared/lib", "/workspace"}

	reply, _, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodWorkspaceDidChangeWorkspaceFolders, protocol.DidChangeWorkspaceFoldersParams{
		Event: protocol.WorkspaceFoldersChangeEvent{
			Removed: []protocol.WorkspaceFolder{
				{URI: "file:///shared/lib", Name: "lib"},
				{URI: "file:///workspace", Name: "workspace"},
			},
		},
	})

	if err := srv.handleDidChangeWorkspaceFolders(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	if defs := srv.index.Lookup("sharedUtil"); len(defs) != 1 {
		t.Error("sharedUtil should be preserved (workspace folder)")
	}
	if defs := srv.index.Lookup("appFunc"); len(defs) != 0 {
		t.Error("appFunc should have been removed")
	}
}

func TestOnTypeFormattingRemovesDuplicateClose(t *testing.T) {
	srv := newTestServer()
	// User typed '>' before existing '>'. Doc now has '>>' at indices 21-22. Cursor at 22.
	srv.setDocument(uri.URI("file:///test.cfm"), `<cfoutput name="test">>hello</cfoutput>`)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentOnTypeFormatting, protocol.DocumentOnTypeFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.cfm"},
		Position:     protocol.Position{Line: 0, Character: 22}, // after the typed '>'
		Ch:           ">",
	})

	if err := srv.handleOnTypeFormatting(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	edits, ok := (*result).([]protocol.TextEdit)
	if !ok {
		t.Fatalf("expected []TextEdit, got %T", *result)
	}
	if len(edits) != 1 {
		t.Fatalf("expected 1 edit, got %d", len(edits))
	}
	if edits[0].NewText != ">" {
		t.Errorf("expected NewText '>', got %q", edits[0].NewText)
	}
	if edits[0].Range.Start.Character != 21 || edits[0].Range.End.Character != 23 {
		t.Errorf("expected range [21,23), got [%d,%d)", edits[0].Range.Start.Character, edits[0].Range.End.Character)
	}
}

func TestOnTypeFormattingMidTagWhitespaceOnly(t *testing.T) {
	srv := newTestServer()
	// User typed '>' with only whitespace before existing '>'. Doc: "<cfif>  >"
	srv.setDocument(uri.URI("file:///test.cfm"), "<cfif>  >")

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentOnTypeFormatting, protocol.DocumentOnTypeFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.cfm"},
		Position:     protocol.Position{Line: 0, Character: 6},
		Ch:           ">",
	})

	if err := srv.handleOnTypeFormatting(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	edits, ok := (*result).([]protocol.TextEdit)
	if !ok {
		t.Fatalf("expected []TextEdit, got %T", *result)
	}
	if len(edits) != 1 {
		t.Fatalf("expected 1 edit, got %d", len(edits))
	}
	// Replaces [typed>, whitespace, orig>] with just ">"
	if edits[0].NewText != ">" {
		t.Errorf("expected NewText '>', got %q", edits[0].NewText)
	}
	if edits[0].Range.Start.Character != 5 || edits[0].Range.End.Character != 9 {
		t.Errorf("expected range [5,9), got [%d,%d)", edits[0].Range.Start.Character, edits[0].Range.End.Character)
	}
}

func TestOnTypeFormattingNoOpNonWhitespace(t *testing.T) {
	srv := newTestServer()
	// Non-whitespace between typed '>' and existing '>': should not act.
	srv.setDocument(uri.URI("file:///test.cfm"), "<cfif> true>")

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentOnTypeFormatting, protocol.DocumentOnTypeFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.cfm"},
		Position:     protocol.Position{Line: 0, Character: 6},
		Ch:           ">",
	})

	if err := srv.handleOnTypeFormatting(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	edits, ok := (*result).([]protocol.TextEdit)
	if !ok {
		t.Fatalf("expected []TextEdit, got %T", *result)
	}
	if len(edits) != 0 {
		t.Errorf("expected no edits, got %d", len(edits))
	}
}

func TestCompletionCloseTagTriggeredByGt(t *testing.T) {
	srv := newTestServer()
	// User typed '>' mid-tag with non-whitespace after. Doc: "<cfif> true>"
	srv.setDocument(uri.URI("file:///test.cfm"), "<cfif> true>")

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.cfm"},
			Position:     protocol.Position{Line: 0, Character: 6},
		},
		Context: &protocol.CompletionContext{
			TriggerKind:      protocol.CompletionTriggerKindTriggerCharacter,
			TriggerCharacter: ">",
		},
	})

	if err := srv.handleCompletion(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	list := completionListFromResult(t, *result)
	if len(list.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(list.Items))
	}
	item := list.Items[0]
	if item.Label != ">" {
		t.Errorf("expected label '>', got %q", item.Label)
	}
	if item.TextEdit == nil {
		t.Fatal("expected TextEdit")
	}
	if item.TextEdit.NewText != " true>" {
		t.Errorf("expected NewText ' true>', got %q", item.TextEdit.NewText)
	}
}

func TestCompletionDuplicateGtAfterTag(t *testing.T) {
	srv := newTestServer()
	// User typed '>' after existing '>'. Doc: "<cfif test>></cfif>", cursor at 12.
	srv.setDocument(uri.URI("file:///test.cfm"), "<cfif test>></cfif>")

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.cfm"},
			Position:     protocol.Position{Line: 0, Character: 12},
		},
		Context: &protocol.CompletionContext{
			TriggerKind:      protocol.CompletionTriggerKindTriggerCharacter,
			TriggerCharacter: ">",
		},
	})

	if err := srv.handleCompletion(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	list := completionListFromResult(t, *result)
	if len(list.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(list.Items))
	}
	item := list.Items[0]
	if item.Detail != "Remove duplicate >" {
		t.Errorf("expected detail 'Remove duplicate >', got %q", item.Detail)
	}
	if item.TextEdit == nil {
		t.Fatal("expected TextEdit")
	}
	if item.TextEdit.NewText != "" {
		t.Errorf("expected empty NewText, got %q", item.TextEdit.NewText)
	}
	if item.TextEdit.Range.Start.Character != 11 || item.TextEdit.Range.End.Character != 12 {
		t.Errorf("expected range [11,12), got [%d,%d)", item.TextEdit.Range.Start.Character, item.TextEdit.Range.End.Character)
	}
}

func TestOnTypeFormattingNoOpWithoutDuplicate(t *testing.T) {
	srv := newTestServer()
	// Document after user typed '>': the '>' at index 21 is the one they typed, cursor at 22.
	srv.setDocument(uri.URI("file:///test.cfm"), `<cfoutput name="test">hello</cfoutput>`)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentOnTypeFormatting, protocol.DocumentOnTypeFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.cfm"},
		Position:     protocol.Position{Line: 0, Character: 22}, // after the '>'
		Ch:           ">",
	})

	if err := srv.handleOnTypeFormatting(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	edits, ok := (*result).([]protocol.TextEdit)
	if !ok {
		t.Fatalf("expected []TextEdit, got %T", *result)
	}
	if len(edits) != 0 {
		t.Errorf("expected no edits, got %d", len(edits))
	}
}

func TestCompletionDotComponentMethods(t *testing.T) {
	// Create a temp CFC file with functions
	dir := t.TempDir()
	sub := filepath.Join(dir, "models")
	_ = os.MkdirAll(sub, 0o755)
	_ = os.WriteFile(filepath.Join(sub, "User.cfc"), []byte("component {\nfunction getName() {}\nfunction setName(required string name) {}\n}"), 0o644)

	// Set up server with a document that references the CFC
	srv := newTestServer()
	docURI := uri.URI("file://" + filepath.Join(dir, "test.cfm"))
	docContent := "userObj = new models.User()\nuserObj."
	srv.setDocument(docURI, docContent)

	// Index the document so the compref is stored
	srv.index.IndexFile(docURI, docContent)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 1, Character: 8}, // after "userObj."
		},
		Context: &protocol.CompletionContext{
			TriggerKind:      protocol.CompletionTriggerKindTriggerCharacter,
			TriggerCharacter: ".",
		},
	})

	if err := srv.handleCompletion(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	list := completionListFromResult(t, *result)
	if len(list.Items) != 2 {
		t.Fatalf("expected 2 method completions, got %d", len(list.Items))
	}

	found := map[string]bool{}
	for _, item := range list.Items {
		found[item.Label] = true
		if item.Kind != protocol.CompletionItemKindMethod {
			t.Errorf("expected Method kind for %s, got %v", item.Label, item.Kind)
		}
	}
	if !found["getName"] || !found["setName"] {
		t.Errorf("expected getName and setName, got %v", found)
	}
}

func TestCompletionDotPositionAware(t *testing.T) {
	// Create two CFC files with different methods
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "models"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "models", "User.cfc"), []byte("component {\nfunction getName() {}\n}"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "models", "Order.cfc"), []byte("component {\nfunction getTotal() {}\n}"), 0o644)

	srv := newTestServer()
	docURI := uri.URI("file://" + filepath.Join(dir, "test.cfm"))
	// myObj assigned to User on line 0, reassigned to Order on line 2
	docContent := "myObj = new models.User()\nmyObj.getName()\nmyObj = new models.Order()\nmyObj."
	srv.setDocument(docURI, docContent)
	srv.index.IndexFile(docURI, docContent)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 3, Character: 6}, // after "myObj." on line 3
		},
		Context: &protocol.CompletionContext{
			TriggerKind:      protocol.CompletionTriggerKindTriggerCharacter,
			TriggerCharacter: ".",
		},
	})

	if err := srv.handleCompletion(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	list := completionListFromResult(t, *result)
	if len(list.Items) != 1 {
		t.Fatalf("expected 1 method (getTotal), got %d: %v", len(list.Items), list.Items)
	}
	if list.Items[0].Label != "getTotal" {
		t.Errorf("expected getTotal, got %s", list.Items[0].Label)
	}
}

func TestCompletionDotUnscopedFromInit(t *testing.T) {
	dir := t.TempDir()
	// Create persist.cfc in the same directory
	_ = os.WriteFile(filepath.Join(dir, "persist.cfc"), []byte("component {\nfunction templateFunction() {}\nfunction otherMethod() {}\n}"), 0o644)

	srv := newTestServer()
	docURI := uri.URI("file://" + filepath.Join(dir, "service.cfc"))
	docContent := `<cfcomponent>
	<cfset VARIABLES._parent = StructNew() />
	<cffunction name="init" returntype="struct">
		<cfargument name="parent" type="struct" required="Yes" />
		<cfset VARIABLES.persist = createObject('component','persist').init(parent=VARIABLES._parent) />
		<cfreturn this />
	</cffunction>
	<cffunction name="templateFunction" output="false" returntype="struct" access="public">
		<cfset persist.>
	</cffunction>
</cfcomponent>`
	srv.setDocument(docURI, docContent)
	srv.index.IndexFile(docURI, docContent)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 8, Character: 17}, // after "persist."
		},
		Context: &protocol.CompletionContext{
			TriggerKind:      protocol.CompletionTriggerKindTriggerCharacter,
			TriggerCharacter: ".",
		},
	})

	if err := srv.handleCompletion(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	list := completionListFromResult(t, *result)
	if len(list.Items) != 2 {
		labels := make([]string, len(list.Items))
		for i, item := range list.Items {
			labels[i] = item.Label
		}
		t.Fatalf("expected 2 completions (templateFunction, otherMethod), got %d: %v", len(list.Items), labels)
	}
	found := map[string]bool{}
	for _, item := range list.Items {
		found[item.Label] = true
	}
	if !found["templateFunction"] || !found["otherMethod"] {
		t.Errorf("expected templateFunction and otherMethod, got %v", found)
	}
}

func TestDefinitionDotQualifiedCall(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "persist.cfc"), []byte("component {\nfunction someMethod() {}\n}"), 0o644)

	srv := newTestServer()
	docURI := uri.URI("file://" + filepath.Join(dir, "service.cfc"))
	docContent := "VARIABLES.persist = createObject(\"component\",\"persist\").init()\npersist.someMethod()"
	srv.setDocument(docURI, docContent)
	srv.index.IndexFile(docURI, docContent)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDefinition, protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 1, Character: 10}, // on "someMethod"
		},
	})
	if err := srv.handleDefinition(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}
	if *result == nil {
		t.Fatal("expected definition result, got nil")
	}
	loc, ok := (*result).(protocol.Location)
	if !ok {
		t.Fatalf("expected Location, got %T", *result)
	}
	if !strings.Contains(string(loc.URI), "persist.cfc") {
		t.Errorf("expected persist.cfc, got %s", loc.URI)
	}
}

func TestDefinitionDotQualifiedCallViaNew(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "Widget.cfc"), []byte("component {\nfunction render() {}\n}"), 0o644)

	srv := newTestServer()
	docURI := uri.URI("file://" + filepath.Join(dir, "caller.cfc"))
	docContent := "widget = new Widget()\nwidget.render()"
	srv.setDocument(docURI, docContent)
	srv.index.IndexFile(docURI, docContent)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDefinition, protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 1, Character: 8}, // on "render"
		},
	})
	if err := srv.handleDefinition(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}
	if *result == nil {
		t.Fatal("expected definition result, got nil")
	}
	loc, ok := (*result).(protocol.Location)
	if !ok {
		t.Fatalf("expected Location, got %T", *result)
	}
	if !strings.Contains(string(loc.URI), "Widget.cfc") {
		t.Errorf("expected Widget.cfc, got %s", loc.URI)
	}
}

func TestDefinitionDotQualifiedCallViaDottedNew(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "models"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "models", "User.cfc"), []byte("component {\nfunction getName() {}\n}"), 0o644)

	srv := newTestServer()
	docURI := uri.URI("file://" + filepath.Join(dir, "caller.cfc"))
	docContent := "user = new models.User()\nuser.getName()"
	srv.setDocument(docURI, docContent)
	srv.index.IndexFile(docURI, docContent)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDefinition, protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 1, Character: 6}, // on "getName"
		},
	})
	if err := srv.handleDefinition(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}
	if *result == nil {
		t.Fatal("expected definition result, got nil")
	}
	loc, ok := (*result).(protocol.Location)
	if !ok {
		t.Fatalf("expected Location, got %T", *result)
	}
	if !strings.Contains(string(loc.URI), "User.cfc") {
		t.Errorf("expected User.cfc, got %s", loc.URI)
	}
}

func TestDefinitionCfInvokeMethodAttribute(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "Widget.cfc"), []byte("<cfcomponent>\n<cffunction name=\"render\">\n</cffunction>\n</cfcomponent>"), 0o644)

	srv := newTestServer()
	docURI := uri.URI("file://" + filepath.Join(dir, "caller.cfm"))
	docContent := `<cfinvoke component="Widget" method="render">`
	srv.setDocument(docURI, docContent)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDefinition, protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 0, Character: 39}, // on "render" in method attr
		},
	})
	if err := srv.handleDefinition(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}
	if *result == nil {
		t.Fatal("expected definition result, got nil")
	}
	loc, ok := (*result).(protocol.Location)
	if !ok {
		t.Fatalf("expected Location, got %T", *result)
	}
	if !strings.Contains(string(loc.URI), "Widget.cfc") {
		t.Errorf("expected Widget.cfc, got %s", loc.URI)
	}
}

func TestDefinitionMethodWithinSameComponent(t *testing.T) {
	srv := newTestServer()
	cfcContent := `component {
function init() {
	var id = generateID();
}
function generateID() {
	return createUUID();
}
}`
	cfcURI := uri.URI("file:///app/Widget.cfc")
	srv.setDocument(cfcURI, cfcContent)
	srv.index.IndexFile(cfcURI, cfcContent)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDefinition, protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(cfcURI)},
			Position:     protocol.Position{Line: 2, Character: 12}, // on "generateID"
		},
	})
	if err := srv.handleDefinition(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}
	if *result == nil {
		t.Fatal("expected definition result, got nil")
	}
	loc, ok := (*result).(protocol.Location)
	if !ok {
		t.Fatalf("expected Location, got %T", *result)
	}
	if loc.URI != protocol.DocumentURI(cfcURI) {
		t.Errorf("expected same file URI, got %s", loc.URI)
	}
	if loc.Range.Start.Line != 4 {
		t.Errorf("expected line 4, got %d", loc.Range.Start.Line)
	}
}

func TestDefinitionComponentResolver(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "packages", "timetable"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "packages", "timetable", "service.cfc"), []byte("component {\nfunction getSchedule() {}\n}"), 0o644)

	srv := newTestServer()
	srv.ComponentResolvers = []config.Resolver{
		{Match: `getService("$1")`, Resolve: "packages.$1.service", Prefix: "getService"},
	}
	srv.WorkspaceFolders = []string{dir}

	docURI := uri.URI("file://" + filepath.Join(dir, "caller.cfc"))
	docContent := "svc = getService(\"timetable\")\nsvc.getSchedule()"
	srv.setDocument(docURI, docContent)
	pr := parser.Parse(docURI, docContent, srv.cfResolvers())
	srv.index.IndexFileFromResult(docURI, pr.Funcs, pr.Refs)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDefinition, protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 1, Character: 5}, // on "getSchedule"
		},
	})
	if err := srv.handleDefinition(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}
	if *result == nil {
		t.Fatal("expected definition result, got nil")
	}
	loc, ok := (*result).(protocol.Location)
	if !ok {
		t.Fatalf("expected Location, got %T", *result)
	}
	if !strings.Contains(string(loc.URI), "service.cfc") {
		t.Errorf("expected service.cfc, got %s", loc.URI)
	}
}

func TestDefinitionMultipleMatchesReturnsAll(t *testing.T) {
	dir := t.TempDir()
	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}

	// Two CFCs with same function name
	_ = os.WriteFile(filepath.Join(dir, "Service1.cfc"), []byte("<cfcomponent>\n<cffunction name=\"doWork\">\n</cffunction>\n</cfcomponent>"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "Service2.cfc"), []byte("<cfcomponent>\n<cffunction name=\"doWork\">\n</cffunction>\n</cfcomponent>"), 0o644)
	srv.index.IndexFile(uri.URI("file://"+filepath.Join(dir, "Service1.cfc")), "<cfcomponent>\n<cffunction name=\"doWork\">\n</cffunction>\n</cfcomponent>")
	srv.index.IndexFile(uri.URI("file://"+filepath.Join(dir, "Service2.cfc")), "<cfcomponent>\n<cffunction name=\"doWork\">\n</cffunction>\n</cfcomponent>")

	// Caller assigns svc via new Service1, so it resolves to Service1
	callerURI := uri.URI("file://" + filepath.Join(dir, "caller.cfm"))
	callerContent := "<cfset svc = new Service1()>\n<cfset result = svc.doWork()>"
	srv.setDocument(callerURI, callerContent)
	srv.index.IndexFile(callerURI, callerContent)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDefinition, protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(callerURI)},
			Position:     protocol.Position{Line: 1, Character: 22}, // on "doWork"
		},
	})
	if err := srv.handleDefinition(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}
	if *result == nil {
		t.Fatal("expected definition result, got nil")
	}
	// Should resolve to Service1 specifically
	loc, ok := (*result).(protocol.Location)
	if !ok {
		t.Fatalf("expected Location (resolved to specific CFC), got %T", *result)
	}
	if !strings.Contains(string(loc.URI), "Service1.cfc") {
		t.Errorf("expected Service1.cfc, got %s", loc.URI)
	}
}

func TestDefinitionPrefersCurrentFile(t *testing.T) {
	srv := newTestServer()
	otherURI := uri.URI("file:///app/Other.cfc")
	srv.index.IndexFile(otherURI, "component {\nfunction helper() {}\n}")

	currentURI := uri.URI("file:///app/Current.cfc")
	currentContent := "component {\nfunction helper() {}\nfunction caller() { helper(); }\n}"
	srv.setDocument(currentURI, currentContent)
	srv.index.IndexFile(currentURI, currentContent)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDefinition, protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(currentURI)},
			Position:     protocol.Position{Line: 2, Character: 22}, // on "helper" in caller()
		},
	})
	if err := srv.handleDefinition(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}
	loc, ok := (*result).(protocol.Location)
	if !ok {
		t.Fatalf("expected Location, got %T", *result)
	}
	if loc.URI != protocol.DocumentURI(currentURI) {
		t.Errorf("expected current file, got %s", loc.URI)
	}
	if loc.Range.Start.Line != 1 {
		t.Errorf("expected line 1, got %d", loc.Range.Start.Line)
	}
}

func TestDefinitionQualifiedCallExcludesCurrentFile(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "Helper.cfc"), []byte("component {\nfunction doStuff() {}\n}"), 0o644)

	srv := newTestServer()
	docURI := uri.URI("file://" + filepath.Join(dir, "caller.cfc"))
	// helper is assigned via createObject, then we call helper.doStuff()
	docContent := "helper = createObject(\"component\",\"Helper\")\nhelper.doStuff()"
	srv.setDocument(docURI, docContent)
	srv.index.IndexFile(docURI, docContent)
	// Also index a doStuff in the current file to verify it's excluded for qualified calls
	srv.index.IndexFile(uri.URI("file://"+filepath.Join(dir, "caller.cfc")), docContent)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDefinition, protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 1, Character: 9}, // on "doStuff"
		},
	})
	if err := srv.handleDefinition(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}
	if *result == nil {
		t.Fatal("expected definition result, got nil")
	}
	loc, ok := (*result).(protocol.Location)
	if !ok {
		t.Fatalf("expected Location, got %T", *result)
	}
	if !strings.Contains(string(loc.URI), "Helper.cfc") {
		t.Errorf("expected Helper.cfc, got %s", loc.URI)
	}
}

func TestDefinitionCfInvokeWithDottedComponent(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "models"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "models", "Widget.cfc"), []byte("<cfcomponent>\n<cffunction name=\"render\">\n</cffunction>\n</cfcomponent>"), 0o644)

	srv := newTestServer()
	docURI := uri.URI("file://" + filepath.Join(dir, "caller.cfm"))
	docContent := `<cfinvoke component="models.Widget" method="render">`
	srv.setDocument(docURI, docContent)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDefinition, protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 0, Character: 46}, // on "render" in method attr
		},
	})
	if err := srv.handleDefinition(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}
	if *result == nil {
		t.Fatal("expected definition result, got nil")
	}
	loc, ok := (*result).(protocol.Location)
	if !ok {
		t.Fatalf("expected Location, got %T", *result)
	}
	if !strings.Contains(string(loc.URI), "Widget.cfc") {
		t.Errorf("expected Widget.cfc, got %s", loc.URI)
	}
}

func TestDefinitionTagFunctionLookup(t *testing.T) {
	srv := newTestServer()
	cfcContent := `<cfcomponent>
<cffunction name="getUser">
	<cfreturn "user">
</cffunction>
<cffunction name="listUsers">
	<cfreturn getUser()>
</cffunction>
</cfcomponent>`
	cfcURI := uri.URI("file:///app/UserService.cfc")
	srv.setDocument(cfcURI, cfcContent)
	srv.index.IndexFile(cfcURI, cfcContent)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDefinition, protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(cfcURI)},
			Position:     protocol.Position{Line: 5, Character: 12}, // on "getUser" in listUsers
		},
	})
	if err := srv.handleDefinition(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}
	if *result == nil {
		t.Fatal("expected definition result, got nil")
	}
	loc, ok := (*result).(protocol.Location)
	if !ok {
		t.Fatalf("expected Location, got %T", *result)
	}
	if loc.Range.Start.Line != 1 {
		t.Errorf("expected line 1, got %d", loc.Range.Start.Line)
	}
}

func TestDefinitionMappingResolution(t *testing.T) {
	dir := t.TempDir()
	modelsDir := filepath.Join(dir, "src", "models")
	_ = os.MkdirAll(modelsDir, 0o755)
	_ = os.WriteFile(filepath.Join(modelsDir, "User.cfc"), []byte("component {\nfunction getName() {}\n}"), 0o644)

	srv := newTestServer()
	srv.Mappings = map[string]string{"models": modelsDir}

	docURI := uri.URI("file://" + filepath.Join(dir, "app", "caller.cfc"))
	docContent := "user = new models.User()\nuser.getName()"
	srv.setDocument(docURI, docContent)
	srv.index.IndexFile(docURI, docContent)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDefinition, protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 1, Character: 6}, // on "getName"
		},
	})
	if err := srv.handleDefinition(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}
	if *result == nil {
		t.Fatal("expected definition result, got nil")
	}
	loc, ok := (*result).(protocol.Location)
	if !ok {
		t.Fatalf("expected Location, got %T", *result)
	}
	if !strings.Contains(string(loc.URI), "User.cfc") {
		t.Errorf("expected User.cfc, got %s", loc.URI)
	}
}

func TestDefinitionCaseInsensitiveFunctionLookup(t *testing.T) {
	dir := t.TempDir()
	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}

	_ = os.WriteFile(filepath.Join(dir, "Service.cfc"), []byte("component {\nfunction getUser() {}\n}"), 0o644)
	cfcURI := uri.URI("file://" + filepath.Join(dir, "Service.cfc"))
	srv.index.IndexFile(cfcURI, "component {\nfunction getUser() {}\n}")

	callerURI := uri.URI("file://" + filepath.Join(dir, "caller.cfm"))
	callerContent := "<cfset svc = new Service()>\n<cfset result = svc.GETUSER()>"
	srv.setDocument(callerURI, callerContent)
	srv.index.IndexFile(callerURI, callerContent)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDefinition, protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(callerURI)},
			Position:     protocol.Position{Line: 1, Character: 20}, // on "GETUSER"
		},
	})
	if err := srv.handleDefinition(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}
	if *result == nil {
		t.Fatal("expected definition result, got nil")
	}
	loc, ok := (*result).(protocol.Location)
	if !ok {
		t.Fatalf("expected Location, got %T", *result)
	}
	if loc.URI != protocol.DocumentURI(cfcURI) {
		t.Errorf("expected Service.cfc, got %s", loc.URI)
	}
}

func TestCompletionDotInvokedTrigger(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "models", "User.cfc"), nil, 0o644)
	_ = os.MkdirAll(filepath.Join(dir, "models"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "models", "User.cfc"), []byte("component {\nfunction getName() {}\n}"), 0o644)

	srv := newTestServer()
	docURI := uri.URI("file://" + filepath.Join(dir, "test.cfm"))
	docContent := "userObj = new models.User()\nuserObj."
	srv.setDocument(docURI, docContent)
	srv.index.IndexFile(docURI, docContent)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 1, Character: 8}, // after "userObj."
		},
		Context: &protocol.CompletionContext{
			TriggerKind: protocol.CompletionTriggerKindInvoked,
		},
	})
	if err := srv.handleCompletion(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}
	list := completionListFromResult(t, *result)
	if len(list.Items) != 1 || list.Items[0].Label != "getName" {
		labels := make([]string, len(list.Items))
		for i, item := range list.Items {
			labels[i] = item.Label
		}
		t.Errorf("expected [getName], got %v", labels)
	}
}

func TestCompletionDotAfterCallExpression(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "packages", "tours"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "packages", "tours", "service.cfc"), []byte("component {\nfunction getToursAndExcursions() {}\nfunction getParameters() {}\n}"), 0o644)

	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}
	srv.ComponentResolvers = []config.Resolver{
		{Match: `getService("$1")`, Resolve: "packages.$1.service", Prefix: "getService"},
	}

	docURI := uri.URI("file://" + filepath.Join(dir, "test.cfm"))
	docContent := `<cfset result = getService("tours").`
	srv.setDocument(docURI, docContent)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 0, Character: 36},
		},
		Context: &protocol.CompletionContext{
			TriggerKind:      protocol.CompletionTriggerKindTriggerCharacter,
			TriggerCharacter: ".",
		},
	})

	if err := srv.handleCompletion(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	list := completionListFromResult(t, *result)
	found := false
	for _, item := range list.Items {
		if item.Label == "getToursAndExcursions" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected completion for getToursAndExcursions after getService(\"tours\").")
	}
}

func TestSignatureHelpQualifiedCall(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "packages", "tours"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "packages", "tours", "service.cfc"), []byte("component {\nfunction getParameters(required string companyCode, string year) {}\n}"), 0o644)

	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}
	srv.ComponentResolvers = []config.Resolver{
		{Match: `getService("$1")`, Resolve: "packages.$1.service", Prefix: "getService"},
	}

	docURI := uri.URI("file://" + filepath.Join(dir, "test.cfm"))
	docContent := `<cfset var svc = getService("tours")>` + "\n" + `<cfset result = svc.getParameters(`
	srv.setDocument(docURI, docContent)

	pr := parser.ParseWithOptions(docURI, string(docContent), parser.ParseOptions{
		Resolvers: []parser.Resolver{{Match: `getService("$1")`, Resolve: "packages.$1.service", Prefix: "getService"}},
	})
	srv.mu.Lock()
	srv.parseResults[docURI] = pr
	srv.mu.Unlock()
	srv.index.IndexFileFromResult(docURI, pr.Funcs, pr.Refs)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentSignatureHelp, protocol.SignatureHelpParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 1, Character: 35},
		},
	})

	if err := srv.handleSignatureHelp(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	help, ok := (*result).(*protocol.SignatureHelp)
	if !ok || help == nil {
		t.Fatalf("expected *SignatureHelp, got %T", *result)
	}
	if len(help.Signatures) == 0 {
		t.Fatal("expected at least one signature")
	}
	if !strings.Contains(help.Signatures[0].Label, "companyCode") {
		t.Errorf("expected signature to contain 'companyCode', got %s", help.Signatures[0].Label)
	}
}

func TestHoverUserDefinedFunction(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "models"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "models", "User.cfc"), []byte("component {\nfunction getName(required string id) {}\n}"), 0o644)

	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}

	docURI := uri.URI("file://" + filepath.Join(dir, "test.cfm"))
	docContent := "myObj = new models.User()\nmyObj.getName()"
	srv.setDocument(docURI, docContent)
	srv.index.IndexFile(docURI, docContent)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentHover, protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 1, Character: 8},
		},
	})

	if err := srv.handleHover(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	hover, ok := (*result).(*protocol.Hover)
	if !ok || hover == nil {
		t.Fatalf("expected *Hover, got %T", *result)
	}
	if !strings.Contains(hover.Contents.Value, "getName") {
		t.Errorf("expected hover to contain 'getName', got %s", hover.Contents.Value)
	}
	if !strings.Contains(hover.Contents.Value, "id") {
		t.Errorf("expected hover to contain parameter 'id', got %s", hover.Contents.Value)
	}
}

func TestSignatureHelpInlineCallExpression(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "packages", "tours"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "packages", "tours", "service.cfc"), []byte("component {\nfunction getParameters(required string companyCode) {}\n}"), 0o644)

	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}
	srv.ComponentResolvers = []config.Resolver{
		{Match: `getService("$1")`, Resolve: "packages.$1.service", Prefix: "getService"},
	}

	docURI := uri.URI("file://" + filepath.Join(dir, "test.cfm"))
	docContent := `<cfset result = getService("tours").getParameters(`
	srv.setDocument(docURI, docContent)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentSignatureHelp, protocol.SignatureHelpParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 0, Character: uint32(len(docContent))},
		},
	})

	if err := srv.handleSignatureHelp(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	help, ok := (*result).(*protocol.SignatureHelp)
	if !ok || help == nil {
		t.Fatalf("expected *SignatureHelp, got %T", *result)
	}
	if len(help.Signatures) == 0 {
		t.Fatal("expected at least one signature")
	}
	if !strings.Contains(help.Signatures[0].Label, "companyCode") {
		t.Errorf("expected signature to contain 'companyCode', got %s", help.Signatures[0].Label)
	}
}

func TestSignatureHelpBuiltinFunction(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///test.cfm")
	srv.setDocument(docURI, "<cfset x = ArrayAppend(arr, ")

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentSignatureHelp, protocol.SignatureHelpParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 0, Character: 28},
		},
	})

	if err := srv.handleSignatureHelp(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	help, ok := (*result).(*protocol.SignatureHelp)
	if !ok || help == nil {
		t.Fatalf("expected *SignatureHelp, got %T", *result)
	}
	if len(help.Signatures) == 0 {
		t.Fatal("expected signature for ArrayAppend")
	}
	if !strings.Contains(strings.ToLower(help.Signatures[0].Label), "arrayappend") {
		t.Errorf("expected label to contain arrayAppend, got %s", help.Signatures[0].Label)
	}
	if help.ActiveParameter != 1 {
		t.Errorf("expected activeParam=1 (after comma), got %d", help.ActiveParameter)
	}
}

func TestSignatureHelpNoContext(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///test.cfm")
	srv.setDocument(docURI, "<cfset x = 123>")

	reply, result, _ := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentSignatureHelp, protocol.SignatureHelpParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 0, Character: 12},
		},
	})

	_ = srv.handleSignatureHelp(context.Background(), reply, req)
	if *result != nil {
		t.Errorf("expected nil result outside function call, got %T", *result)
	}
}

func TestHoverQualifiedCallExpression(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "packages", "general"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "packages", "general", "service.cfc"), []byte("component {\nfunction getYearGroups(required string companyCode) {}\n}"), 0o644)

	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}
	srv.ComponentResolvers = []config.Resolver{
		{Match: `getService("$1")`, Resolve: "packages.$1.service", Prefix: "getService"},
	}

	docURI := uri.URI("file://" + filepath.Join(dir, "test.cfm"))
	docContent := "<cfset var svc = getService(\"general\")>\n<cfset result = svc.getYearGroups()>"
	srv.setDocument(docURI, docContent)

	pr := srv.parseContent(docURI, docContent)
	srv.mu.Lock()
	srv.parseResults[docURI] = pr
	srv.mu.Unlock()
	srv.index.IndexFileFromResult(docURI, pr.Funcs, pr.Refs)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentHover, protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 1, Character: 22},
		},
	})

	if err := srv.handleHover(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	hover, ok := (*result).(*protocol.Hover)
	if !ok || hover == nil {
		t.Fatalf("expected *Hover, got %T", *result)
	}
	if !strings.Contains(hover.Contents.Value, "getYearGroups") {
		t.Errorf("expected hover to contain 'getYearGroups', got %s", hover.Contents.Value)
	}
	if !strings.Contains(hover.Contents.Value, "companyCode") {
		t.Errorf("expected hover to contain 'companyCode', got %s", hover.Contents.Value)
	}
}

func TestDocumentLinkResolve(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "header.cfm"), []byte("header"), 0o644)

	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}

	docURI := uri.URI("file://" + filepath.Join(dir, "test.cfm"))
	srv.setDocument(docURI, `<cfinclude template="header.cfm">`)

	// Test documentLink request
	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDocumentLink, protocol.DocumentLinkParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
	})

	if err := srv.handleDocumentLink(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	links, ok := (*result).([]protocol.DocumentLink)
	if !ok || len(links) == 0 {
		t.Fatal("expected at least one document link")
	}
	if links[0].Tooltip != "header.cfm" {
		t.Errorf("expected tooltip 'header.cfm', got %q", links[0].Tooltip)
	}

	// Test resolve
	reply2, result2, replyErr2 := captureReply(t)
	resolveReq := makeCall(t, protocol.MethodDocumentLinkResolve, links[0])
	if err := srv.handleDocumentLinkResolve(context.Background(), reply2, resolveReq); err != nil {
		t.Fatal(err)
	}
	if *replyErr2 != nil {
		t.Fatal(*replyErr2)
	}

	resolved, ok := (*result2).(protocol.DocumentLink)
	if !ok {
		t.Fatalf("expected DocumentLink, got %T", *result2)
	}
	if resolved.Target == "" {
		t.Error("expected resolved target to be non-empty")
	}
	if !strings.Contains(string(resolved.Target), "header.cfm") {
		t.Errorf("expected target to contain 'header.cfm', got %s", resolved.Target)
	}
}

func TestResolveComponentPathCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "models"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "models", "user.cfc"), []byte("component {}"), 0o644)

	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}

	// Request with uppercase User — should still resolve to lowercase user.cfc
	cfcPath := srv.getResolver().ComponentPath("models.User", dir)
	if cfcPath == "" {
		t.Fatal("expected to resolve models.User")
	}
	if !strings.HasSuffix(cfcPath, "user.cfc") {
		t.Errorf("expected path to end with 'user.cfc', got %s", cfcPath)
	}
}

func TestSignatureHelpActiveParamMultiple(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///test.cfm")
	srv.setDocument(docURI, "<cfset x = Replace(str, find, repl, ")

	reply, result, _ := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentSignatureHelp, protocol.SignatureHelpParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 0, Character: 36},
		},
	})
	_ = srv.handleSignatureHelp(context.Background(), reply, req)
	help := (*result).(*protocol.SignatureHelp)
	if help.ActiveParameter != 3 {
		t.Errorf("expected activeParam=3, got %d", help.ActiveParameter)
	}
}

func TestSignatureHelpNestedCall(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///test.cfm")
	// Cursor inside Len( — should show Len signature, not ArrayAppend
	srv.setDocument(docURI, "<cfset x = ArrayAppend(arr, Len(")

	reply, result, _ := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentSignatureHelp, protocol.SignatureHelpParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 0, Character: 32},
		},
	})
	_ = srv.handleSignatureHelp(context.Background(), reply, req)
	help := (*result).(*protocol.SignatureHelp)
	if help == nil || len(help.Signatures) == 0 {
		t.Fatal("expected signature")
	}
	if !strings.Contains(strings.ToLower(help.Signatures[0].Label), "len") {
		t.Errorf("expected Len signature, got %s", help.Signatures[0].Label)
	}
}

func TestCompletionDotAfterVariableRef(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "packages", "general"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "packages", "general", "service.cfc"), []byte("component {\nfunction getYearGroups() {}\nfunction getCompanies() {}\n}"), 0o644)

	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}
	srv.ComponentResolvers = []config.Resolver{
		{Match: `getService("$1")`, Resolve: "packages.$1.service", Prefix: "getService"},
	}

	docURI := uri.URI("file://" + filepath.Join(dir, "test.cfm"))
	docContent := "svc = getService(\"general\")\nsvc."
	srv.setDocument(docURI, docContent)

	pr := srv.parseContent(docURI, docContent)
	srv.mu.Lock()
	srv.parseResults[docURI] = pr
	srv.mu.Unlock()
	srv.index.IndexFileFromResult(docURI, pr.Funcs, pr.Refs)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 1, Character: 4},
		},
		Context: &protocol.CompletionContext{
			TriggerKind:      protocol.CompletionTriggerKindTriggerCharacter,
			TriggerCharacter: ".",
		},
	})

	if err := srv.handleCompletion(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	list := completionListFromResult(t, *result)
	var names []string
	for _, item := range list.Items {
		names = append(names, item.Label)
	}
	if !strings.Contains(strings.Join(names, ","), "getYearGroups") {
		t.Errorf("expected getYearGroups in completions, got %v", names)
	}
	if !strings.Contains(strings.Join(names, ","), "getCompanies") {
		t.Errorf("expected getCompanies in completions, got %v", names)
	}
}

func TestDocumentLinkSkipsHashExpressions(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///test.cfm")
	srv.setDocument(docURI, `<cfinclude template="#dynamicPath#">`)

	reply, result, _ := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDocumentLink, protocol.DocumentLinkParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
	})
	_ = srv.handleDocumentLink(context.Background(), reply, req)

	links, _ := (*result).([]protocol.DocumentLink)
	if len(links) != 0 {
		t.Errorf("expected no links for hash expression, got %d", len(links))
	}
}

func TestDocumentLinkSkipsURLs(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///test.cfm")
	srv.setDocument(docURI, `<a href="https://example.com">link</a>`)

	reply, result, _ := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDocumentLink, protocol.DocumentLinkParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
	})
	_ = srv.handleDocumentLink(context.Background(), reply, req)

	links, _ := (*result).([]protocol.DocumentLink)
	if len(links) != 0 {
		t.Errorf("expected no links for URL, got %d", len(links))
	}
}

func TestResolverSingleQuotesMatch(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "packages", "teacher"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "packages", "teacher", "service.cfc"), []byte("component {\nfunction getTeachers() {}\n}"), 0o644)

	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}
	srv.ComponentResolvers = []config.Resolver{
		{Match: `getService("$1")`, Resolve: "packages.$1.service", Prefix: "getService"},
	}

	docURI := uri.URI("file://" + filepath.Join(dir, "test.cfm"))
	// Single quotes in source, double quotes in resolver pattern
	docContent := "svc = getService('teacher')\nsvc."
	srv.setDocument(docURI, docContent)

	pr := srv.parseContent(docURI, docContent)
	srv.mu.Lock()
	srv.parseResults[docURI] = pr
	srv.mu.Unlock()
	srv.index.IndexFileFromResult(docURI, pr.Funcs, pr.Refs)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 1, Character: 4},
		},
		Context: &protocol.CompletionContext{
			TriggerKind:      protocol.CompletionTriggerKindTriggerCharacter,
			TriggerCharacter: ".",
		},
	})

	if err := srv.handleCompletion(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	list := completionListFromResult(t, *result)
	found := false
	for _, item := range list.Items {
		if item.Label == "getTeachers" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected getTeachers in completions — single quotes should match double-quote pattern")
	}
}

func TestExecuteCommandReindex(t *testing.T) {
	srv := newTestServer()
	reply, _, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodWorkspaceExecuteCommand, protocol.ExecuteCommandParams{
		Command: "cfmleditor.reindex",
	})
	if err := srv.handleExecuteCommand(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}
}

func TestExecuteCommandCopyPackage(t *testing.T) {
	srv := newTestServer()
	srv.WorkspaceFolders = []string{"/project"}

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodWorkspaceExecuteCommand, protocol.ExecuteCommandParams{
		Command:   "cfmleditor.copyPackage",
		Arguments: []interface{}{"file:///project/models/User.cfc"},
	})
	if err := srv.handleExecuteCommand(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}
	dotPath, _ := (*result).(string)
	if dotPath != "models.User" {
		t.Errorf("expected 'models.User', got %q", dotPath)
	}
}

func TestHoverBuiltinCaseInsensitive(t *testing.T) {
	srv := newTestServer()
	srv.setDocument(uri.URI("file:///test.cfm"), "<cfset x = ARRAYAPPEND(arr, val)>")

	reply, result, _ := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentHover, protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.cfm"},
			Position:     protocol.Position{Line: 0, Character: 15},
		},
	})
	_ = srv.handleHover(context.Background(), reply, req)
	hover, ok := (*result).(*protocol.Hover)
	if !ok || hover == nil {
		t.Fatal("expected hover for uppercase ARRAYAPPEND")
	}
	if !strings.Contains(strings.ToLower(hover.Contents.Value), "arrayappend") {
		t.Errorf("expected arrayappend in hover, got %s", hover.Contents.Value)
	}
}

func TestDefinitionViaRegexResolver(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "packages", "finance"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "packages", "finance", "service.cfc"), []byte("component {\nfunction getReport() {}\n}"), 0o644)

	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}
	srv.ComponentResolvers = []config.Resolver{
		{Match: `kernel\.get([A-Za-z0-9_]+)\(\)`, Resolve: "packages.$1.service", Prefix: "kernel.get"},
	}

	docURI := uri.URI("file://" + filepath.Join(dir, "test.cfm"))
	docContent := "svc = SERVER.kernel.getFinance()\nsvc.getReport()"
	srv.setDocument(docURI, docContent)

	pr := srv.parseContent(docURI, docContent)
	srv.mu.Lock()
	srv.parseResults[docURI] = pr
	srv.mu.Unlock()
	srv.index.IndexFileFromResult(docURI, pr.Funcs, pr.Refs)

	// Check that svc resolved to finance
	ref := srv.index.LookupComponentRefInFile("svc", docURI, 1)
	if ref == nil {
		t.Fatal("expected component ref for svc")
	}
	if !strings.Contains(strings.ToLower(ref.Component), "finance") {
		t.Errorf("expected component to contain 'finance', got %s", ref.Component)
	}
}

func TestCompletionDotOnThis(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///test.cfc")
	docContent := "component {\nfunction init() {}\nfunction getData() {}\n}\nthis."
	srv.setDocument(docURI, docContent)

	pr := srv.parseContent(docURI, docContent)
	srv.mu.Lock()
	srv.parseResults[docURI] = pr
	srv.mu.Unlock()
	srv.index.IndexFileFromResult(docURI, pr.Funcs, pr.Refs)
	srv.index.SetThisVars(docURI, pr.ThisVars())

	// Rebuild completion cache
	srv.rebuildFileCompletionCacheFromPR(docURI, pr)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 4, Character: 5},
		},
		Context: &protocol.CompletionContext{
			TriggerKind:      protocol.CompletionTriggerKindTriggerCharacter,
			TriggerCharacter: ".",
		},
	})

	if err := srv.handleCompletion(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	list := completionListFromResult(t, *result)
	var names []string
	for _, item := range list.Items {
		names = append(names, item.Label)
	}
	joined := strings.Join(names, ",")
	if !strings.Contains(joined, "init") || !strings.Contains(joined, "getData") {
		t.Errorf("expected this. to show init and getData, got %v", names)
	}
}

func TestDocumentLinkMultipleOnSameLine(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///test.cfm")
	srv.setDocument(docURI, `<cfinclude template="a.cfm"><cfinclude template="b.cfm">`)

	reply, result, _ := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDocumentLink, protocol.DocumentLinkParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
	})
	_ = srv.handleDocumentLink(context.Background(), reply, req)

	links, _ := (*result).([]protocol.DocumentLink)
	if len(links) != 2 {
		t.Errorf("expected 2 links, got %d", len(links))
	}
}

func TestFuncRefsLazyExtraction(t *testing.T) {
	content := `<cfcomponent>
	<cffunction name="init">
		<cfset VARIABLES.svc = getService("general") />
	</cffunction>
	<cffunction name="doWork">
		<cfset var helper = getService("helper") />
	</cffunction>
</cfcomponent>`

	resolvers := []parser.Resolver{
		{Match: `getService("$1")`, Resolve: "packages.$1.service", Prefix: "getService"},
	}
	pr := parser.ParseWithOptions(uri.URI("file:///test.cfc"), content, parser.ParseOptions{
		Resolvers: resolvers,
	})

	// init refs should be in pr.Refs (eagerly scanned)
	found := false
	for _, ref := range pr.Refs {
		if ref.Variable == "svc" && strings.Contains(ref.Component, "general") {
			found = true
		}
	}
	if !found {
		t.Error("expected eager ref for svc in init()")
	}

	// doWork refs should NOT be in pr.Refs (lazy)
	for _, ref := range pr.Refs {
		if ref.Variable == "helper" {
			t.Error("did not expect eager ref for helper in doWork() — should be lazy")
		}
	}

	// But FuncRefs should find it
	for _, sc := range pr.Scopes {
		for _, f := range pr.Funcs {
			if f.Name == "doWork" && int(f.Line) == sc.Start {
				refs, _ := pr.FuncRefs(sc.Start, sc.End)
				found = false
				for _, ref := range refs {
					if ref.Variable == "helper" && strings.Contains(ref.Component, "helper") {
						found = true
					}
				}
				if !found {
					t.Error("expected lazy ref for helper via FuncRefs")
				}
			}
		}
	}
}

func TestExtractLinksFromContent(t *testing.T) {
	content := `<cfinclude template="header.cfm">
<a href="page.html">link</a>
<cfmodule template="mod.cfm">
<cfinclude template="#dynamic#">`

	links := parser.ExtractLinks(content)
	if len(links) != 3 {
		t.Errorf("expected 3 links (header, page, mod), got %d", len(links))
		for _, l := range links {
			t.Logf("  %s line=%d", l.Path, l.Line)
		}
	}
}

func TestExecuteCommandUnknown(t *testing.T) {
	srv := newTestServer()
	reply, _, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodWorkspaceExecuteCommand, protocol.ExecuteCommandParams{
		Command: "cfmleditor.nonexistent",
	})
	_ = srv.handleExecuteCommand(context.Background(), reply, req)
	if *replyErr == nil {
		t.Error("expected error for unknown command")
	}
}

func TestInvalidateAllCache(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///test.cfc")
	srv.compCache.PutFile(docURI, []protocol.CompletionItem{{Label: "test"}})

	items := srv.compCache.GetFile(docURI)
	if len(items) == 0 {
		t.Fatal("expected cached items")
	}

	srv.compCache.InvalidateAll()
	items = srv.compCache.GetFile(docURI)
	if len(items) != 0 {
		t.Errorf("expected empty after InvalidateAll, got %d", len(items))
	}
}

func TestSignatureHelpUserFunctionInSameFile(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///test.cfc")
	docContent := "component {\nfunction myHelper(required string name, numeric age) {}\nfunction init() {\nmyHelper(\n}\n}"
	srv.setDocument(docURI, docContent)
	srv.index.IndexFile(docURI, docContent)

	reply, result, _ := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentSignatureHelp, protocol.SignatureHelpParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 3, Character: 9},
		},
	})
	_ = srv.handleSignatureHelp(context.Background(), reply, req)
	help, ok := (*result).(*protocol.SignatureHelp)
	if !ok || help == nil || len(help.Signatures) == 0 {
		t.Fatal("expected signature for myHelper")
	}
	if !strings.Contains(help.Signatures[0].Label, "name") {
		t.Errorf("expected 'name' param, got %s", help.Signatures[0].Label)
	}
	if len(help.Signatures[0].Parameters) != 2 {
		t.Errorf("expected 2 parameters, got %d", len(help.Signatures[0].Parameters))
	}
}

func TestHoverNoResultForUnknownWord(t *testing.T) {
	srv := newTestServer()
	srv.setDocument(uri.URI("file:///test.cfm"), "<cfset xyz123 = 1>")

	reply, result, _ := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentHover, protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.cfm"},
			Position:     protocol.Position{Line: 0, Character: 8},
		},
	})
	_ = srv.handleHover(context.Background(), reply, req)
	if *result != nil {
		t.Errorf("expected nil hover for unknown word, got %T", *result)
	}
}

func TestCompletionDotAfterNewExpression(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "models"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "models", "User.cfc"), []byte("component {\nfunction getName() {}\nfunction getAge() {}\n}"), 0o644)

	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}
	docURI := uri.URI("file://" + filepath.Join(dir, "test.cfm"))
	docContent := "obj = new models.User()\nobj."
	srv.setDocument(docURI, docContent)
	srv.index.IndexFile(docURI, docContent)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 1, Character: 4},
		},
		Context: &protocol.CompletionContext{
			TriggerKind:      protocol.CompletionTriggerKindTriggerCharacter,
			TriggerCharacter: ".",
		},
	})
	if err := srv.handleCompletion(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}
	list := completionListFromResult(t, *result)
	var names []string
	for _, item := range list.Items {
		names = append(names, item.Label)
	}
	if !strings.Contains(strings.Join(names, ","), "getName") {
		t.Errorf("expected getName after new models.User(), got %v", names)
	}
}

func TestDocumentLinkHrefAndAction(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///test.cfm")
	srv.setDocument(docURI, `<a href="about.cfm">About</a>
<form action="submit.cfm">`)

	reply, result, _ := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDocumentLink, protocol.DocumentLinkParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
	})
	_ = srv.handleDocumentLink(context.Background(), reply, req)

	links, _ := (*result).([]protocol.DocumentLink)
	if len(links) != 2 {
		t.Errorf("expected 2 links (href + action), got %d", len(links))
	}
	paths := make(map[string]bool)
	for _, l := range links {
		paths[l.Tooltip] = true
	}
	if !paths["about.cfm"] {
		t.Error("expected link for about.cfm")
	}
	if !paths["submit.cfm"] {
		t.Error("expected link for submit.cfm")
	}
}

func TestResolverMultipleCaptures(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "packages", "admin", "dao"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "packages", "admin", "dao", "UserDAO.cfc"), []byte("component {\nfunction findAll() {}\n}"), 0o644)

	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}
	srv.ComponentResolvers = []config.Resolver{
		{Match: `getBean("$1", "$2")`, Resolve: "packages.$2.dao.$1", Prefix: "getBean"},
	}

	docURI := uri.URI("file://" + filepath.Join(dir, "test.cfm"))
	docContent := "dao = getBean(\"UserDAO\", \"admin\")\ndao."
	srv.setDocument(docURI, docContent)

	pr := srv.parseContent(docURI, docContent)
	srv.mu.Lock()
	srv.parseResults[docURI] = pr
	srv.mu.Unlock()
	srv.index.IndexFileFromResult(docURI, pr.Funcs, pr.Refs)

	ref := srv.index.LookupComponentRefInFile("dao", docURI, 1)
	if ref == nil {
		t.Fatal("expected ref for dao")
	}
	if ref.Component != "packages.admin.dao.UserDAO" {
		t.Errorf("expected packages.admin.dao.UserDAO, got %s", ref.Component)
	}
}

func TestFindCallContextNoParens(t *testing.T) {
	content := "<cfset x = someVar>"
	name, qual, _ := parser.FindCallContext(content, 0, 15)
	if name != "" || qual != "" {
		t.Errorf("expected empty outside parens, got name=%q qual=%q", name, qual)
	}
}

func TestWordAtPositionEdgeCases(t *testing.T) {
	tests := []struct {
		content string
		line    int
		char    int
		want    string
	}{
		{"hello world", 0, 0, "hello"},
		{"hello world", 0, 5, "hello"},
		{"hello world", 0, 6, "world"},
		{"", 0, 0, ""},
		{"a", 0, 1, "a"},
		{"foo.bar", 0, 1, "foo"},
	}
	for _, tt := range tests {
		got := parser.WordAtPosition(tt.content, tt.line, tt.char)
		if got != tt.want {
			t.Errorf("parser.WordAtPosition(%q, %d, %d) = %q, want %q", tt.content, tt.line, tt.char, got, tt.want)
		}
	}
}

func TestSignatureHelpAfterSecondComma(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///test.cfm")
	srv.setDocument(docURI, `<cfset x = ListAppend(list, val, `)

	reply, result, _ := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentSignatureHelp, protocol.SignatureHelpParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 0, Character: 33},
		},
	})
	_ = srv.handleSignatureHelp(context.Background(), reply, req)
	help := (*result).(*protocol.SignatureHelp)
	if help == nil || help.ActiveParameter != 2 {
		t.Errorf("expected activeParam=2, got %v", help)
	}
}

func TestCompletionDotAfterCreateObject(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "models"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "models", "Order.cfc"), []byte("component {\nfunction getTotal() {}\n}"), 0o644)

	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}
	docURI := uri.URI("file://" + filepath.Join(dir, "test.cfm"))
	docContent := "obj = createObject('component','models.Order')\nobj."
	srv.setDocument(docURI, docContent)
	srv.index.IndexFile(docURI, docContent)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 1, Character: 4},
		},
		Context: &protocol.CompletionContext{TriggerKind: protocol.CompletionTriggerKindTriggerCharacter, TriggerCharacter: "."},
	})
	if err := srv.handleCompletion(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}
	list := completionListFromResult(t, *result)
	found := false
	for _, item := range list.Items {
		if item.Label == "getTotal" {
			found = true
		}
	}
	if !found {
		t.Error("expected getTotal after createObject dot completion")
	}
}

func TestDefinitionFallsBackToGlobalLookup(t *testing.T) {
	srv := newTestServer()
	srv.GlobalFunctionResolution = true
	docURI := uri.URI("file:///test.cfm")
	otherURI := uri.URI("file:///other.cfc")
	// svc has no resolved ref — qualified fallback finds myFunc globally
	callerContent := "<cfset result = svc.myFunc()>"
	srv.setDocument(docURI, callerContent)
	srv.index.IndexFile(docURI, callerContent)
	srv.index.IndexFileFromResult(otherURI, []parser.FunctionDef{
		{Name: "myFunc", URI: otherURI, Line: 10},
	}, nil)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDefinition, protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 0, Character: 22},
		},
	})
	if err := srv.handleDefinition(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}
	loc, ok := (*result).(protocol.Location)
	if !ok {
		t.Fatalf("expected Location, got %T", *result)
	}
	if loc.URI != protocol.DocumentURI(otherURI) {
		t.Errorf("expected URI %s, got %s", otherURI, loc.URI)
	}
	if loc.Range.Start.Line != 10 {
		t.Errorf("expected line 10, got %d", loc.Range.Start.Line)
	}
}

func TestDocumentLinkInsideFunction(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///test.cfc")
	docContent := "component {\nfunction render() {\n<cfinclude template=\"partial.cfm\">\n}\n}"
	srv.setDocument(docURI, docContent)

	pr := srv.parseContent(docURI, docContent)
	srv.mu.Lock()
	srv.parseResults[docURI] = pr
	srv.mu.Unlock()

	reply, result, _ := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDocumentLink, protocol.DocumentLinkParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
	})
	_ = srv.handleDocumentLink(context.Background(), reply, req)

	links, _ := (*result).([]protocol.DocumentLink)
	if len(links) == 0 {
		t.Error("expected link inside function body via FuncRefs")
	}
}

func TestEnsureIndexedLoadsFromDisk(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "models"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "models", "Item.cfc"), []byte("component {\nfunction getPrice() {}\nfunction getName() {}\n}"), 0o644)

	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}

	cfcPath := filepath.Join(dir, "models", "Item.cfc")
	defs := srv.getResolver().EnsureIndexed(cfcPath)
	if len(defs) != 2 {
		t.Errorf("expected 2 functions from disk, got %d", len(defs))
	}

	// Second call should use cache
	defs2 := srv.getResolver().EnsureIndexed(cfcPath)
	if len(defs2) != 2 {
		t.Errorf("expected 2 functions from cache, got %d", len(defs2))
	}
}

func TestResolveComponentPathWithMappings(t *testing.T) {
	dir := t.TempDir()
	libDir := filepath.Join(dir, "lib")
	_ = os.MkdirAll(libDir, 0o755)
	_ = os.WriteFile(filepath.Join(libDir, "Utils.cfc"), []byte("component {}"), 0o644)

	srv := newTestServer()
	srv.Mappings = map[string]string{"mylib": libDir}

	result := srv.getResolver().ComponentPath("mylib.Utils", dir)
	if result == "" {
		t.Fatal("expected to resolve mylib.Utils via mapping")
	}
	if !strings.HasSuffix(result, "Utils.cfc") {
		t.Errorf("expected path ending in Utils.cfc, got %s", result)
	}
}

func TestHandleDidOpenNonCFML(t *testing.T) {
	srv := newTestServer()
	reply, _, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        "file:///test.js",
			LanguageID: "javascript",
			Version:    1,
			Text:       "const x = 1;",
		},
	})
	if err := srv.handleDidOpen(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}
	// Should not be stored
	if _, ok := srv.getDocument(uri.URI("file:///test.js")); ok {
		t.Error("non-CFML file should not be stored in documents")
	}
}

func TestResolveMappings(t *testing.T) {
	result := cfpath.ResolveMappings(map[string]string{
		"models": "./src/models",
		"lib":    "/absolute/lib",
	}, "/project")

	if result["models"] != "/project/src/models" {
		t.Errorf("models: got %q, want /project/src/models", result["models"])
	}
	if result["lib"] != "/absolute/lib" {
		t.Errorf("lib: got %q, want /absolute/lib", result["lib"])
	}
}

func TestHoverUnqualifiedUserFunction(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///test.cfc")
	docContent := "component {\nfunction doStuff(required string id, boolean flag) {}\n}\ndoStuff()"
	srv.setDocument(docURI, docContent)
	srv.index.IndexFile(docURI, docContent)

	reply, result, _ := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentHover, protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 3, Character: 3},
		},
	})
	_ = srv.handleHover(context.Background(), reply, req)
	hover, ok := (*result).(*protocol.Hover)
	if !ok || hover == nil {
		t.Fatal("expected hover for unqualified user function")
	}
	if !strings.Contains(hover.Contents.Value, "flag") {
		t.Errorf("expected 'flag' param in hover, got %s", hover.Contents.Value)
	}
}

func TestCompletionClosingTagSlash(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///test.cfm")
	srv.setDocument(docURI, "<cfoutput></")

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 0, Character: 12},
		},
		Context: &protocol.CompletionContext{TriggerKind: protocol.CompletionTriggerKindTriggerCharacter, TriggerCharacter: "/"},
	})
	if err := srv.handleCompletion(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}
	list := completionListFromResult(t, *result)
	found := false
	for _, item := range list.Items {
		if strings.Contains(item.Label, "cfoutput") {
			found = true
		}
	}
	if !found {
		t.Error("expected closing tag completion for cfoutput")
	}
}

func TestDefinitionPrefersSameFile(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///main.cfc")
	otherURI := uri.URI("file:///other.cfc")
	srv.setDocument(docURI, "component {\nfunction helper() {}\n}\nhelper()")
	srv.index.IndexFileFromResult(docURI, []parser.FunctionDef{
		{Name: "helper", URI: docURI, Line: 1},
	}, nil)
	srv.index.IndexFileFromResult(otherURI, []parser.FunctionDef{
		{Name: "helper", URI: otherURI, Line: 50},
	}, nil)

	reply, result, _ := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDefinition, protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 3, Character: 3},
		},
	})
	_ = srv.handleDefinition(context.Background(), reply, req)
	loc, ok := (*result).(protocol.Location)
	if !ok {
		t.Fatalf("expected Location, got %T", *result)
	}
	if loc.URI != protocol.DocumentURI(docURI) {
		t.Error("expected definition to prefer same file")
	}
}

func TestDocumentSymbolBasic(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///test.cfc")
	docContent := "component {\nfunction init() {}\nfunction getData() {}\n}"
	srv.setDocument(docURI, docContent)
	srv.index.IndexFile(docURI, docContent)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDocumentSymbol, protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
	})
	if err := srv.handleDocumentSymbol(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}
	symbols, ok := (*result).([]protocol.DocumentSymbol)
	if !ok {
		// Might be SymbolInformation
		syms, ok2 := (*result).([]protocol.SymbolInformation)
		if !ok2 || len(syms) < 2 {
			t.Fatalf("expected at least 2 symbols, got %T", *result)
		}
		return
	}
	if len(symbols) < 2 {
		t.Errorf("expected at least 2 symbols, got %d", len(symbols))
	}
}

func TestWorkspaceSymbolQuery(t *testing.T) {
	srv := newTestServer()
	srv.index.IndexFileFromResult(uri.URI("file:///a.cfc"), []parser.FunctionDef{
		{Name: "getUserById", URI: "file:///a.cfc", Line: 5},
		{Name: "deleteUser", URI: "file:///a.cfc", Line: 20},
	}, nil)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodWorkspaceSymbol, protocol.WorkspaceSymbolParams{Query: "getUser"})
	if err := srv.handleWorkspaceSymbol(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}
	symbols, ok := (*result).([]protocol.SymbolInformation)
	if !ok || len(symbols) == 0 {
		t.Fatal("expected at least one workspace symbol")
	}
	if symbols[0].Name != "getUserById" {
		t.Errorf("expected getUserById, got %s", symbols[0].Name)
	}
}

func TestIsCFMLFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"file:///test.cfc", true},
		{"file:///test.cfm", true},
		{"file:///test.cfml", true},
		{"file:///test.cfs", true},
		{"file:///test.CFC", true},
		{"file:///test.js", false},
		{"file:///test.go", false},
		{"abc", false},
	}
	for _, tt := range tests {
		if got := isCFMLFile(tt.path); got != tt.want {
			t.Errorf("isCFMLFile(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestIsCFCFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"file:///test.cfc", true},
		{"file:///test.CFC", true},
		{"file:///test.cfm", false},
		{"file:///test.js", false},
	}
	for _, tt := range tests {
		if got := isCFCFile(tt.path); got != tt.want {
			t.Errorf("isCFCFile(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestResolveComponentPathNotFound(t *testing.T) {
	srv := newTestServer()
	srv.WorkspaceFolders = []string{"/nonexistent"}
	result := srv.getResolver().ComponentPath("no.Such.Component", "/tmp")
	if result != "" {
		t.Errorf("expected empty for nonexistent component, got %s", result)
	}
}

func TestDocumentLinkEmptyDocument(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///empty.cfm")
	srv.setDocument(docURI, "")

	reply, result, _ := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDocumentLink, protocol.DocumentLinkParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
	})
	_ = srv.handleDocumentLink(context.Background(), reply, req)
	links, _ := (*result).([]protocol.DocumentLink)
	if len(links) != 0 {
		t.Errorf("expected no links for empty doc, got %d", len(links))
	}
}

func TestSignatureHelpEmptyDocument(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///empty.cfm")
	srv.setDocument(docURI, "")

	reply, result, _ := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentSignatureHelp, protocol.SignatureHelpParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 0, Character: 0},
		},
	})
	_ = srv.handleSignatureHelp(context.Background(), reply, req)
	if *result != nil {
		t.Errorf("expected nil for empty doc, got %T", *result)
	}
}

func TestHoverEmptyDocument(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///empty.cfm")
	srv.setDocument(docURI, "")

	reply, result, _ := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentHover, protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 0, Character: 0},
		},
	})
	_ = srv.handleHover(context.Background(), reply, req)
	if *result != nil {
		t.Errorf("expected nil for empty doc, got %T", *result)
	}
}

func TestDefinitionEmptyWord(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///test.cfm")
	srv.setDocument(docURI, "   ")

	reply, result, _ := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDefinition, protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 0, Character: 1},
		},
	})
	_ = srv.handleDefinition(context.Background(), reply, req)
	if *result != nil {
		t.Errorf("expected nil for whitespace, got %T", *result)
	}
}

func TestExecuteCommandShowResolvers(t *testing.T) {
	srv := newTestServer()
	srv.Mappings = map[string]string{"models": "/app/models"}
	srv.ComponentResolvers = []config.Resolver{
		{Match: `getService("$1")`, Resolve: "packages.$1.service", Prefix: "getService"},
	}

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodWorkspaceExecuteCommand, protocol.ExecuteCommandParams{
		Command: "cfmleditor.showResolvers",
	})
	if err := srv.handleExecuteCommand(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}
	msg, _ := (*result).(string)
	if !strings.Contains(msg, "models") {
		t.Errorf("expected mappings in output, got %s", msg)
	}
	if !strings.Contains(msg, "getService") {
		t.Errorf("expected resolver in output, got %s", msg)
	}
}

func TestExecuteCommandShowFileIndex(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///test.cfc")
	srv.index.IndexFileFromResult(docURI, []parser.FunctionDef{
		{Name: "init", URI: docURI, Line: 1},
		{Name: "getData", URI: docURI, Line: 5},
	}, nil)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodWorkspaceExecuteCommand, protocol.ExecuteCommandParams{
		Command:   "cfmleditor.showFileIndex",
		Arguments: []interface{}{"file:///test.cfc"},
	})
	if err := srv.handleExecuteCommand(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}
	msg, _ := (*result).(string)
	if !strings.Contains(msg, "init") || !strings.Contains(msg, "getData") {
		t.Errorf("expected function names in output, got %s", msg)
	}
}

func TestSimpleMatchExactNoPlaceholder(t *testing.T) {
	resolvers := []parser.Resolver{
		{Match: "_parent", Resolve: "packages.core.kernel", Prefix: "_parent"},
	}
	got := parser.ResolveFromCall("_parent", resolvers)
	if got != "packages.core.kernel" {
		t.Errorf("expected packages.core.kernel, got %q", got)
	}
}

func TestSimpleMatchNoMatch(t *testing.T) {
	resolvers := []parser.Resolver{
		{Match: `getService("$1")`, Resolve: "packages.$1.service", Prefix: "getService"},
	}
	got := parser.ResolveFromCall("somethingElse()", resolvers)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestExpandGlobSimplePattern(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.cfc"), []byte(""), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "b.txt"), []byte(""), 0o644)

	matches := cfpath.ExpandGlob(filepath.Join(dir, "*.cfc"))
	if len(matches) != 1 {
		t.Errorf("expected 1 match, got %d: %v", len(matches), matches)
	}
}

func TestExpandGlobDoubleStarPattern(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	_ = os.MkdirAll(sub, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "top.cfc"), []byte(""), 0o644)
	_ = os.WriteFile(filepath.Join(sub, "deep.cfc"), []byte(""), 0o644)
	_ = os.WriteFile(filepath.Join(sub, "skip.txt"), []byte(""), 0o644)

	matches := cfpath.ExpandGlob(dir + "/**/*.cfc")
	if len(matches) != 2 {
		t.Errorf("expected 2 matches, got %d: %v", len(matches), matches)
	}
}

func TestHoverQualifiedOverridesBuiltin(t *testing.T) {
	// "len" is a builtin, but widget.len() should show the user-defined function
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "models"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "models", "Widget.cfc"), []byte("component {\nfunction len(required string input) {}\n}"), 0o644)

	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}
	docURI := uri.URI("file://" + filepath.Join(dir, "test.cfm"))
	docContent := "widget = new models.Widget()\nwidget.len()"
	srv.setDocument(docURI, docContent)
	srv.index.IndexFile(docURI, docContent)

	reply, result, _ := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentHover, protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 1, Character: 8},
		},
	})
	_ = srv.handleHover(context.Background(), reply, req)
	hover, ok := (*result).(*protocol.Hover)
	if !ok || hover == nil {
		t.Fatal("expected hover result")
	}
	// Should show user-defined len with "input" param, not the builtin Len
	if !strings.Contains(hover.Contents.Value, "input") {
		t.Errorf("expected user-defined len(input), got builtin: %s", hover.Contents.Value)
	}
}

func TestHoverUnqualifiedShowsBuiltin(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///test.cfm")
	srv.setDocument(docURI, "<cfset x = Len(y)>")

	reply, result, _ := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentHover, protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.cfm"},
			Position:     protocol.Position{Line: 0, Character: 12},
		},
	})
	_ = srv.handleHover(context.Background(), reply, req)
	hover, ok := (*result).(*protocol.Hover)
	if !ok || hover == nil {
		t.Fatal("expected hover for builtin Len")
	}
	// Should show builtin, not a user function
	if !strings.Contains(strings.ToLower(hover.Contents.Value), "len") {
		t.Errorf("expected builtin len hover, got %s", hover.Contents.Value)
	}
	// Should NOT contain user-defined params
	if strings.Contains(hover.Contents.Value, "input") {
		t.Error("should show builtin, not user-defined")
	}
}

func TestHoverMultipleMatchesNoQualifier(t *testing.T) {
	// Same function name in two files, no qualifier — should NOT show hover (ambiguous)
	srv := newTestServer()
	docURI := uri.URI("file:///test.cfm")
	srv.setDocument(docURI, "getData()")
	srv.index.IndexFileFromResult(uri.URI("file:///a.cfc"), []parser.FunctionDef{
		{Name: "getData", URI: "file:///a.cfc", Line: 1},
	}, nil)
	srv.index.IndexFileFromResult(uri.URI("file:///b.cfc"), []parser.FunctionDef{
		{Name: "getData", URI: "file:///b.cfc", Line: 5},
	}, nil)

	reply, result, _ := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentHover, protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 0, Character: 3},
		},
	})
	_ = srv.handleHover(context.Background(), reply, req)
	if *result != nil {
		t.Error("expected nil hover for ambiguous unqualified function")
	}
}

func TestHoverSingleGlobalMatch(t *testing.T) {
	// Only one match globally — should show hover even without qualifier
	srv := newTestServer()
	srv.GlobalFunctionResolution = true
	docURI := uri.URI("file:///test.cfm")
	srv.setDocument(docURI, "uniqueFunc()")
	srv.index.IndexFileFromResult(uri.URI("file:///only.cfc"), []parser.FunctionDef{
		{Name: "uniqueFunc", URI: "file:///only.cfc", Line: 10, Arguments: []parser.Argument{{Name: "x", Type: "string"}}},
	}, nil)

	reply, result, _ := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentHover, protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 0, Character: 5},
		},
	})
	_ = srv.handleHover(context.Background(), reply, req)
	hover, ok := (*result).(*protocol.Hover)
	if !ok || hover == nil {
		t.Fatal("expected hover for globally unique function")
	}
	if !strings.Contains(hover.Contents.Value, "uniqueFunc") {
		t.Errorf("expected uniqueFunc in hover, got %s", hover.Contents.Value)
	}
}

func TestArgumentCompletionBuiltin(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///test.cfm")
	srv.setDocument(docURI, "<cfset x = ArrayAppend(")

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 0, Character: 23},
		},
		Context: &protocol.CompletionContext{TriggerKind: protocol.CompletionTriggerKindInvoked},
	})
	if err := srv.handleCompletion(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}
	list := completionListFromResult(t, *result)
	// Should have named argument items with = suffix
	foundArg := false
	for _, item := range list.Items {
		if strings.HasSuffix(item.Label, "=") && item.Kind == protocol.CompletionItemKindField {
			foundArg = true
			break
		}
	}
	if !foundArg {
		t.Error("expected named argument completions inside ArrayAppend(")
	}
}

func TestArgumentCompletionUserFunction(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///test.cfc")
	docContent := "component {\nfunction save(required string name, numeric age) {}\n}\nsave("
	srv.setDocument(docURI, docContent)
	srv.index.IndexFile(docURI, docContent)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 3, Character: 5},
		},
		Context: &protocol.CompletionContext{TriggerKind: protocol.CompletionTriggerKindInvoked},
	})
	if err := srv.handleCompletion(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}
	list := completionListFromResult(t, *result)
	var argNames []string
	for _, item := range list.Items {
		if item.Kind == protocol.CompletionItemKindField {
			argNames = append(argNames, item.Label)
		}
	}
	if !strings.Contains(strings.Join(argNames, ","), "name=") {
		t.Errorf("expected name= in argument completions, got %v", argNames)
	}
	if !strings.Contains(strings.Join(argNames, ","), "age=") {
		t.Errorf("expected age= in argument completions, got %v", argNames)
	}
}

func TestArgumentCompletionSortOrder(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///test.cfm")
	srv.setDocument(docURI, "<cfset x = Len(")

	reply, result, _ := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 0, Character: 15},
		},
		Context: &protocol.CompletionContext{TriggerKind: protocol.CompletionTriggerKindInvoked},
	})
	_ = srv.handleCompletion(context.Background(), reply, req)
	list := completionListFromResult(t, *result)

	// First items should be argument completions (sort with !)
	if len(list.Items) > 0 && list.Items[0].Kind == protocol.CompletionItemKindField {
		if !strings.HasPrefix(list.Items[0].SortText, SortFuncArguments) {
			t.Errorf("expected argument items to sort first with SortFuncArguments, got sortText=%q", list.Items[0].SortText)
		}
	}
}

func TestCompletionSnippetFormat(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///test.cfc")
	docContent := "component {\nfunction myFunc(required string name, numeric count) {}\n}"
	srv.setDocument(docURI, docContent)

	pr := srv.parseContent(docURI, docContent)
	srv.mu.Lock()
	srv.parseResults[docURI] = pr
	srv.mu.Unlock()
	srv.rebuildFileCompletionCacheFromPR(docURI, pr)

	items := srv.completionFromCache(docURI, 5)
	var found *protocol.CompletionItem
	for i := range items {
		if items[i].Label == "myFunc" {
			found = &items[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected myFunc in completions")
	}
	if found.InsertTextFormat != protocol.InsertTextFormatSnippet {
		t.Error("expected snippet format")
	}
	if !strings.Contains(found.InsertText, "${1:name}") {
		t.Errorf("expected ${1:name} placeholder, got %s", found.InsertText)
	}
	if !strings.Contains(found.InsertText, "${2:count}") {
		t.Errorf("expected ${2:count} placeholder, got %s", found.InsertText)
	}
}

func TestScopeSortOrder(t *testing.T) {
	items := getBuiltinFuncItems()
	var scopeItems []protocol.CompletionItem
	for _, item := range items {
		if item.Kind == protocol.CompletionItemKindKeyword {
			scopeItems = append(scopeItems, item)
		}
	}
	if len(scopeItems) == 0 {
		t.Fatal("expected scope items")
	}
	// All scopes should have ~ prefix in SortText
	for _, item := range scopeItems {
		if !strings.HasPrefix(item.SortText, SortScopes) {
			t.Errorf("scope %q should have SortScopes prefix, got %q", item.Label, item.SortText)
		}
	}
	// VARIABLES should sort before SESSION
	var varIdx, sessIdx int
	for i, item := range scopeItems {
		if item.Label == "VARIABLES" {
			varIdx = i
		}
		if item.Label == "SESSION" {
			sessIdx = i
		}
	}
	if varIdx >= sessIdx {
		t.Error("VARIABLES should appear before SESSION")
	}
}

func TestCompletionResponseTime(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///test.cfc")
	docContent := "component {\nfunction init() {}\nfunction getData() {}\n}\n"
	srv.setDocument(docURI, docContent)
	srv.index.IndexFile(docURI, docContent)
	pr := srv.parseContent(docURI, docContent)
	srv.mu.Lock()
	srv.parseResults[docURI] = pr
	srv.mu.Unlock()
	srv.rebuildFileCompletionCacheFromPR(docURI, pr)

	reply, _, _ := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 4, Character: 0},
		},
		Context: &protocol.CompletionContext{TriggerKind: protocol.CompletionTriggerKindInvoked},
	})

	start := time.Now()
	for i := 0; i < 100; i++ {
		_ = srv.handleCompletion(context.Background(), reply, req)
	}
	elapsed := time.Since(start)
	avg := elapsed / 100
	if avg > 5*time.Millisecond {
		t.Errorf("completion too slow: avg %v per request (threshold 5ms)", avg)
	}
}

func TestHoverResponseTime(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///test.cfm")
	srv.setDocument(docURI, "<cfset x = ArrayAppend(arr, val)>")

	reply, _, _ := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentHover, protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 0, Character: 15},
		},
	})

	start := time.Now()
	for i := 0; i < 100; i++ {
		_ = srv.handleHover(context.Background(), reply, req)
	}
	elapsed := time.Since(start)
	avg := elapsed / 100
	if avg > 2*time.Millisecond {
		t.Errorf("hover too slow: avg %v per request (threshold 2ms)", avg)
	}
}

func TestDefinitionResponseTime(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///test.cfc")
	docContent := "component {\nfunction myFunc() {}\n}\nmyFunc()"
	srv.setDocument(docURI, docContent)
	srv.index.IndexFile(docURI, docContent)

	reply, _, _ := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDefinition, protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 3, Character: 3},
		},
	})

	start := time.Now()
	for i := 0; i < 100; i++ {
		_ = srv.handleDefinition(context.Background(), reply, req)
	}
	elapsed := time.Since(start)
	avg := elapsed / 100
	if avg > 2*time.Millisecond {
		t.Errorf("definition too slow: avg %v per request (threshold 2ms)", avg)
	}
}

func TestDocumentLinkResponseTime(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///test.cfm")
	// Generate a large document with many includes
	var lines []string
	for i := 0; i < 500; i++ {
		lines = append(lines, fmt.Sprintf(`<cfinclude template="file%d.cfm">`, i))
	}
	srv.setDocument(docURI, strings.Join(lines, "\n"))

	pr := srv.parseContent(docURI, strings.Join(lines, "\n"))
	srv.mu.Lock()
	srv.parseResults[docURI] = pr
	srv.mu.Unlock()

	reply, _, _ := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDocumentLink, protocol.DocumentLinkParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
	})

	start := time.Now()
	for i := 0; i < 10; i++ {
		_ = srv.handleDocumentLink(context.Background(), reply, req)
	}
	elapsed := time.Since(start)
	avg := elapsed / 10
	if avg > 50*time.Millisecond {
		t.Errorf("documentLink too slow for 500-line doc: avg %v (threshold 50ms)", avg)
	}
}

func TestIndexingDoesNotExtractLinks(t *testing.T) {
	srv := newTestServer()
	docURI := uri.URI("file:///indexed.cfc")
	content := "component {\n<cfinclude template=\"header.cfm\">\nfunction init() {}\n}"

	pr := srv.parseContentForIndex(docURI, content)
	if len(pr.Links) != 0 {
		t.Errorf("parseContentForIndex should not extract links, got %d", len(pr.Links))
	}

	pr2 := srv.parseContent(docURI, content)
	if len(pr2.Links) == 0 {
		t.Error("parseContent should extract links")
	}
}

func TestResolverRegexNotRecompiledPerCall(t *testing.T) {
	resolvers := []parser.Resolver{
		{Match: `kernel\.get([A-Za-z0-9_]+)\(\)`, Resolve: "packages.$1", Prefix: "kernel.get"},
	}

	// First call compiles the regex
	parser.ResolveFromCall("kernel.getFoo()", resolvers)

	// Subsequent calls should be fast (cached regex)
	start := time.Now()
	for i := 0; i < 10000; i++ {
		parser.ResolveFromCall("kernel.getBar()", resolvers)
	}
	elapsed := time.Since(start)
	avg := elapsed / 10000
	if avg > 10*time.Microsecond {
		t.Errorf("resolver too slow (regex not cached?): avg %v per call (threshold 10µs)", avg)
	}
}

func TestDefinitionNoGlobalResolution(t *testing.T) {
	srv := newTestServer()
	srv.GlobalFunctionResolution = false
	docURI := uri.URI("file:///test.cfm")
	otherURI := uri.URI("file:///other.cfc")
	srv.setDocument(docURI, "myFunc()")
	srv.index.IndexFileFromResult(otherURI, []parser.FunctionDef{
		{Name: "myFunc", URI: otherURI, Line: 10},
	}, nil)

	reply, result, _ := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDefinition, protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 0, Character: 3},
		},
	})
	_ = srv.handleDefinition(context.Background(), reply, req)
	// With global resolution disabled, should NOT resolve to other file
	if *result != nil {
		t.Errorf("expected nil with GlobalFunctionResolution disabled, got %T", *result)
	}
}

func TestHoverNoGlobalResolution(t *testing.T) {
	srv := newTestServer()
	srv.GlobalFunctionResolution = false
	docURI := uri.URI("file:///test.cfm")
	srv.setDocument(docURI, "uniqueFunc()")
	srv.index.IndexFileFromResult(uri.URI("file:///only.cfc"), []parser.FunctionDef{
		{Name: "uniqueFunc", URI: "file:///only.cfc", Line: 10, Arguments: []parser.Argument{{Name: "x"}}},
	}, nil)

	reply, result, _ := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentHover, protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 0, Character: 5},
		},
	})
	_ = srv.handleHover(context.Background(), reply, req)
	// With global resolution disabled, should NOT show hover from other file
	if *result != nil {
		t.Errorf("expected nil with GlobalFunctionResolution disabled, got %T", *result)
	}
}

func TestDefinitionWithGlobalResolutionEnabled(t *testing.T) {
	srv := newTestServer()
	srv.GlobalFunctionResolution = true
	docURI := uri.URI("file:///test.cfm")
	otherURI := uri.URI("file:///other.cfc")
	srv.setDocument(docURI, "myFunc()")
	srv.index.IndexFileFromResult(otherURI, []parser.FunctionDef{
		{Name: "myFunc", URI: otherURI, Line: 10},
	}, nil)

	reply, result, _ := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDefinition, protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 0, Character: 3},
		},
	})
	_ = srv.handleDefinition(context.Background(), reply, req)
	// With global resolution enabled, should resolve
	if *result == nil {
		t.Error("expected definition with GlobalFunctionResolution enabled")
	}
}

func TestDefinitionViaCreateObject(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "models"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "models", "Order.cfc"), []byte("component {\nfunction getTotal() {}\nfunction getItems() {}\n}"), 0o644)

	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}

	docURI := uri.URI("file://" + filepath.Join(dir, "test.cfm"))
	docContent := `<cfset obj = createObject("component","models.Order")>
<cfset total = obj.getTotal()>`
	srv.setDocument(docURI, docContent)
	srv.index.IndexFile(docURI, docContent)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDefinition, protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 1, Character: 20}, // on "getTotal"
		},
	})
	if err := srv.handleDefinition(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}
	if *result == nil {
		t.Fatal("expected definition for obj.getTotal() via createObject")
	}
}

func TestCompletionViaCreateObject(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "models"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "models", "Order.cfc"), []byte("component {\nfunction getTotal() {}\nfunction getItems() {}\n}"), 0o644)

	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}

	docURI := uri.URI("file://" + filepath.Join(dir, "test.cfm"))
	docContent := "<cfset obj = createObject(\"component\",\"models.Order\")>\n<cfset x = obj."
	srv.setDocument(docURI, docContent)
	srv.index.IndexFile(docURI, docContent)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 1, Character: 15},
		},
		Context: &protocol.CompletionContext{TriggerKind: protocol.CompletionTriggerKindTriggerCharacter, TriggerCharacter: "."},
	})
	if err := srv.handleCompletion(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}
	list := completionListFromResult(t, *result)
	var names []string
	for _, item := range list.Items {
		names = append(names, item.Label)
	}
	if !strings.Contains(strings.Join(names, ","), "getTotal") {
		t.Errorf("expected getTotal in completions via createObject, got %v", names)
	}
}

func TestDefinitionViaBeanProperty(t *testing.T) {
	dir := t.TempDir()
	// Create bean directory with a CFC
	_ = os.MkdirAll(filepath.Join(dir, "dao"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "dao", "UserDAO.cfc"), []byte("component {\nfunction getById(required numeric id) {}\nfunction getAll() {}\n}"), 0o644)

	// Create Application.cfc with beanPaths
	_ = os.WriteFile(filepath.Join(dir, "Application.cfc"), []byte(`component {
	this.beanPaths["dao"] = expandPath("./dao");
}`), 0o644)

	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}

	// CFC with property that resolves via bean lookup
	cfcContent := `component {
	property name="userDAO" inject="UserDAO@dao";

	function listUsers() {
		return variables.userDAO.getAll();
	}
}`
	docURI := uri.URI("file://" + filepath.Join(dir, "Service.cfc"))
	srv.setDocument(docURI, cfcContent)

	// Parse with bean lookup from Application.cfc
	pr := parser.ParseWithOptions(docURI, cfcContent, parser.ParseOptions{
		BeanLookup: func(name string) string {
			// Simulate bean resolution: UserDAO@dao -> dao/UserDAO.cfc
			lower := strings.ToLower(name)
			if lower == "userdao" {
				return filepath.Join(dir, "dao", "UserDAO.cfc")
			}
			return ""
		},
	})
	srv.index.IndexFileFromResult(docURI, pr.Funcs, pr.Refs)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDefinition, protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 4, Character: 28}, // on "getAll"
		},
	})
	if err := srv.handleDefinition(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}
	if *result == nil {
		t.Fatal("expected definition for variables.userDAO.getAll() via bean property")
	}
}

func TestCompletionViaBeanProperty(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "dao"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "dao", "UserDAO.cfc"), []byte("component {\nfunction getById() {}\nfunction getAll() {}\n}"), 0o644)

	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}

	cfcContent := `component {
	property name="userDAO" inject="UserDAO@dao";

	function doStuff() {
		variables.userDAO.
	}
}`
	docURI := uri.URI("file://" + filepath.Join(dir, "Service.cfc"))
	srv.setDocument(docURI, cfcContent)

	pr := parser.ParseWithOptions(docURI, cfcContent, parser.ParseOptions{
		BeanLookup: func(name string) string {
			if strings.EqualFold(name, "userdao") {
				return filepath.Join(dir, "dao", "UserDAO.cfc")
			}
			return ""
		},
	})
	srv.index.IndexFileFromResult(docURI, pr.Funcs, pr.Refs)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
			Position:     protocol.Position{Line: 4, Character: 20},
		},
		Context: &protocol.CompletionContext{TriggerKind: protocol.CompletionTriggerKindTriggerCharacter, TriggerCharacter: "."},
	})
	if err := srv.handleCompletion(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}
	list := completionListFromResult(t, *result)
	var names []string
	for _, item := range list.Items {
		names = append(names, item.Label)
	}
	joined := strings.Join(names, ",")
	if !strings.Contains(joined, "getById") || !strings.Contains(joined, "getAll") {
		t.Errorf("expected getById and getAll via bean property, got %v", names)
	}
}

func TestBeansTestdata_InjectResolution(t *testing.T) {
	dir := filepath.Join(testdataDir(), "beans")
	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}

	// Build bean map from Application.cfc (simulates workspace indexing)
	beanPaths := cfpath.LoadAppBeanPaths(dir)
	if len(beanPaths) == 0 {
		t.Fatal("expected bean paths from Application.cfc")
	}
	beans := buildBeanMap(beanPaths, srv.FS)
	srv.index.SetBeans(beans)

	// Parse PropertyTest.cfc with bean lookup
	ptPath := filepath.Join(dir, "PropertyTest.cfc")
	ptContent, _ := os.ReadFile(ptPath)
	ptURI := uri.URI("file://" + ptPath)
	pr := parser.ParseWithOptions(ptURI, string(ptContent), parser.ParseOptions{
		BeanLookup: srv.index.LookupBean,
	})
	srv.index.IndexFileFromResult(ptURI, pr.Funcs, pr.Refs)
	srv.setDocument(ptURI, string(ptContent))

	// Index UserDAO.cfc
	daoPath := filepath.Join(dir, "dao", "UserDAO.cfc")
	daoContent, _ := os.ReadFile(daoPath)
	daoURI := uri.URI("file://" + daoPath)
	srv.index.IndexFile(daoURI, string(daoContent))

	// Test: variables.userDAO.getById should resolve to UserDAO.cfc
	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDefinition, protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(ptURI)},
			Position:     protocol.Position{Line: 13, Character: 38}, // on "getById"
		},
	})
	if err := srv.handleDefinition(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}
	if *result == nil {
		t.Fatal("expected definition for variables.userDAO.getById via inject bean")
	}
	loc, ok := (*result).(protocol.Location)
	if !ok {
		t.Fatalf("expected Location, got %T", *result)
	}
	if !strings.Contains(string(loc.URI), "UserDAO.cfc") {
		t.Errorf("expected UserDAO.cfc, got %s", loc.URI)
	}
}

func TestBeansTestdata_PositionalTypeResolution(t *testing.T) {
	dir := filepath.Join(testdataDir(), "beans")
	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}

	// Build bean map
	beanPaths := cfpath.LoadAppBeanPaths(dir)
	beans := buildBeanMap(beanPaths, srv.FS)
	srv.index.SetBeans(beans)

	// Parse PositionalProps.cfc — has "property UserDAO userDAO;"
	ppPath := filepath.Join(dir, "PositionalProps.cfc")
	ppContent, _ := os.ReadFile(ppPath)
	ppURI := uri.URI("file://" + ppPath)
	pr := parser.ParseWithOptions(ppURI, string(ppContent), parser.ParseOptions{
		BeanLookup: srv.index.LookupBean,
	})
	srv.index.IndexFileFromResult(ppURI, pr.Funcs, pr.Refs)
	srv.setDocument(ppURI, string(ppContent))

	// Check that userDAO ref was created
	ref := srv.index.LookupComponentRefInFile("userDAO", ppURI, 100)
	if ref == nil {
		t.Fatal("expected component ref for userDAO from positional property type")
	}
	t.Logf("userDAO resolved to: %s", ref.Component)
}

func TestBeansTestdata_ServiceInjectResolution(t *testing.T) {
	dir := filepath.Join(testdataDir(), "beans")
	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}

	// Build bean map
	beanPaths := cfpath.LoadAppBeanPaths(dir)
	beans := buildBeanMap(beanPaths, srv.FS)
	srv.index.SetBeans(beans)

	// Parse BeanUserService.cfc
	svcPath := filepath.Join(dir, "services", "BeanUserService.cfc")
	svcContent, _ := os.ReadFile(svcPath)
	svcURI := uri.URI("file://" + svcPath)
	pr := parser.ParseWithOptions(svcURI, string(svcContent), parser.ParseOptions{
		BeanLookup: srv.index.LookupBean,
	})
	srv.index.IndexFileFromResult(svcURI, pr.Funcs, pr.Refs)

	// Check that userDAO ref was created from inject="UserDAO@dao"
	ref := srv.index.LookupComponentRefInFile("userDAO", svcURI, 100)
	if ref == nil {
		t.Fatal("expected component ref for userDAO in BeanUserService via inject")
	}
	if !strings.Contains(strings.ToLower(ref.Component), "userdao") {
		t.Errorf("expected component containing 'userdao', got %s", ref.Component)
	}
}

func TestFindAllCalls_GetById(t *testing.T) {
	dir := filepath.Join(testdataDir(), "beans")
	beanPaths := cfpath.LoadAppBeanPaths(dir)
	beans := buildBeanMap(beanPaths, vfs.OS{})

	// Build a bean lookup function
	beanLookup := func(name string) string {
		return beans[strings.ToLower(name)]
	}

	// Build resolvers (none for this test)
	var resolvers []parser.Resolver

	entries := refs.Find(vfs.OS{}, []string{dir}, refs.Options{
		FuncName:   "getById",
		Resolvers:  resolvers,
		BeanLookup: beanLookup,
	})

	if len(entries) == 0 {
		t.Fatal("expected at least one call to getById")
	}

	// Should find calls in PropertyTest.cfc and BeanUserService.cfc
	var files []string
	for _, e := range entries {
		files = append(files, filepath.Base(e.File))
	}
	joined := strings.Join(files, ",")
	if !strings.Contains(joined, "PropertyTest.cfc") {
		t.Errorf("expected PropertyTest.cfc in results, got %v", files)
	}
	if !strings.Contains(joined, "BeanUserService.cfc") {
		t.Errorf("expected BeanUserService.cfc in results, got %v", files)
	}
}

func TestFindAllCalls_GetAll(t *testing.T) {
	dir := filepath.Join(testdataDir(), "beans")

	entries := refs.Find(vfs.OS{}, []string{dir}, refs.Options{
		FuncName: "getAll",
	})

	if len(entries) == 0 {
		t.Fatal("expected at least one call to getAll")
	}

	// Should find call in BeanUserService.cfc (variables.userDAO.getAll())
	found := false
	for _, e := range entries {
		if strings.Contains(filepath.Base(e.File), "BeanUserService.cfc") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected getAll call in BeanUserService.cfc")
	}
}

func TestFindAllCalls_Resolved(t *testing.T) {
	dir := filepath.Join(testdataDir(), "beans")

	entries := refs.Find(vfs.OS{}, []string{dir}, refs.Options{
		FuncName: "getById",
	})

	// Calls via variables.userDAO.getById should be resolved (qualified)
	hasResolved := false
	for _, e := range entries {
		if e.Resolved {
			hasResolved = true
			break
		}
	}
	if !hasResolved {
		t.Error("expected at least one resolved (qualified) call to getById")
	}
}

func TestFindAllCalls_NoResults(t *testing.T) {
	dir := filepath.Join(testdataDir(), "beans")

	entries := refs.Find(vfs.OS{}, []string{dir}, refs.Options{
		FuncName: "nonExistentFunction",
	})

	if len(entries) != 0 {
		t.Errorf("expected no results for nonExistentFunction, got %d", len(entries))
	}
}

func TestFindComponentRefs_UserDAO(t *testing.T) {
	dir := filepath.Join(testdataDir(), "beans")
	beanPaths := cfpath.LoadAppBeanPaths(dir)
	beans := buildBeanMap(beanPaths, vfs.OS{})
	daoPath := beans["userdao@dao"]

	entries := refs.Find(vfs.OS{}, []string{dir}, refs.Options{
		Component: daoPath,
		BeanLookup: func(name string) string {
			return beans[strings.ToLower(name)]
		},
	})

	if len(entries) == 0 {
		t.Fatal("expected at least one reference to UserDAO component")
	}

	var files []string
	for _, e := range entries {
		files = append(files, filepath.Base(e.File))
	}
	joined := strings.Join(files, ",")
	if !strings.Contains(joined, "PropertyTest.cfc") {
		t.Errorf("expected PropertyTest.cfc to reference UserDAO, got %v", files)
	}
}
