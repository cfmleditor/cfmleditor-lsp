package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/garethedwards/cfmleditor-lsp/cfml"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func (s *Server) handleHover(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.HoverParams
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

	if e, ok := cfml.LookupFunction(word); ok {
		return reply(ctx, &protocol.Hover{
			Contents: protocol.MarkupContent{
				Kind:  protocol.Markdown,
				Value: fmt.Sprintf("**%s**\n\n```cfml\n%s\n```\n\n%s", e.Name, e.Syntax, e.Doc()),
			},
		}, nil)
	}

	if e, ok := cfml.LookupTag(word); ok {
		return reply(ctx, &protocol.Hover{
			Contents: protocol.MarkupContent{
				Kind:  protocol.Markdown,
				Value: fmt.Sprintf("**<%s>**\n\n%s", e.Name, e.Doc()),
			},
		}, nil)
	}

	return reply(ctx, nil, nil)
}
