package server

import (
	"context"
	json "github.com/go-json-experiment/json"
	"strings"

	"go.lsp.dev/protocol"
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

	line := int(params.Position.Line)
	char := int(params.Position.Character)

	lines := strings.SplitAfter(content, "\n")
	if line >= len(lines) {
		return []protocol.TextEdit{}, nil
	}

	lineText := lines[line]

	// Find the next '>' after the cursor on the same line.
	rest := lineText[char:]

	idx := strings.IndexByte(rest, '>')
	if idx == -1 {
		return []protocol.TextEdit{}, nil
	}

	// Verify we're inside a tag.
	before := lineText[:char]

	openIdx := strings.LastIndexByte(before, '<')
	if openIdx == -1 {
		return []protocol.TextEdit{}, nil
	}

	if strings.ContainsRune(lineText[openIdx:char-1], '>') {
		return []protocol.TextEdit{}, nil
	}

	middle := rest[:idx]

	// Only act if the content between typed '>' and existing '>' is whitespace-only.
	if strings.TrimSpace(middle) != "" {
		return []protocol.TextEdit{}, nil
	}

	// Remove the typed '>' and the whitespace and the original '>'.
	// Result: cursor ends up after the original '>' position (which stays).
	endChar := char + idx + 1
	edits := []protocol.TextEdit{{
		Range: protocol.Range{
			Start: protocol.Position{Line: params.Position.Line, Character: uint32(char - 1)},
			End:   protocol.Position{Line: params.Position.Line, Character: uint32(endChar)},
		},
		NewText: ">",
	}}

	return edits, nil
}
