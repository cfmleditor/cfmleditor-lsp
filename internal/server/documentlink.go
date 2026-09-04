package server

import (
	"context"
	json "github.com/go-json-experiment/json"
	"path/filepath"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"go.lsp.dev/protocol"
)

func (s *Server) handleDocumentLink(_ context.Context, rawParams []byte) (any, error) {
	var params protocol.DocumentLinkParams
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return nil, err
	}

	content, ok := s.getDocument(params.TextDocument.URI)
	if !ok {
		return nil, nil
	}

	// Use cached parse result for global-scope links; scan function bodies on demand
	docURI := params.TextDocument.URI

	defer s.lockDoc(docURI)()

	s.mu.RLock()
	pr := s.parseResults[docURI]
	s.mu.RUnlock()

	var docLinks []parser.DocumentLink
	if pr != nil {
		// Global scope links from parse time
		docLinks = append(docLinks, pr.Links...)
		// Function body links (pre-computed at parse time)
		for _, sc := range pr.Scopes {
			docLinks = append(docLinks, pr.FuncLinks(sc.Start, sc.End)...)
		}
	} else {
		// Fallback: full scan
		docLinks = parser.ExtractLinks(content)
	}

	var links []protocol.DocumentLink

	for _, dl := range docLinks {
		tip := dl.Path
		data, _ := json.Marshal(map[string]string{
			"path": dl.Path,
			"uri":  string(params.TextDocument.URI),
		})
		links = append(links, protocol.DocumentLink{
			Range: protocol.Range{
				Start: protocol.Position{Line: dl.Line, Character: dl.Start},
				End:   protocol.Position{Line: dl.Line, Character: dl.End},
			},
			Tooltip: &tip,
			Data:    protocol.LSPAny(data),
		})
	}

	return links, nil
}

func (s *Server) handleDocumentLinkResolve(_ context.Context, rawParams []byte) (any, error) {
	var link protocol.DocumentLink
	if err := json.Unmarshal(rawParams, &link); err != nil {
		return nil, err
	}

	var data map[string]string

	_ = json.Unmarshal(link.Data, &data)

	if data == nil {
		return link, nil
	}

	filePath := data["path"]
	docURI := data["uri"]

	if filePath == "" || docURI == "" {
		return link, nil
	}

	baseDir := filepath.Dir(cfpath.FromURI(docURI))

	target := s.resolveLink(filePath, baseDir)
	if target != "" {
		targetURI := cfpath.ToURI(target)
		link.Target = &targetURI
	}

	return link, nil
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
