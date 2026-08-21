package server

import (
	"context"
	json "github.com/go-json-experiment/json"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/protocol"
)

func (s *Server) handleDocumentSymbol(_ context.Context, rawParams []byte) (any, error) {
	var params protocol.DocumentSymbolParams
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return nil, err
	}

	docURI := params.TextDocument.URI

	defer s.lockDoc(docURI)()

	s.mu.RLock()
	pr := s.parseResults[docURI]
	s.mu.RUnlock()

	var defs []parser.FunctionDef
	if pr != nil {
		defs = pr.Funcs
	} else {
		content, ok := s.getDocument(docURI)
		if !ok {
			return nil, nil
		}

		defs = parser.ParseFunctionDefs(docURI, content)
	}

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

	return symbols, nil
}

func (s *Server) handleWorkspaceSymbol(_ context.Context, rawParams []byte) (any, error) {
	var params protocol.WorkspaceSymbolParams
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return nil, err
	}

	query := strings.ToLower(params.Query)
	symbols := []protocol.SymbolInformation{}

	for _, d := range s.index.AllFunctions() {
		if query != "" && !containsFoldStr(d.Name, query) {
			continue
		}

		symbols = append(symbols, protocol.SymbolInformation{
			BaseSymbolInformation: protocol.BaseSymbolInformation{
				Name: d.Name,
				Kind: protocol.SymbolKindFunction,
			},
			Location: protocol.Location{
				URI: d.URI,
				Range: protocol.Range{
					Start: protocol.Position{Line: d.Line, Character: 0},
					End:   protocol.Position{Line: d.Line, Character: 0},
				},
			},
		})
	}

	return symbols, nil
}

// containsFoldStr reports whether s contains substr (case-insensitive, ASCII).
// substr must already be lowercase.
func containsFoldStr(s, substr string) bool {
	n := len(substr)
	if n == 0 {
		return true
	}

	end := len(s) - n
	for i := 0; i <= end; i++ {
		match := true

		for j := range n {
			if s[i+j]|0x20 != substr[j] {
				match = false

				break
			}
		}

		if match {
			return true
		}
	}

	return false
}
