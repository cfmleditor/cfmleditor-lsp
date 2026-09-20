package server

import (
	"context"
	"encoding/json/v2"
	"strings"

	"go.lsp.dev/protocol"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
)

func (s *Server) handleOnTypeFormatting(_ context.Context, rawParams []byte) (any, error) {
	var params protocol.DocumentOnTypeFormattingParams
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return nil, err
	}

	if params.Ch != ">" {
		return []protocol.TextEdit{}, nil
	}

	content, ok := s.getDocument(params.TextDocument.URI)
	if !ok {
		return []protocol.TextEdit{}, nil
	}

	return onTypeEdits(content, int(params.Position.Line), int(params.Position.Character)), nil
}

// onTypeEdits is the handler's decision, separated from its plumbing so a test
// can measure what it costs without a JSON round trip swamping the answer.
func onTypeEdits(content string, line, char int) []protocol.TextEdit {
	// One line, for the reason duplicateGtCompletion gives: this runs on every
	// '>' the user types, and splitting the document to read one row of it was
	// 400us and 519KB of that keystroke on a 32,000-line component.
	//
	// The bound is explicit rather than left to the slice below. A line past
	// the end of the document comes back empty, and a character past the end of
	// its line is a position no editor should send — but the old spelling
	// happened to tolerate one, because the line it sliced carried its newline
	// and so was a byte longer than the line the position describes.
	lineText := parser.LineTextAt(content, line)
	if char > len(lineText) {
		return []protocol.TextEdit{}
	}

	// Find the next '>' after the cursor on the same line.
	rest := lineText[char:]

	idx := strings.IndexByte(rest, '>')
	if idx == -1 {
		return []protocol.TextEdit{}
	}

	// Verify we're inside a tag.
	before := lineText[:char]

	openIdx := strings.LastIndexByte(before, '<')
	if openIdx == -1 {
		return []protocol.TextEdit{}
	}

	if strings.ContainsRune(lineText[openIdx:char-1], '>') {
		return []protocol.TextEdit{}
	}

	middle := rest[:idx]

	// Only act if the content between typed '>' and existing '>' is whitespace-only.
	if strings.TrimSpace(middle) != "" {
		return []protocol.TextEdit{}
	}

	// Remove the typed '>' and the whitespace and the original '>'.
	// Result: cursor ends up after the original '>' position (which stays).
	endChar := char + idx + 1
	edits := []protocol.TextEdit{{
		Range: protocol.Range{
			Start: protocol.Position{Line: uint32(line), Character: uint32(char - 1)},
			End:   protocol.Position{Line: uint32(line), Character: uint32(endChar)},
		},
		NewText: ">",
	}}

	return edits
}
