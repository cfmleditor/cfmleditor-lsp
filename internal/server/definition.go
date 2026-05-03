package server

import (
	"context"
	"encoding/json"
	"strings"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func (s *Server) handleDefinition(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DefinitionParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	content, ok := s.getDocument(uri.URI(params.TextDocument.URI))
	if !ok {
		return reply(ctx, nil, nil)
	}

	word := wordAtPosition(content, int(params.Position.Line), int(params.Position.Character))
	if word == "" {
		return reply(ctx, nil, nil)
	}

	defs := s.index.Lookup(word)
	if len(defs) == 0 {
		return reply(ctx, nil, nil)
	}

	var locations []protocol.Location
	for _, d := range defs {
		locations = append(locations, protocol.Location{
			URI: protocol.DocumentURI(d.URI),
			Range: protocol.Range{
				Start: protocol.Position{Line: d.Line, Character: 0},
				End:   protocol.Position{Line: d.Line, Character: 0},
			},
		})
	}

	if len(locations) == 1 {
		return reply(ctx, locations[0], nil)
	}
	return reply(ctx, locations, nil)
}

func wordAtPosition(content string, line, char int) string {
	lines := strings.Split(content, "\n")
	if line >= len(lines) {
		return ""
	}
	lineText := lines[line]
	if char > len(lineText) {
		char = len(lineText)
	}

	start := char
	for start > 0 && isWordChar(lineText[start-1]) {
		start--
	}
	end := char
	for end < len(lineText) && isWordChar(lineText[end]) {
		end++
	}
	if start == end {
		return ""
	}
	return lineText[start:end]
}

func isWordChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}
