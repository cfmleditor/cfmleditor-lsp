package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/language"
	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	sitter "github.com/tree-sitter/go-tree-sitter"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func (s *Server) scanWorkspace(ctx context.Context) {
	var files []string

	for _, folder := range s.WorkspaceFolders {
		_ = s.FS.Walk(folder, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				name := info.Name()
				if name == ".git" || name == "node_modules" || name == ".svn" || name == "target" || name == "vendor" {
					return filepath.SkipDir
				}

				return nil
			}

			if cfpath.IsCFMLFile(path) {
				files = append(files, path)
			}

			return nil
		})
	}

	s.log.Info("scanWorkspace: starting", cflog.Int("files", len(files)))

	var totalErrors int

	for _, f := range files {
		data, err := s.FS.ReadFile(f)
		if err != nil {
			continue
		}

		tree := language.Parse(language.CFML, data, nil)
		if tree == nil {
			continue
		}

		if !tree.RootNode().HasError() {
			tree.Close()

			continue
		}

		diags := collectErrorDiagnostics(tree.RootNode(), data)
		tree.Close()

		if len(diags) > 0 {
			totalErrors += len(diags)
			s.notify(ctx, protocol.MethodTextDocumentPublishDiagnostics, &protocol.PublishDiagnosticsParams{
				URI:         uri.File(f),
				Diagnostics: diags,
			})
		}
	}

	s.log.Info("scanWorkspace: complete", cflog.Int("files", len(files)), cflog.Int("errors", totalErrors))
	s.notify(ctx, protocol.MethodWindowShowMessage, &protocol.ShowMessageParams{
		Type:    protocol.MessageTypeInfo,
		Message: fmt.Sprintf("Scan complete: %d parse errors in %d files", totalErrors, len(files)),
	})
}

func collectErrorDiagnostics(n *sitter.Node, src []byte) []protocol.Diagnostic {
	var diags []protocol.Diagnostic
	collectErrors(n, src, &diags)

	return diags
}

func collectErrors(n *sitter.Node, src []byte, diags *[]protocol.Diagnostic) {
	if n.IsError() || n.IsMissing() {
		start := n.StartPosition()
		end := n.EndPosition()

		snippet := string(src[n.StartByte():n.EndByte()])
		if len(snippet) > 50 {
			snippet = snippet[:50] + "..."
		}

		msg := "Parse error"
		if n.IsMissing() {
			msg = "Missing " + n.Kind()
		} else if snippet != "" {
			msg = "Parse error near " + strings.TrimSpace(snippet)
		}

		*diags = append(*diags, protocol.Diagnostic{
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(start.Row), Character: uint32(start.Column)},
				End:   protocol.Position{Line: uint32(end.Row), Character: uint32(end.Column)},
			},
			Severity: protocol.DiagnosticSeverityError,
			Source:   "cfmleditor",
			Message:  msg,
		})

		return
	}

	for i := uint(0); i < n.ChildCount(); i++ {
		collectErrors(n.Child(i), src, diags)
	}
}
