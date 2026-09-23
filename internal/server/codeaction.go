package server

import (
	"context"
	"encoding/json/v2"

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

	// The workspace reports come last, and wherever the cursor is: they are
	// about the project, not the word under it. A client with no way to run a
	// server command of its own, such as Zed, reaches them only from here.
	reports := workspaceReportActions()

	word := parser.WordAtPosition(content, line, char)
	if word == "" {
		return reports, nil
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

	// Writing the report is its own action, because the two above are ordinary
	// editor gestures and this one puts files in the user's source tree. It is
	// also the only way to reach the export from a client that cannot pass
	// command arguments of its own.
	actions = append(actions, protocol.CodeAction{
		Title: "Export references to " + word + " to a file",

		Command: protocol.Command{
			Title:     "Export references to " + word + " to a file",
			Command:   "cfmleditor.findRefs",
			Arguments: lspAnyArgs(word, docURI, true),
		},
	})

	return append(actions, reports...), nil
}

// workspaceReportActions offers the generated known-issues reports, which
// scan the whole workspace and write their files beside .cfmleditor.json (see
// handleExport).
func workspaceReportActions() []protocol.CodeAction {
	reports := []struct{ title, command string }{
		{"Export unresolved calls report for the workspace", "cfmleditor.exportUnresolved"},
		{"Export CFLint report for the workspace", "cfmleditor.exportCFLint"},
	}

	out := make([]protocol.CodeAction, 0, len(reports))

	for _, r := range reports {
		out = append(out, protocol.CodeAction{
			Title:   r.title,
			Command: protocol.Command{Title: r.title, Command: r.command},
		})
	}

	return out
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
