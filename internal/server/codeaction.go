package server

import (
	"context"
	json "github.com/go-json-experiment/json"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/protocol"
)

func (s *Server) handleCodeAction(_ context.Context, rawParams []byte) (any, error) {
	var params protocol.CodeActionParams
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return nil, err
	}

	content, ok := s.getDocument(params.TextDocument.URI)
	if !ok {
		return nil, nil
	}

	line := int(params.Range.Start.Line)
	char := int(params.Range.Start.Character)

	word := parser.WordAtPosition(content, line, char)
	if word == "" {
		return nil, nil
	}

	docURI := string(params.TextDocument.URI)

	var actions []protocol.CodeAction

	// If on a function name with a qualifier, offer component-level actions
	if qualifier := parser.QualifierBeforeWord(content, line, char); qualifier != "" {
		actions = append(actions, protocol.CodeAction{
			Title: "Find all references to " + word,

			Command: protocol.Command{
				Title:     "Find all references to " + word,
				Command:   "cfmleditor.findRefs",
				Arguments: lspAnyArgs(word, docURI),
			},
		})
		actions = append(actions, protocol.CodeAction{
			Title: "Export dependency graph for " + qualifier + "." + word,

			Command: protocol.Command{
				Title:     "Export dependency graph for " + qualifier + "." + word,
				Command:   "cfmleditor.exportDeps",
				Arguments: lspAnyArgs(docURI, word),
			},
		})
	} else {
		// Unqualified — offer function-level actions
		actions = append(actions, protocol.CodeAction{
			Title: "Find all calls to " + word,

			Command: protocol.Command{
				Title:     "Find all calls to " + word,
				Command:   "cfmleditor.findRefs",
				Arguments: lspAnyArgs(word, docURI),
			},
		})
		actions = append(actions, protocol.CodeAction{
			Title: "Export dependency graph for " + word,

			Command: protocol.Command{
				Title:     "Export dependency graph for " + word,
				Command:   "cfmleditor.exportDeps",
				Arguments: lspAnyArgs(docURI, word),
			},
		})
	}

	return actions, nil
}

// lspAnyArgs marshals each argument to a protocol.LSPAny for use in a Command's Arguments field.
func lspAnyArgs(args ...any) []protocol.LSPAny {
	out := make([]protocol.LSPAny, len(args))

	for i, a := range args {
		b, _ := json.Marshal(a)
		out[i] = protocol.LSPAny(b)
	}

	return out
}
