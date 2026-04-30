package server

import (
	"path/filepath"
	"strings"
	"sync"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
	"go.uber.org/zap"
)

type Server struct {
	conn        jsonrpc2.Conn
	logger      *zap.Logger
	initialized bool

	mu               sync.RWMutex
	documents        map[uri.URI]string
	workspaceRoots   []string
	WorkspaceFolders []string // project folders from config
	IndexGlobs       []string // optional glob filters (absolute paths)
	index            *Index
}

// NewServer creates a new LSP server. If sharedIndex is non-nil it is used
// instead of creating a private index, allowing multiple sessions to share one.
func NewServer(conn jsonrpc2.Conn, logger *zap.Logger, sharedIndex ...*Index) *Server {
	idx := NewIndex()
	if len(sharedIndex) > 0 && sharedIndex[0] != nil {
		idx = sharedIndex[0]
	}
	return &Server{
		conn:      conn,
		logger:    logger,
		documents: make(map[uri.URI]string),
		index:     idx,
	}
}

func (s *Server) capabilities() protocol.ServerCapabilities {
	return protocol.ServerCapabilities{
		TextDocumentSync: protocol.TextDocumentSyncOptions{
			OpenClose: true,
			Change:    protocol.TextDocumentSyncKindFull,
		},
		CompletionProvider: &protocol.CompletionOptions{
			TriggerCharacters: []string{"<", " ", "/"},
		},
		DefinitionProvider:      true,
		DocumentSymbolProvider:  true,
		WorkspaceSymbolProvider: true,
		HoverProvider:           true,
		Workspace: &protocol.ServerCapabilitiesWorkspace{
			WorkspaceFolders: &protocol.ServerCapabilitiesWorkspaceFolders{
				Supported:           true,
				ChangeNotifications: true,
			},
		},
	}
}

func (s *Server) getDocument(docURI uri.URI) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	content, ok := s.documents[docURI]
	return content, ok
}

func (s *Server) setDocument(docURI uri.URI, content string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.documents[docURI] = content
}

func (s *Server) removeDocument(docURI uri.URI) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.documents, docURI)
}

func (s *Server) isWorkspaceFolder(root string) bool {
	for _, p := range s.WorkspaceFolders {
		if p == root {
			return true
		}
	}
	return false
}

// isIncludedPath checks whether a file URI should be indexed based on config.
func (s *Server) isIncludedPath(rawURI string) bool {
	filePath := strings.TrimPrefix(rawURI, "file://")
	// If index globs are defined, match against them
	if len(s.IndexGlobs) > 0 {
		return matchesGlob(filePath, s.IndexGlobs)
	}
	// Otherwise, any .cfc under a workspace folder is included
	for _, f := range s.WorkspaceFolders {
		if strings.HasPrefix(filePath, f+"/") {
			return true
		}
	}
	return false
}

func matchesGlob(filePath string, globs []string) bool {
	for _, g := range globs {
		if !strings.Contains(g, "**") {
			if matched, _ := filepath.Match(g, filePath); matched {
				return true
			}
			if strings.HasPrefix(filePath, g+"/") || filePath == g {
				return true
			}
			continue
		}
		idx := strings.Index(g, "**")
		base := filepath.Clean(g[:idx])
		suffix := g[idx+2:]
		suffix = strings.TrimPrefix(suffix, string(filepath.Separator))
		if !strings.HasPrefix(filePath, base+"/") && filePath != base {
			continue
		}
		if suffix == "" {
			return true
		}
		if matched, _ := filepath.Match(suffix, filepath.Base(filePath)); matched {
			return true
		}
	}
	return false
}
