package server

import (
	"context"
	"encoding/json"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func (s *Server) handleCodeAction(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.CodeActionParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	content, ok := s.getDocument(uri.URI(params.TextDocument.URI))
	if !ok {
		return reply(ctx, nil, nil)
	}

	line := int(params.Range.Start.Line)
	char := int(params.Range.Start.Character)
	word := wordAtPosition(content, line, char)
	if word == "" {
		return reply(ctx, nil, nil)
	}

	docURI := string(params.TextDocument.URI)
	var actions []protocol.CodeAction

	// If on a function name with a qualifier, offer component-level actions
	if qualifier := qualifierBeforeWord(content, line, char); qualifier != "" {
		actions = append(actions, protocol.CodeAction{
			Title: "Find all references to " + word,
			
			Command: &protocol.Command{
				Title:     "Find all references to " + word,
				Command:   "cfmleditor.findRefs",
				Arguments: []interface{}{word, docURI},
			},
		})
		actions = append(actions, protocol.CodeAction{
			Title: "Export dependency graph",
			
			Command: &protocol.Command{
				Title:     "Export dependency graph",
				Command:   "cfmleditor.exportDeps",
				Arguments: []interface{}{docURI},
			},
		})
	} else {
		// Unqualified — offer function-level actions
		actions = append(actions, protocol.CodeAction{
			Title: "Find all calls to " + word,
			
			Command: &protocol.Command{
				Title:     "Find all calls to " + word,
				Command:   "cfmleditor.findRefs",
				Arguments: []interface{}{word, docURI},
			},
		})
		actions = append(actions, protocol.CodeAction{
			Title: "Show dependencies for this file",
			
			Command: &protocol.Command{
				Title:     "Show dependencies for this file",
				Command:   "cfmleditor.exportDeps",
				Arguments: []interface{}{docURI},
			},
		})
	}

	return reply(ctx, actions, nil)
}
