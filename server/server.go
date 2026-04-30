package server

import (
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

	mu              sync.RWMutex
	documents       map[uri.URI]string
	workspaceRoots  []string
	ExtraIndexPaths []string
	index           *Index
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

func (s *Server) isExtraIndexPath(root string) bool {
	for _, p := range s.ExtraIndexPaths {
		if p == root {
			return true
		}
	}
	return false
}
