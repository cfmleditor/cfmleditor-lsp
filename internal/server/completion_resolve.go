package server

import (
	"context"
	"encoding/json/v2"
	"slices"
	"sync"

	"github.com/cfmleditor/cfmleditor-lsp/internal/docs"
	"go.lsp.dev/protocol"
)

// A completion list is mostly built-in functions — 977 items on an ordinary
// component — and each carried its syntax line in detail and its description
// in documentation: two thirds of the 322KB response, for the one item the
// user ends up looking at. The encode is almost all of what the request costs
// (see PERFORMANCE-GAPS.md), so the only saving available is sending less.
//
// A client that can fill those fields in later says so at initialize, in
// textDocument.completion.completionItem.resolveSupport.properties. For such a
// client the built-in and member-function items are sent without the fields it
// listed, and completionItem/resolve fills them in for the item it highlights.
// A client that did not list them gets the full items exactly as before: one
// that cannot resolve would otherwise show a popup with no documentation at
// all, which is a regression rather than an optimisation.
//
// Only those two lists are deferred. They are the whole of the payload that
// carries documentation, and their items can be found again from what comes
// back on resolve, where a user function or a variable would need the document
// and the index at the time of the request.

// completionDefer is the set of fields a session leaves out of a completion
// list and fills in on resolve.
type completionDefer struct {
	documentation bool
	detail        bool
}

// variant indexes the process-wide item lists, one per combination.
func (d completionDefer) variant() int {
	v := 0
	if d.documentation {
		v |= 1
	}

	if d.detail {
		v |= 2
	}

	return v
}

// clientDefers reads which fields the client will resolve lazily.
func clientDefers(caps protocol.ClientCapabilities) completionDefer {
	td := caps.TextDocument
	if td == nil || td.Completion == nil || td.Completion.CompletionItem == nil {
		return completionDefer{}
	}

	props := td.Completion.CompletionItem.ResolveSupport.Properties

	return completionDefer{
		documentation: slices.Contains(props, "documentation"),
		detail:        slices.Contains(props, "detail"),
	}
}

// deferredItems holds a list in each of its four shapes. Each is built once per
// process and shared by every session that asks for it, like the full list it
// is derived from, so nothing may write to what get returns.
type deferredItems struct {
	once  [4]sync.Once
	items [4][]protocol.CompletionItem
}

// key names the docs entry item i of the full list came from, or "" for an
// item with nothing to defer.
func (v *deferredItems) get(d completionDefer, full func() []protocol.CompletionItem, key func(i int, it protocol.CompletionItem) string) []protocol.CompletionItem {
	i := d.variant()
	if i == 0 {
		return full()
	}

	v.once[i].Do(func() {
		src := full()
		out := make([]protocol.CompletionItem, len(src))

		for j, it := range src {
			if k := key(j, it); k != "" {
				if d.documentation {
					it.Documentation = nil
				}

				if d.detail {
					it.Detail.Clear()
				}

				if k != it.Label {
					it.Data = protocol.LSPAny(mustMarshal(k))
				}
			}

			out[j] = it
		}

		v.items[i] = out
	})

	return v.items[i]
}

var (
	deferredBuiltins deferredItems
	deferredMembers  deferredItems
)

// builtinFuncItems is getBuiltinFuncItems in the shape this session's client
// asked for.
func (s *Server) builtinFuncItems() []protocol.CompletionItem {
	return deferredBuiltins.get(s.completionDefer, getBuiltinFuncItems, builtinKey)
}

// memberFuncItems is getMemberFuncItems in the shape this session's client
// asked for.
func (s *Server) memberFuncItems() []protocol.CompletionItem {
	return deferredMembers.get(s.completionDefer, getMemberFuncItems, memberKey)
}

// builtinKey names the docs entry a built-in function item came from, or "" for
// the scope keywords that share its list and carry nothing to defer. The label
// is the entry's name, so a built-in carries no data of its own on the wire:
// resolve recognises one by its sort text, which only built-ins start with
// SortBuiltinFuncs, and looks its label up.
func builtinKey(_ int, it protocol.CompletionItem) string {
	if it.Kind != protocol.CompletionItemKindFunction {
		return ""
	}

	return it.Label
}

// memberKey names the docs entry a member-function item came from, recorded
// when the list was built. A member name is not unique — len is arrayLen's,
// stringLen's and structLen's — so the entry travels in the item's data and
// resolve reads it from there.
func memberKey(i int, _ protocol.CompletionItem) string {
	getMemberFuncItems()

	return memberFuncEntries[i]
}

// trueOrOmitted is a capability flag: nil, so omitted, when false.
func trueOrOmitted(on bool) *bool {
	if !on {
		return nil
	}

	return &on
}

func mustMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}

	return b
}

// handleCompletionResolve fills in the fields a deferred item left out. An item
// it does not recognise — any item not from the built-in or member lists, or
// one from a client that deferred nothing — comes back as it went.
func (s *Server) handleCompletionResolve(_ context.Context, rawParams []byte) (any, error) {
	var item protocol.CompletionItem
	if err := json.Unmarshal(rawParams, &item); err != nil {
		return nil, err
	}

	resolveCompletionItem(&item)

	return &item, nil
}

func resolveCompletionItem(item *protocol.CompletionItem) {
	var name string

	switch item.Kind { //nolint:exhaustive // only the two deferred lists resolve
	case protocol.CompletionItemKindFunction:
		if sort, _ := item.SortText.Get(); sort == SortBuiltinFuncs+item.Label {
			name = item.Label
		}
	case protocol.CompletionItemKindMethod:
		// Only a deferred member item carries data, and its data is the name
		// of the entry it came from. Its label is not: member len resolved
		// by label would find the built-in Len.
		if len(item.Data) > 0 {
			_ = json.Unmarshal(item.Data, &name) //nolint:errcheck // unreadable data resolves nothing
		}
	}

	if name == "" {
		return
	}

	e, ok := docs.LookupFunction(name)
	if !ok {
		return
	}

	if item.Detail.IsZero() {
		item.Detail = optStr(e.Syntax)
	}

	if item.Documentation == nil {
		item.Documentation = tooltip(e.Description)
	}
}
