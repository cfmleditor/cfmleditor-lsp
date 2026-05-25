package server

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func (s *Server) handleDocumentLink(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DocumentLinkParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	content, ok := s.getDocument(uri.URI(params.TextDocument.URI))
	if !ok {
		return reply(ctx, nil, nil)
	}

	// Use cached parse result for global-scope links; scan function bodies on demand
	docURI := uri.URI(params.TextDocument.URI)
	s.mu.RLock()
	pr := s.parseResults[docURI]
	s.mu.RUnlock()

	var docLinks []parser.DocumentLink
	if pr != nil {
		// Global scope links from parse time
		docLinks = append(docLinks, pr.Links...)
		// Function body links (lightweight string scan only)
		for _, sc := range pr.Scopes {
			_, funcLinks := pr.FuncRefs(sc.Start, sc.End)
			docLinks = append(docLinks, funcLinks...)
		}
	} else {
		// Fallback: full scan
		docLinks = parser.ExtractLinks(content)
	}

	var links []protocol.DocumentLink
	for _, dl := range docLinks {
		links = append(links, protocol.DocumentLink{
			Range: protocol.Range{
				Start: protocol.Position{Line: dl.Line, Character: dl.Start},
				End:   protocol.Position{Line: dl.Line, Character: dl.End},
			},
			Tooltip: dl.Path,
			Data: map[string]string{
				"path": dl.Path,
				"uri":  string(params.TextDocument.URI),
			},
		})
	}

	return reply(ctx, links, nil)
}

func (s *Server) handleDocumentLinkResolve(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var link protocol.DocumentLink
	if err := json.Unmarshal(req.Params(), &link); err != nil {
		return reply(ctx, nil, err)
	}

	data, _ := link.Data.(map[string]interface{})
	if data == nil {
		return reply(ctx, link, nil)
	}
	filePath, _ := data["path"].(string)
	docURI, _ := data["uri"].(string)
	if filePath == "" || docURI == "" {
		return reply(ctx, link, nil)
	}

	baseDir := filepath.Dir(strings.TrimPrefix(docURI, "file://"))
	target := s.resolveLink(filePath, baseDir)
	if target != "" {
		link.Target = protocol.DocumentURI(uri.URI("file://" + target))
	}
	return reply(ctx, link, nil)
}

func (s *Server) resolveLink(filePath, baseDir string) string {
	candidate := filepath.Join(baseDir, filePath)
	if _, err := s.FS.Stat(candidate); err == nil {
		return candidate
	}
	if appDir := s.getResolver().FindApplicationRoot(baseDir); appDir != "" {
		candidate = filepath.Join(appDir, filePath)
		if _, err := s.FS.Stat(candidate); err == nil {
			return candidate
		}
	}
	for _, root := range s.WorkspaceFolders {
		candidate = filepath.Join(root, filePath)
		if _, err := s.FS.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}
