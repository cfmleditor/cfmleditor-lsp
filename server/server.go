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

	mu        sync.RWMutex
	documents map[uri.URI]string
}

func NewServer(conn jsonrpc2.Conn, logger *zap.Logger) *Server {
	return &Server{
		conn:      conn,
		logger:    logger,
		documents: make(map[uri.URI]string),
	}
}

func (s *Server) capabilities() protocol.ServerCapabilities {
	return protocol.ServerCapabilities{
		TextDocumentSync: protocol.TextDocumentSyncOptions{
			OpenClose: true,
			Change:    protocol.TextDocumentSyncKindFull,
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
