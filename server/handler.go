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

	for _, folder := range params.WorkspaceFolders {
		root := strings.TrimPrefix(string(folder.URI), "file://")
		s.workspaceRoots = append(s.workspaceRoots, root)
	}
	if len(s.workspaceRoots) == 0 && params.RootURI != "" {
		s.workspaceRoots = append(s.workspaceRoots, strings.TrimPrefix(string(params.RootURI), "file://"))
	}

	go s.indexWorkspace()

	s.logger.Info("CFML LSP initialized")

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
	s.index.removeFile(docURI)
	s.logger.Info("document closed", zap.String("uri", string(docURI)))

	return reply(ctx, nil, nil)
}

func (s *Server) reindexIfCFC(docURI uri.URI, content string) {
	if strings.HasSuffix(strings.ToLower(string(docURI)), ".cfc") {
		s.index.indexFile(docURI, content)
	}
}
