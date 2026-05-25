package server

import (
	"context"
	"encoding/json"
	"strings"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

func (s *Server) handleOnTypeFormatting(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DocumentOnTypeFormattingParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	if params.Ch != ">" {
		return reply(ctx, []protocol.TextEdit{}, nil)
	}

	content, ok := s.getDocument(params.TextDocument.URI)
	if !ok {
		return reply(ctx, []protocol.TextEdit{}, nil)
	}

	line := int(params.Position.Line)
	char := int(params.Position.Character)

	lines := strings.SplitAfter(content, "\n")
	if line >= len(lines) {
		return reply(ctx, []protocol.TextEdit{}, nil)
	}

	lineText := lines[line]

	// Find the next '>' after the cursor on the same line.
	rest := lineText[char:]

	idx := strings.IndexByte(rest, '>')
	if idx == -1 {
		return reply(ctx, []protocol.TextEdit{}, nil)
	}

	// Verify we're inside a tag.
	before := lineText[:char]

	openIdx := strings.LastIndexByte(before, '<')
	if openIdx == -1 {
		return reply(ctx, []protocol.TextEdit{}, nil)
	}

	if strings.ContainsRune(lineText[openIdx:char-1], '>') {
		return reply(ctx, []protocol.TextEdit{}, nil)
	}

	middle := rest[:idx]

	// Only act if the content between typed '>' and existing '>' is whitespace-only.
	if strings.TrimSpace(middle) != "" {
		return reply(ctx, []protocol.TextEdit{}, nil)
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

	return reply(ctx, edits, nil)
}
