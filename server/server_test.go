package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
	"go.uber.org/zap"
)

func newTestServer() *Server {
	return NewServer(nil, zap.NewNop())
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
	srv.setDocument(uri.URI("file:///test.cfm"), "content")

	reply, _, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDidClose, protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.cfm"},
	})

	if err := srv.handleDidClose(context.Background(), reply, req); err != nil {
		t.Fatal(err)
	}
	if *replyErr != nil {
		t.Fatal(*replyErr)
	}

	if _, ok := srv.getDocument(uri.URI("file:///test.cfm")); ok {
		t.Error("document should have been removed")
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
		t.Errorf("expected first tag to be cfoutput, got %s", list.Items[0].Label)
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
		if item.Kind != protocol.CompletionItemKindFunction {
			t.Errorf("expected Function kind for %s, got %v", item.Label, item.Kind)
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
		if item.Kind != protocol.CompletionItemKindFunction {
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
		if item.Label == "datasource" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected datasource attribute in cfquery completions")
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
		if item.Kind != protocol.CompletionItemKindFunction {
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findUnclosedTags(tt.content, tt.line, tt.char)
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
		{"script public", "public function getData() {", []string{"getData"}},
		{"script bare", "function doStuff() {", []string{"doStuff"}},
		{"script with return type", "private struct function buildQuery() {", []string{"buildQuery"}},
		{"multiple", "<cffunction name=\"a\">\nfunction b() {", []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defs := parseFunctionDefs("file:///test.cfc", tt.content)
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
	srv := newTestServer()

	cfcContent := `<cfcomponent>
<cffunction name="getUser">
	<cfreturn "user">
</cffunction>
</cfcomponent>`
	cfcURI := uri.URI("file:///app/User.cfc")
	srv.index.indexFile(cfcURI, cfcContent)

	callerContent := `<cfset result = getUser()>`
	callerURI := uri.URI("file:///app/index.cfm")
	srv.setDocument(callerURI, callerContent)

	reply, result, replyErr := captureReply(t)
	req := makeCall(t, protocol.MethodTextDocumentDefinition, protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(callerURI)},
			Position:     protocol.Position{Line: 0, Character: 18},
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
	if loc.URI != protocol.DocumentURI(cfcURI) {
		t.Errorf("expected URI %s, got %s", cfcURI, loc.URI)
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
			got := wordAtPosition(tt.content, tt.line, tt.char)
			if got != tt.want {
				t.Errorf("wordAtPosition() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIndexReindexOnChange(t *testing.T) {
	srv := newTestServer()
	cfcURI := uri.URI("file:///app/Service.cfc")

	srv.index.indexFile(cfcURI, `function oldFunc() {}`)
	if defs := srv.index.Lookup("oldFunc"); len(defs) != 1 {
		t.Fatal("expected oldFunc indexed")
	}

	srv.index.indexFile(cfcURI, `function newFunc() {}`)
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
function saveUser() {
}
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
	srv.index.indexFile("file:///app/User.cfc", `function getUser() {}
function deleteUser() {}`)
	srv.index.indexFile("file:///app/Order.cfc", `function getOrder() {}`)

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
	srv.index.indexFile("file:///app/User.cfc", `function getUser() {}
function deleteUser() {}`)

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
	if hover.Contents.Kind != protocol.Markdown {
		t.Errorf("expected markdown, got %s", hover.Contents.Kind)
	}
	if !strings.Contains(hover.Contents.Value, "Len") {
		t.Errorf("expected hover to contain 'Len', got %s", hover.Contents.Value)
	}
	if !strings.Contains(hover.Contents.Value, "Len(value)") {
		t.Errorf("expected hover to contain signature, got %s", hover.Contents.Value)
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
