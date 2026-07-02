package server

import (
	json "github.com/go-json-experiment/json"

	"go.lsp.dev/protocol"
)

// optStr wraps a string for protocol fields typed as Optional[string].
func optStr(s string) protocol.Optional[string] {
	return protocol.NewOptional(s)
}

// tooltip wraps a string for protocol fields typed as InlayHintTooltip (documentation/message unions).
func tooltip(s string) protocol.InlayHintTooltip {
	return protocol.String(s)
}

// changeRangeAndText extracts the range, text and whether the change is a whole-document
// replacement from a TextDocumentContentChangeEvent union value.
func changeRangeAndText(change protocol.TextDocumentContentChangeEvent) (r protocol.Range, text string, isFull bool) {
	switch c := change.(type) {
	case *protocol.TextDocumentContentChangePartial:
		return c.Range, c.Text, false
	case *protocol.TextDocumentContentChangeWholeDocument:
		return protocol.Range{}, c.Text, true
	default:
		return protocol.Range{}, "", true
	}
}

// argString decodes the i'th ExecuteCommand argument (a raw JSON LSPAny) as a string.
func argString(args []protocol.LSPAny, i int) (string, bool) {
	if i < 0 || i >= len(args) {
		return "", false
	}

	var v string
	if err := json.Unmarshal(args[i], &v); err != nil {
		return "", false
	}

	return v, true
}

// argFloat decodes the i'th ExecuteCommand argument (a raw JSON LSPAny) as a float64.
func argFloat(args []protocol.LSPAny, i int) (float64, bool) {
	if i < 0 || i >= len(args) {
		return 0, false
	}

	var v float64
	if err := json.Unmarshal(args[i], &v); err != nil {
		return 0, false
	}

	return v, true
}
