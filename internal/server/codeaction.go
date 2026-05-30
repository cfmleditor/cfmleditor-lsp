package server

import (
	"context"
	"encoding/json"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

func (s *Server) handleCodeAction(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.CodeActionParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	content, ok := s.getDocument(params.TextDocument.URI)
	if !ok {
		return reply(ctx, nil, nil)
	}

	line := int(params.Range.Start.Line)
	char := int(params.Range.Start.Character)

	word := parser.WordAtPosition(content, line, char)
	if word == "" {
		return reply(ctx, nil, nil)
	}

	docURI := string(params.TextDocument.URI)

	var actions []protocol.CodeAction

	// If on a function name with a qualifier, offer component-level actions
	if qualifier := parser.QualifierBeforeWord(content, line, char); qualifier != "" {
		actions = append(actions, protocol.CodeAction{
			Title: "Find all references to " + word,

			Command: &protocol.Command{
				Title:     "Find all references to " + word,
				Command:   "cfmleditor.findRefs",
				Arguments: []any{word, docURI},
			},
		})
		actions = append(actions, protocol.CodeAction{
			Title: "Export dependency graph for " + qualifier + "." + word,

			Command: &protocol.Command{
				Title:     "Export dependency graph for " + qualifier + "." + word,
				Command:   "cfmleditor.exportDeps",
				Arguments: []any{docURI, word},
			},
		})
	} else {
		// Unqualified — offer function-level actions
		actions = append(actions, protocol.CodeAction{
			Title: "Find all calls to " + word,

			Command: &protocol.Command{
				Title:     "Find all calls to " + word,
				Command:   "cfmleditor.findRefs",
				Arguments: []any{word, docURI},
			},
		})
		actions = append(actions, protocol.CodeAction{
			Title: "Export dependency graph for " + word,

			Command: &protocol.Command{
				Title:     "Export dependency graph for " + word,
				Command:   "cfmleditor.exportDeps",
				Arguments: []any{docURI, word},
			},
		})
	}

	return reply(ctx, actions, nil)
}
