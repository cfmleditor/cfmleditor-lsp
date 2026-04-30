package server

import (
	"context"
	"encoding/json"
	"strings"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
	"go.uber.org/zap"
)

func (s *Server) Handler() jsonrpc2.Handler {
	return func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
		switch req.Method() {
		case protocol.MethodInitialize:
			return s.handleInitialize(ctx, reply, req)
		case protocol.MethodInitialized:
			return reply(ctx, nil, nil)
		case protocol.MethodShutdown:
			return reply(ctx, nil, nil)
		case protocol.MethodExit:
			return reply(ctx, nil, nil)
		case protocol.MethodTextDocumentDidOpen:
			return s.handleDidOpen(ctx, reply, req)
		case protocol.MethodTextDocumentDidChange:
			return s.handleDidChange(ctx, reply, req)
		case protocol.MethodTextDocumentDidClose:
			return s.handleDidClose(ctx, reply, req)
		case protocol.MethodTextDocumentCompletion:
			return s.handleCompletion(ctx, reply, req)
		case protocol.MethodTextDocumentDefinition:
			return s.handleDefinition(ctx, reply, req)
		case protocol.MethodTextDocumentDocumentSymbol:
			return s.handleDocumentSymbol(ctx, reply, req)
		case protocol.MethodWorkspaceSymbol:
			return s.handleWorkspaceSymbol(ctx, reply, req)
		case protocol.MethodTextDocumentHover:
			return s.handleHover(ctx, reply, req)
		case protocol.MethodWorkspaceDidChangeWorkspaceFolders:
			return s.handleDidChangeWorkspaceFolders(ctx, reply, req)
		default:
			return jsonrpc2.MethodNotFoundHandler(ctx, reply, req)
		}
	}
}

func (s *Server) handleInitialize(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.InitializeParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	s.initialized = true

	s.logger.Info("initialize params workspace folders", zap.Int("count", len(params.WorkspaceFolders)))
	for i, folder := range params.WorkspaceFolders {
		s.logger.Info("workspace folder", zap.Int("index", i), zap.String("name", folder.Name), zap.String("uri", string(folder.URI)))
	}

	for _, folder := range params.WorkspaceFolders {
		root := strings.TrimPrefix(string(folder.URI), "file://")
		s.workspaceRoots = append(s.workspaceRoots, root)
	}
	if len(s.workspaceRoots) == 0 && params.RootURI != "" {
		s.workspaceRoots = append(s.workspaceRoots, strings.TrimPrefix(string(params.RootURI), "file://"))
	}

	go s.indexWorkspace()

	s.logger.Info("CFML LSP initialized", zap.Strings("workspaceRoots", s.workspaceRoots))

	return reply(ctx, protocol.InitializeResult{
		Capabilities: s.capabilities(),
		ServerInfo: &protocol.ServerInfo{
			Name:    "cfmleditor-lsp",
			Version: "0.1.0",
		},
	}, nil)
}

func (s *Server) handleDidOpen(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidOpenTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	docURI := uri.URI(params.TextDocument.URI)
	s.setDocument(docURI, params.TextDocument.Text)
	s.reindexIfCFC(docURI, params.TextDocument.Text)
	s.logger.Info("document opened", zap.String("uri", string(docURI)))

	return reply(ctx, nil, nil)
}

func (s *Server) handleDidChange(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidChangeTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	docURI := uri.URI(params.TextDocument.URI)
	if len(params.ContentChanges) > 0 {
		newText := params.ContentChanges[len(params.ContentChanges)-1].Text
		s.setDocument(docURI, newText)
		s.reindexIfCFC(docURI, newText)
	}

	return reply(ctx, nil, nil)
}

func (s *Server) handleDidClose(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidCloseTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	docURI := uri.URI(params.TextDocument.URI)
	s.removeDocument(docURI)
	s.logger.Info("document closed", zap.String("uri", string(docURI)))

	return reply(ctx, nil, nil)
}

func (s *Server) reindexIfCFC(docURI uri.URI, content string) {
	if !strings.HasSuffix(strings.ToLower(string(docURI)), ".cfc") {
		return
	}
	if len(s.WorkspaceFolders) > 0 && !s.isIncludedPath(string(docURI)) {
		return
	}
	s.index.indexFile(docURI, content)
}

func (s *Server) handleDidChangeWorkspaceFolders(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidChangeWorkspaceFoldersParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	for _, removed := range params.Event.Removed {
		root := strings.TrimPrefix(removed.URI, "file://")
		if !s.isWorkspaceFolder(root) {
			s.index.removeFilesUnder(removed.URI)
		}
		s.mu.Lock()
		for i, r := range s.workspaceRoots {
			if r == root {
				s.workspaceRoots = append(s.workspaceRoots[:i], s.workspaceRoots[i+1:]...)
				break
			}
		}
		s.mu.Unlock()
		s.logger.Info("workspace folder removed", zap.String("uri", removed.URI))
	}

	for _, added := range params.Event.Added {
		root := strings.TrimPrefix(added.URI, "file://")
		s.mu.Lock()
		s.workspaceRoots = append(s.workspaceRoots, root)
		s.mu.Unlock()
		s.indexRoot(root)
		s.logger.Info("workspace folder added", zap.String("uri", added.URI))
	}

	return reply(ctx, nil, nil)
}
