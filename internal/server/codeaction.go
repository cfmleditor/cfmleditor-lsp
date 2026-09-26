package server

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"path/filepath"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
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
	char := byteCol(content, line, params.Range.Start.Character)
	docURI := string(params.TextDocument.URI)

	var actions []protocol.CodeAction

	// Offered only on a line that looks as if it holds a call, so it is not
	// noise on every line of the file; the cursor need not be on the call
	// itself. See lineMayHoldCall for why this is not a parse.
	if lineMayHoldCall(content, line) {
		title := fmt.Sprintf("Explain call resolution on line %d", line+1)
		actions = append(actions, protocol.CodeAction{
			Title: title,

			Command: protocol.Command{
				Title:     title,
				Command:   "cfmleditor.explainCall",
				Arguments: lspAnyArgs(docURI, line),
			},
		})
	}

	// The file and workspace actions come last, and wherever the cursor is:
	// they are about the file or the project, not the word under the cursor. A
	// client with no way to run a server command of its own, such as Zed
	// before its LSP command picker, reaches them only from here.
	general := append(fileActions(docURI), workspaceActions()...)

	word := parser.WordAtPosition(content, line, char)
	if word == "" {
		return append(actions, general...), nil
	}

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

	return append(actions, general...), nil
}

// fileActions offers what is about the whole file. The dependency graph here
// is the file-level form of cfmleditor.exportDeps, which takes the document
// URI alone; the word actions above pass a function name as well.
func fileActions(docURI string) []protocol.CodeAction {
	title := "Export dependency graph for " + filepath.Base(cfpath.FromURI(docURI))

	return []protocol.CodeAction{{
		Title: title,

		Command: protocol.Command{
			Title:     title,
			Command:   "cfmleditor.exportDeps",
			Arguments: lspAnyArgs(docURI),
		},
	}}
}

// workspaceActions offers the commands that walk the whole workspace: the
// parse-error scan, which publishes its findings as diagnostics, and the
// generated known-issues reports, which write their files beside
// .cfmleditor.json (see handleExport).
func workspaceActions() []protocol.CodeAction {
	cmds := []struct{ title, command string }{
		{"Scan workspace for parse errors", "cfmleditor.scanWorkspace"},
		{"Export unresolved calls report for the workspace", "cfmleditor.exportUnresolved"},
		{"Export CFLint report for the workspace", "cfmleditor.exportCFLint"},
	}

	out := make([]protocol.CodeAction, 0, len(cmds))

	for _, r := range cmds {
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
