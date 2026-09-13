package server

import (
	"context"
	"strings"

	json "github.com/go-json-experiment/json"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/protocol"
)

// handleDocumentHighlight answers textDocument/documentHighlight: the editor
// asks what else in *this document* is the same thing as the identifier under
// the cursor, and shades those occurrences.
//
// This is deliberately a textual answer, and it reports
// DocumentHighlightKindText — literally "a textual occurrence" — rather than
// claiming more than it knows. Matching is whole-identifier and case-folded,
// through the same identSpan the references handler uses, so CFML's
// case-insensitivity is honoured and `user` does not light up inside
// `username`.
//
// It stays inside the open document, which is what separates it from
// textDocument/references: no file is read, nothing is parsed, and the whole
// answer is a scan of one buffer. That is why this is on by default while
// references is opt-in.
//
// The Read/Write distinction the protocol offers is deliberately not attempted.
// CFML spells far too many things with `=` — an assignment, a named argument
// (`f( name = 1 )`), a tag attribute (`<cffunction name="x">`), an `<cfset>`
// body — and a cheap "is the next token `=`" rule would mark the attribute
// *name* in every tag as a write to a variable. A wrong Write badge is worse
// than no badge, and nothing here can tell the cases apart without the semantic
// pass this handler exists to avoid.
func (s *Server) handleDocumentHighlight(_ context.Context, rawParams []byte) (any, error) {
	var params protocol.DocumentHighlightParams
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return nil, err
	}

	content, ok := s.getDocument(params.TextDocument.URI)
	if !ok {
		return nil, nil
	}

	word := parser.WordAtPosition(content, int(params.Position.Line), int(params.Position.Character))
	if word == "" {
		return []protocol.DocumentHighlight{}, nil
	}

	return highlightsOf(content, word), nil
}

// highlightsOf returns every whole-identifier occurrence of word in content.
func highlightsOf(content, word string) []protocol.DocumentHighlight {
	out := []protocol.DocumentHighlight{}

	// A trailing "\r" is deliberately left on. It cannot affect either answer:
	// it is never an identifier byte, so it never blocks a match at the end of
	// a CRLF line, and it can never precede one, so it never shifts a column.
	// entryRange in references.go does trim, because it has a whole-line
	// fallback whose end a "\r" would move; there is no such fallback here.
	for i, text := range strings.Split(content, "\n") {
		for at := 0; ; {
			start, end, found := identSpanFrom(text, word, at)
			if !found {
				break
			}

			out = append(out, protocol.DocumentHighlight{
				Range: protocol.Range{
					Start: protocol.Position{Line: uint32(i), Character: utf16Len(text[:start])},
					End:   protocol.Position{Line: uint32(i), Character: utf16Len(text[:end])},
				},
				Kind: protocol.DocumentHighlightKindText,
			})

			// Resume after this match, not one byte on: overlapping matches of
			// the same identifier are impossible, and restarting at start+1
			// re-scans the whole word for nothing.
			at = end
		}
	}

	return out
}
