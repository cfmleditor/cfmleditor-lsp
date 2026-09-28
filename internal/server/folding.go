package server

import (
	"cmp"
	"context"
	"encoding/json/v2"
	"slices"

	"github.com/cfmleditor/cfmleditor-lsp/internal/conv"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/protocol"
)

// handleFoldingRange answers textDocument/foldingRange with a fold for every
// function and every comment spanning more than one line.
//
// The functions come from the document's cached ParseResult, which didChange
// keeps current, so a request costs a walk of its scopes rather than a parse.
// Folding used to come from the tree-sitter CST instead, which folds every
// block — ifs, loops, tags, literals — and cost a full parse per request, twice
// over for a script-syntax component. Functions and comments are about a
// quarter of what that folded; the rest is to be added from the parser.
func (s *Server) handleFoldingRange(_ context.Context, rawParams []byte) (any, error) {
	if !s.Features.Folding {
		return nil, nil
	}

	var params protocol.FoldingRangeParams
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return nil, err
	}

	docURI := params.TextDocument.URI

	defer s.lockDoc(docURI)()

	content, ok := s.getDocument(docURI)
	if !ok {
		return nil, nil
	}

	s.mu.RLock()
	pr := s.parseResults[docURI]
	s.mu.RUnlock()

	if pr == nil {
		pr = parser.Parse(docURI, content)
	}

	return foldingRanges(pr, content), nil
}

// foldingRanges builds the folds for one document, sorted by start line.
//
// A function folds from its first line to the line before its closing one, so
// the `}` or `</cffunction>` stays on screen: a fold that hides the delimiter
// reads as though the construct had been deleted. A comment folds to its last
// line, which carries content of its own.
func foldingRanges(pr *parser.ParseResult, content string) []protocol.FoldingRange {
	comments := parser.CommentSpans(content)
	folds := make([]protocol.FoldingRange, 0, len(pr.Scopes)+len(comments))

	for _, sc := range pr.Scopes {
		if sc.End-1 > sc.Start {
			folds = append(folds, protocol.FoldingRange{
				StartLine: conv.Uint32(sc.Start),
				EndLine:   conv.Uint32(sc.End - 1),
			})
		}
	}

	for _, c := range comments {
		folds = append(folds, protocol.FoldingRange{
			StartLine: conv.Uint32(c.Start),
			EndLine:   conv.Uint32(c.End),
			Kind:      protocol.FoldingRangeKindComment,
		})
	}

	slices.SortStableFunc(folds, func(a, b protocol.FoldingRange) int {
		return cmp.Or(cmp.Compare(a.StartLine, b.StartLine), cmp.Compare(b.EndLine, a.EndLine))
	})

	// Two scopes can name the same lines — a tag function's scope is found by
	// a search of its own — and a client should not be offered one fold twice.
	return slices.CompactFunc(folds, func(a, b protocol.FoldingRange) bool {
		return a.StartLine == b.StartLine && a.EndLine == b.EndLine
	})
}
