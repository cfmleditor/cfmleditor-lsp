package server

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
	"go.uber.org/zap"
)

// Handler returns a jsonrpc2.Handler that dispatches LSP method calls.
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
		case protocol.MethodTextDocumentDidSave:
			return s.handleDidSave(ctx, reply, req)
		case protocol.MethodTextDocumentCompletion:
			return s.handleCompletion(ctx, reply, req)
		case protocol.MethodTextDocumentDefinition:
			return s.handleDefinition(ctx, reply, req)
		case protocol.MethodTextDocumentFormatting:
			return s.handleFormatting(ctx, reply, req)
		case protocol.MethodTextDocumentOnTypeFormatting:
			return s.handleOnTypeFormatting(ctx, reply, req)
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
	
	
	if len(s.workspaceRoots) == 0 && params.RootURI != "" { //nolint:all // this is for compatibility
		s.workspaceRoots = append(s.workspaceRoots, strings.TrimPrefix(string(params.RootURI), "file://")) //nolint:all // this is for compatibility
	}

	go s.indexWorkspace()
	go s.initLinter()

	s.logger.Info("CFML LSP initialized", zap.Strings("workspaceRoots", s.workspaceRoots))

	return reply(ctx, protocol.InitializeResult{
		Capabilities: s.capabilities(),
		ServerInfo: &protocol.ServerInfo{
			Name:    "cfmleditor-lsp",
			Version: s.Version,
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

	s.mu.Lock()
	s.funcRanges[docURI] = s.funcRangesForContent(docURI, params.TextDocument.Text)
	s.mu.Unlock()

	go s.rebuildCompletionCache(docURI, params.TextDocument.Text)
	s.logger.Info("document opened", zap.String("uri", string(docURI)))

	return reply(ctx, nil, nil)
}

func (s *Server) handleDidChange(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidChangeTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	docURI := uri.URI(params.TextDocument.URI)
	if len(params.ContentChanges) == 0 {
		return reply(ctx, nil, nil)
	}

	content, ok := s.getDocument(docURI)
	if !ok {
		return reply(ctx, nil, nil)
	}

	// Track edit lines before applying
	editInFuncBody := true
	var lineDelta int
	for _, change := range params.ContentChanges {
		if change.Range == (protocol.Range{}) && change.RangeLength == 0 {
			editInFuncBody = false
			content = change.Text
		} else {
			if editInFuncBody && !s.isEditInsideFuncBody(docURI, change.Range) {
				editInFuncBody = false
			}
			// Compute line delta: new lines in text minus replaced lines
			oldLines := int(change.Range.End.Line) - int(change.Range.Start.Line)
			newLines := strings.Count(change.Text, "\n")
			lineDelta += newLines - oldLines
			content = applyEdit(content, change.Range, change.Text)
		}
	}

	s.setDocument(docURI, content)

	if !editInFuncBody {
		s.reindexIfCFC(docURI, content)
		s.mu.Lock()
		s.funcRanges[docURI] = s.funcRangesForContent(docURI, content)
		s.mu.Unlock()
		// Immediate rebuild — structure changed
		go s.rebuildCompletionCache(docURI, content)
	} else if lineDelta != 0 {
		// Shift function ranges and index line numbers for functions below the edit
		editLine := int(params.ContentChanges[0].Range.Start.Line)
		s.mu.Lock()
		ranges := s.funcRanges[docURI]
		for i := range ranges {
			if ranges[i].Start > editLine {
				ranges[i].Start += lineDelta
				ranges[i].End += lineDelta
			} else if ranges[i].End >= editLine {
				// Edit is inside this function — only end shifts
				ranges[i].End += lineDelta
			}
		}
		s.mu.Unlock()
		s.index.ShiftLines(docURI, editLine, lineDelta)
		// Debounced rebuild — only local vars changed
		s.debounceCacheRebuild(docURI, content)
	} else {
		// No line change, still inside function — debounce
		s.debounceCacheRebuild(docURI, content)
	}

	return reply(ctx, nil, nil)
}

// isEditInsideFuncBody returns true if the edit range falls entirely within
// a known function body (not on the signature line itself).
func (s *Server) isEditInsideFuncBody(docURI uri.URI, r protocol.Range) bool {
	s.mu.RLock()
	ranges := s.funcRanges[docURI]
	s.mu.RUnlock()

	if len(ranges) == 0 {
		return false
	}

	startLine := int(r.Start.Line)
	endLine := int(r.End.Line)

	for _, f := range ranges {
		// Edit must be strictly inside the function (after signature line)
		if startLine > f.Start && endLine <= f.End {
			return true
		}
	}
	return false
}

const cacheRebuildDelay = 150 * time.Millisecond

// debounceCacheRebuild resets the debounce timer for a file's completion cache rebuild.
func (s *Server) debounceCacheRebuild(docURI uri.URI, content string) {
	s.mu.Lock()
	if t, ok := s.cacheTimers[docURI]; ok {
		t.Stop()
	}
	s.cacheTimers[docURI] = time.AfterFunc(cacheRebuildDelay, func() {
		s.rebuildCompletionCache(docURI, content)
	})
	s.mu.Unlock()
}

// applyEdit replaces the text in the given range with newText.
func applyEdit(content string, r protocol.Range, newText string) string {
	offset := positionToOffset(content, r.Start)
	endOffset := positionToOffset(content, r.End)
	return content[:offset] + newText + content[endOffset:]
}

// positionToOffset converts a line/character position to a byte offset.
func positionToOffset(content string, pos protocol.Position) int {
	line := int(pos.Line)
	char := int(pos.Character)
	offset := 0
	for i := 0; i < line; i++ {
		idx := strings.IndexByte(content[offset:], '\n')
		if idx < 0 {
			return len(content)
		}
		offset += idx + 1
	}
	offset += char
	if offset > len(content) {
		offset = len(content)
	}
	return offset
}

func (s *Server) handleDidClose(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidCloseTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	docURI := uri.URI(params.TextDocument.URI)
	s.removeDocument(docURI)
	s.logger.Info("document closed", zap.String("uri", string(docURI)))

	// Clear diagnostics on close
	if s.conn != nil {
		_ = s.conn.Notify(ctx, protocol.MethodTextDocumentPublishDiagnostics, &protocol.PublishDiagnosticsParams{
			URI:         protocol.DocumentURI(docURI),
			Diagnostics: []protocol.Diagnostic{},
		})
	}

	return reply(ctx, nil, nil)
}

func (s *Server) handleDidSave(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidSaveTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	docURI := uri.URI(params.TextDocument.URI)
	go s.runDiagnostics(ctx, docURI)

	return reply(ctx, nil, nil)
}

func (s *Server) runDiagnostics(ctx context.Context, docURI uri.URI) {
	if s.linter == nil || s.conn == nil {
		return
	}

	// Cancel any in-flight scan for this file
	s.mu.Lock()
	if cancel, ok := s.lintCancels[docURI]; ok {
		cancel()
	}
	scanCtx, cancel := context.WithCancel(ctx)
	s.lintCancels[docURI] = cancel
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.lintCancels, docURI)
		s.mu.Unlock()
		cancel()
	}()

	filePath := strings.TrimPrefix(string(docURI), "file://")
	s.logger.Info("cflint scan starting", zap.String("file", filePath))

	// Show progress
	_ = s.conn.Notify(scanCtx, protocol.MethodProgress, map[string]interface{}{
		"token": "cflint",
		"value": map[string]interface{}{"kind": "begin", "title": "CFLint", "message": filepath.Base(filePath)},
	})

	diags, err := s.linter.Scan(scanCtx, filePath)

	_ = s.conn.Notify(scanCtx, protocol.MethodProgress, map[string]interface{}{
		"token": "cflint",
		"value": map[string]interface{}{"kind": "end"},
	})

	if scanCtx.Err() != nil {
		s.logger.Info("cflint scan cancelled", zap.String("file", filePath))
		return
	}

	if err != nil {
		s.logger.Warn("cflint scan failed", zap.String("file", filePath), zap.Error(err))
		return
	}

	if diags == nil {
		diags = []protocol.Diagnostic{}
	}

	s.logger.Info("cflint scan complete", zap.String("file", filePath), zap.Int("issues", len(diags)))

	_ = s.conn.Notify(ctx, protocol.MethodTextDocumentPublishDiagnostics, &protocol.PublishDiagnosticsParams{
		URI:         protocol.DocumentURI(docURI),
		Diagnostics: diags,
	})
}

func (s *Server) reindexIfCFC(docURI uri.URI, content string) {
	lower := strings.ToLower(string(docURI))
	isCFC := strings.HasSuffix(lower, ".cfc")
	isCFML := isCFC || strings.HasSuffix(lower, ".cfm") || strings.HasSuffix(lower, ".cfml") || strings.HasSuffix(lower, ".cfs")
	if !isCFML {
		return
	}
	if isCFC && len(s.WorkspaceFolders) > 0 && !s.isIncludedPath(string(docURI)) {
		return
	}
	s.index.IndexFile(docURI, content)
}

func (s *Server) handleDidChangeWorkspaceFolders(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidChangeWorkspaceFoldersParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	for _, removed := range params.Event.Removed {
		root := strings.TrimPrefix(removed.URI, "file://")
		if !s.isWorkspaceFolder(root) {
			s.index.RemoveFilesUnder(removed.URI)
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
