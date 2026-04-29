package server

import (
	"context"
	"encoding/json"
	"strings"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func (s *Server) handleDocumentSymbol(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DocumentSymbolParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	docURI := uri.URI(params.TextDocument.URI)
	content, ok := s.getDocument(docURI)
	if !ok {
		return reply(ctx, nil, nil)
	}

	defs := parseFunctionDefs(docURI, content)
	symbols := make([]protocol.DocumentSymbol, 0, len(defs))
	for _, d := range defs {
		r := protocol.Range{
			Start: protocol.Position{Line: d.Line, Character: 0},
			End:   protocol.Position{Line: d.Line, Character: 0},
		}
		symbols = append(symbols, protocol.DocumentSymbol{
			Name:           d.Name,
			Kind:           protocol.SymbolKindFunction,
			Range:          r,
			SelectionRange: r,
		})
	}

	return reply(ctx, symbols, nil)
}

func (s *Server) handleWorkspaceSymbol(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.WorkspaceSymbolParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	query := strings.ToLower(params.Query)
	symbols := []protocol.SymbolInformation{}

	for _, d := range s.index.AllFunctions() {
		if query != "" && !strings.Contains(strings.ToLower(d.Name), query) {
			continue
		}
		symbols = append(symbols, protocol.SymbolInformation{
			Name: d.Name,
			Kind: protocol.SymbolKindFunction,
			Location: protocol.Location{
				URI: protocol.DocumentURI(d.URI),
				Range: protocol.Range{
					Start: protocol.Position{Line: d.Line, Character: 0},
					End:   protocol.Position{Line: d.Line, Character: 0},
				},
			},
		})
	}

	return reply(ctx, symbols, nil)
}
