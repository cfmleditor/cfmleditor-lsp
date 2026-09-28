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

// handleFoldingRange answers textDocument/foldingRange from the parser: every
// function (the cached ParseResult's scopes), every comment, and in CFScript
// every block, closure, literal, multi-line argument list and switch case
// (parser.StructureSpans). Tags in a page do not fold yet; FOLDING-PLAN.md has
// the plan.
//
// Folding used to come from the tree-sitter CST, which cost a full parse per
// request, twice over for a script-syntax component.
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
// A construct folds from its first line to the line before its last, so a
// closing `}`, `)`, `]` or `</cffunction>` stays on screen: a fold that hides
// the delimiter reads as though the construct had been deleted. A switch case
// keeps its last line too, which is usually its `break`. A comment folds to its
// last line, which carries content of its own.
//
// Functions come from the parse's scopes as well as from the brace pass: a
// tag-syntax <cffunction> has no braces, and a script function is found by
// both, which the dedupe below collapses.
func foldingRanges(pr *parser.ParseResult, content string) []protocol.FoldingRange {
	spans := parser.StructureSpans(content)
	folds := make([]protocol.FoldingRange, 0, len(pr.Scopes)+len(spans))

	for _, sc := range pr.Scopes {
		if sc.End-1 > sc.Start {
			folds = append(folds, protocol.FoldingRange{
				StartLine: conv.Uint32(sc.Start),
				EndLine:   conv.Uint32(sc.End - 1),
			})
		}
	}

	for _, sp := range spans {
		f := protocol.FoldingRange{StartLine: conv.Uint32(sp.Start), EndLine: conv.Uint32(sp.End - 1)}
		if sp.Kind == parser.SpanComment {
			f.EndLine, f.Kind = conv.Uint32(sp.End), protocol.FoldingRangeKindComment
		}

		if f.EndLine > f.StartLine {
			folds = append(folds, f)
		}
	}

	slices.SortStableFunc(folds, func(a, b protocol.FoldingRange) int {
		return cmp.Or(cmp.Compare(a.StartLine, b.StartLine), cmp.Compare(b.EndLine, a.EndLine))
	})

	// The same lines are one fold: a script function is both a scope and a
	// brace block, and a closure passed to a call can share both its lines
	// with the call.
	return slices.CompactFunc(folds, func(a, b protocol.FoldingRange) bool {
		return a.StartLine == b.StartLine && a.EndLine == b.EndLine
	})
}
