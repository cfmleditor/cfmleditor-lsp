package server

import (
	"context"
	"encoding/json/v2"
	"reflect"
	"testing"

	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// initializeWithResolve runs initialize with the given resolveSupport properties, or
// with no completion capabilities at all when props is nil.
func initializeWithResolve(t *testing.T, props []string) (*Server, protocol.ServerCapabilities) {
	t.Helper()

	caps := map[string]any{}
	if props != nil {
		caps["textDocument"] = map[string]any{
			"completion": map[string]any{
				"completionItem": map[string]any{
					"resolveSupport": map[string]any{"properties": props},
				},
			},
		}
	}

	dir := t.TempDir()
	s := NewServer(nil, cflog.NewLogger(false))

	raw, err := json.Marshal(map[string]any{
		"processId":        nil,
		"rootUri":          "file://" + dir,
		"capabilities":     caps,
		"workspaceFolders": []map[string]any{{"uri": "file://" + dir, "name": "w"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	res, err := s.handleInitialize(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}

	out, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}

	var result protocol.InitializeResult
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatal(err)
	}

	return s, result.Capabilities
}

// A field is left out only when the client said it will resolve that field,
// and resolveProvider is advertised only when something was left out. A client
// that cannot resolve would otherwise show a popup with no documentation.
func TestCompletionDefersOnlyWhatTheClientResolves(t *testing.T) {
	cases := []struct {
		name    string
		props   []string
		want    completionDefer
		resolve bool
	}{
		{"no completion capabilities", nil, completionDefer{}, false},
		{"resolves nothing", []string{}, completionDefer{}, false},
		{"resolves unrelated fields", []string{"additionalTextEdits"}, completionDefer{}, false},
		{"resolves documentation", []string{"documentation"}, completionDefer{documentation: true}, true},
		{"resolves both", []string{"documentation", "detail", "additionalTextEdits"}, completionDefer{documentation: true, detail: true}, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, caps := initializeWithResolve(t, c.props)

			if s.completionDefer != c.want {
				t.Errorf("deferred %+v, want %+v", s.completionDefer, c.want)
			}

			got := caps.CompletionProvider != nil && caps.CompletionProvider.ResolveProvider != nil && *caps.CompletionProvider.ResolveProvider
			if got != c.resolve {
				t.Errorf("resolveProvider %v, want %v", got, c.resolve)
			}
		})
	}
}

// Every deferred item resolves to exactly the item a client that deferred
// nothing is sent. Checked over the whole of both lists rather than a sample,
// because the member list is where a wrong answer hides: member names repeat,
// and resolving one by its label would hand len the built-in Len's text.
func TestResolvedItemsMatchTheFullOnes(t *testing.T) {
	both := completionDefer{documentation: true, detail: true}

	lists := []struct {
		name string
		full func() []protocol.CompletionItem
		lean []protocol.CompletionItem
	}{
		{"builtin", getBuiltinFuncItems, deferredBuiltins.get(both, getBuiltinFuncItems, builtinKey)},
		{"member", getMemberFuncItems, deferredMembers.get(both, getMemberFuncItems, memberKey)},
	}

	for _, l := range lists {
		full := l.full()
		if len(full) != len(l.lean) {
			t.Fatalf("%s: %d deferred items for %d full ones", l.name, len(l.lean), len(full))
		}

		deferred := 0

		for i := range full {
			item := roundTrip(t, &l.lean[i])

			if item.Documentation == nil && full[i].Documentation != nil {
				deferred++
			}

			resolveCompletionItem(&item)

			// Data is how a member item finds its entry again; it is the
			// one field a deferred item carries that the full one does not.
			item.Data = nil

			if !reflect.DeepEqual(item, full[i]) {
				t.Errorf("%s %q resolved to\n%+v\nwant\n%+v", l.name, full[i].Label, item, full[i])
			}
		}

		if deferred == 0 {
			t.Errorf("%s: nothing was deferred", l.name)
		}
	}
}

// roundTrip sends an item through JSON, as a client does between completion
// and resolve.
func roundTrip(t *testing.T, it *protocol.CompletionItem) protocol.CompletionItem {
	t.Helper()

	b, err := json.Marshal(it)
	if err != nil {
		t.Fatal(err)
	}

	var out protocol.CompletionItem
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}

	return out
}

// The full lists are shared by every session in the process. Building the
// deferred shape from one must not write to it, or the first session to defer
// would strip the documentation from every other session's popup.
func TestDeferringLeavesTheSharedListsWhole(t *testing.T) {
	both := completionDefer{documentation: true, detail: true}
	deferredBuiltins.get(both, getBuiltinFuncItems, builtinKey)
	deferredMembers.get(both, getMemberFuncItems, memberKey)

	for _, list := range [][]protocol.CompletionItem{getBuiltinFuncItems(), getMemberFuncItems()} {
		for _, it := range list {
			if it.Kind == protocol.CompletionItemKindKeyword {
				continue
			}

			if it.Documentation == nil || it.Detail.IsZero() || len(it.Data) > 0 {
				t.Fatalf("shared item %q was modified: %+v", it.Label, it)
			}
		}
	}
}

// An item from anywhere else comes back as it went: a user function, a
// variable, a built-in whose sort text says it is not one.
func TestResolveLeavesOtherItemsAlone(t *testing.T) {
	items := []protocol.CompletionItem{
		{Label: "getUser", Kind: protocol.CompletionItemKindFunction, SortText: optStr("1getUser")},
		{Label: "abs", Kind: protocol.CompletionItemKindFunction, SortText: optStr("1abs")},
		{Label: "len", Kind: protocol.CompletionItemKindMethod, SortText: optStr(SortMemberFuncs + "len")},
		{Label: "x", Kind: protocol.CompletionItemKindVariable},
	}

	for _, it := range items {
		got := it
		resolveCompletionItem(&got)

		if !reflect.DeepEqual(got, it) {
			t.Errorf("%q changed on resolve: %+v", it.Label, got)
		}
	}
}

// The saving is the point, so it is measured rather than assumed: a deferred
// response is well under half the size of the full one.
func TestDeferredCompletionIsSmaller(t *testing.T) {
	size := func(d completionDefer) int {
		s := newTestServer()
		s.completionDefer = d
		docURI := uri.File("/ws/open/Doc.cfc")
		s.setDocument(docURI, benchDoc(20))

		req := makeCall(t, protocol.MethodTextDocumentCompletion, protocol.CompletionParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
				Position:     protocol.Position{Line: 4, Character: 6},
			},
		})

		res, err := s.handleCompletion(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}

		b, err := json.Marshal(res)
		if err != nil {
			t.Fatal(err)
		}

		return len(b)
	}

	full, lean := size(completionDefer{}), size(completionDefer{documentation: true, detail: true})
	t.Logf("full %d bytes, deferred %d bytes (%.0f%%)", full, lean, 100*float64(lean)/float64(full))

	if lean*2 > full {
		t.Errorf("deferred response is %d bytes against %d; expected under half", lean, full)
	}
}
