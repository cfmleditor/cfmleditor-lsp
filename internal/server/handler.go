package server

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/cfmleditor/cfmleditor-lsp/internal/cache"
	"github.com/cfmleditor/cfmleditor-lsp/internal/cfparser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
	"go.uber.org/zap"
)

// zapAdapter adapts *zap.Logger to cfparser.Logger.
type zapAdapter struct{ l *zap.Logger }

func (z *zapAdapter) Info(msg string, kv ...interface{}) { z.l.Sugar().Infow(msg, kv...) }
func (z *zapAdapter) Warn(msg string, kv ...interface{}) { z.l.Sugar().Warnw(msg, kv...) }

// Handler returns a jsonrpc2.Handler that dispatches LSP method calls.
func (s *Server) Handler() jsonrpc2.Handler {
	return func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) (err error) {
		defer func() {
			if r := recover(); r != nil {
				s.logger.Error("handler panic", zap.String("method", req.Method()), zap.Any("panic", r))
				err = reply(ctx, nil, fmt.Errorf("internal error: %v", r))
			}
		}()
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

	s.logger.Debug("initialize params workspace folders", zap.Int("count", len(params.WorkspaceFolders)))
	for i, folder := range params.WorkspaceFolders {
		s.logger.Debug("workspace folder", zap.Int("index", i), zap.String("name", folder.Name), zap.String("uri", string(folder.URI)))
	}

	for _, folder := range params.WorkspaceFolders {
		root := strings.TrimPrefix(string(folder.URI), "file://")
		s.workspaceRoots = append(s.workspaceRoots, root)
	}

	if len(s.workspaceRoots) == 0 && params.RootURI != "" { //nolint:all // this is for compatibility
		s.workspaceRoots = append(s.workspaceRoots, strings.TrimPrefix(string(params.RootURI), "file://")) //nolint:all // this is for compatibility
	}

	s.safeGo("indexWorkspace", s.indexWorkspace)
	s.safeGo("initLinter", s.initLinter)

	// In standalone mode, load config from workspace roots if not already configured
	if len(s.ComponentResolvers) == 0 {
		s.loadConfigFromRoots()
	}

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

	if !isCFMLFile(string(docURI)) {
		return reply(ctx, nil, nil)
	}

	s.setDocument(docURI, params.TextDocument.Text)

	pr := s.parseContent(docURI, params.TextDocument.Text)
	pr.Log = &zapAdapter{s.logger}
	s.logger.Debug("document opened: parse result",
		zap.String("uri", string(docURI)),
		zap.Int("funcs", len(pr.Funcs)),
		zap.Int("refs", len(pr.Refs)),
		zap.Int("resolvers", len(pr.Resolvers)))
	for _, ref := range pr.Refs {
		s.logger.Debug("document opened: ref", zap.String("var", ref.Variable), zap.String("component", ref.Component))
	}
	s.mu.Lock()
	s.parseResults[docURI] = pr
	s.funcRanges[docURI] = scopesToFuncRanges(pr)
	s.mu.Unlock()

	s.reindexFromParseResult(docURI, pr)

	s.safeGo("rebuildFileCompletionCacheFromPR", func() { s.rebuildFileCompletionCacheFromPR(docURI, pr) })
	s.logger.Debug("document opened", zap.String("uri", string(docURI)))

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

	if !isCFMLFile(string(docURI)) {
		return reply(ctx, nil, nil)
	}

	s.mu.RLock()
	pr := s.parseResults[docURI]
	s.mu.RUnlock()

	content, ok := s.getDocument(docURI)
	if !ok {
		return reply(ctx, nil, nil)
	}

	s.mu.Lock()
	// Track rapid sequential changes: reset window if >500ms since last burst.
	now := time.Now()
	if ws, ok := s.changeWindowStart[docURI]; !ok || now.Sub(ws) > 200*time.Millisecond {
		s.changeWindowStart[docURI] = now
		s.changeCount[docURI] = 0
	}
	s.changeCount[docURI]++
	rapidChanges := s.changeCount[docURI] > 5 || len(params.ContentChanges) > 50
	s.mu.Unlock()

	totalBytes := 0
	for _, c := range params.ContentChanges {
		totalBytes += len(c.Text)
	}
	s.logger.Debug("didChange",
		zap.String("uri", string(docURI)),
		zap.Int("changeCount", s.changeCount[docURI]),
		zap.Int("edits", len(params.ContentChanges)),
		zap.Int("bytes", totalBytes),
	)

	var editLine int
	var lastKind cfparser.EditKind

	if rapidChanges {
		// Too many changes in quick succession — just apply text and defer reindex.
		for _, change := range params.ContentChanges {
			if change.Range == (protocol.Range{}) && change.RangeLength == 0 {
				content = change.Text
			} else {
				content = applyEdit(content, change.Range, change.Text)
			}
		}
		s.setDocument(docURI, content)
		s.mu.Lock()
		if timer, ok := s.cacheTimers[docURI]; ok {
			timer.Stop()
		}
		s.cacheTimers[docURI] = time.AfterFunc(200*time.Millisecond, func() {
			defer func() {
				if r := recover(); r != nil {
					s.logger.Error("goroutine panic", zap.String("label", "rapidChangeTimer"), zap.Any("panic", r))
				}
			}()
			s.mu.Lock()
			delete(s.changeCount, docURI)
			delete(s.changeWindowStart, docURI)
			existingPR := s.parseResults[docURI]
			s.mu.Unlock()
			latest, ok := s.getDocument(docURI)
			if !ok {
				return
			}
			if existingPR != nil {
				existingPR.ApplyFullReplace(latest)
				s.mu.Lock()
				s.funcRanges[docURI] = scopesToFuncRanges(existingPR)
				s.mu.Unlock()
				s.reindexFromParseResult(docURI, existingPR)
			}
		})
		s.mu.Unlock()
		return reply(ctx, nil, nil)
	}

	for _, change := range params.ContentChanges {
		if change.Range == (protocol.Range{}) && change.RangeLength == 0 { //nolint:gocritic // ifElseChain: intentional for clarity
			// Full document replacement
			content = change.Text
			if pr != nil {
				pr.ApplyFullReplace(content)
				lastKind = cfparser.EditFull
			}
		} else {
			content = applyEdit(content, change.Range, change.Text)
			editLine = int(change.Range.Start.Line)
			if pr != nil {
				lastKind = pr.ApplyEdit(
					int(change.Range.Start.Line), int(change.Range.Start.Character),
					int(change.Range.End.Line), int(change.Range.End.Character),
					change.Text,
				)
			}
		}
	}

	s.setDocument(docURI, content)

	if pr == nil {
		// No cached parse result — fall back to full parse
		pr = s.parseContent(docURI, content)
		pr.Log = &zapAdapter{s.logger}
		s.mu.Lock()
		s.parseResults[docURI] = pr
		s.funcRanges[docURI] = scopesToFuncRanges(pr)
		s.mu.Unlock()
		s.reindexFromParseResult(docURI, pr)
		return reply(ctx, nil, nil)
	}

	// Update funcRanges from the parse result
	switch lastKind {
	case cfparser.EditGlobal, cfparser.EditFull:
		// Signatures changed — update funcRanges and the index
		s.mu.Lock()
		s.funcRanges[docURI] = scopesToFuncRanges(pr)
		s.mu.Unlock()
		s.reindexFromParseResult(docURI, pr)
	case cfparser.EditInFunc:
		// Only function body changed — shift index lines and rebuild local vars
		lineDelta := 0
		for _, change := range params.ContentChanges {
			if change.Range != (protocol.Range{}) || change.RangeLength != 0 {
				oldLines := int(change.Range.End.Line) - int(change.Range.Start.Line)
				newLines := strings.Count(change.Text, "\n")
				lineDelta += newLines - oldLines
			}
		}
		if lineDelta != 0 {
			s.index.ShiftLines(docURI, editLine, lineDelta)
		}
		s.debounceCacheRebuild(docURI, content, editLine)
	}

	return reply(ctx, nil, nil)
}

const cacheRebuildDelay = 150 * time.Millisecond

// debounceCacheRebuild resets the debounce timer for a file's completion cache rebuild.
func (s *Server) debounceCacheRebuild(docURI uri.URI, content string, editLine int) {
	s.mu.Lock()
	if t, ok := s.cacheTimers[docURI]; ok {
		t.Stop()
	}
	s.cacheTimers[docURI] = time.AfterFunc(cacheRebuildDelay, func() {
		defer func() {
			if r := recover(); r != nil {
				s.logger.Error("goroutine panic", zap.String("label", "cacheRebuild"), zap.Any("panic", r))
			}
		}()
		s.rebuildCompletionCache(docURI, content, editLine)
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
	s.mu.Lock()
	delete(s.parseResults, docURI)
	s.mu.Unlock()
	s.logger.Debug("document closed", zap.String("uri", string(docURI)))

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
	s.invalidateResolveCache()

	// Invalidate Application.cfc mappings cache if an Application file was saved
	filePath := strings.TrimPrefix(string(docURI), "file://")
	baseName := filepath.Base(filePath)
	if strings.EqualFold(baseName, "Application.cfc") || strings.EqualFold(baseName, "Application.cfm") {
		cfpath.InvalidateAppMappingsCache()
	}

	if isCFMLFile(filePath) {
		s.safeGo("runDiagnostics", func() { s.runDiagnostics(ctx, docURI) })
		s.safeGo("rebuildFileCompletionCache", func() { s.rebuildFileCompletionCache(docURI) })
	}

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
	s.logger.Debug("cflint scan starting", zap.String("file", filePath))

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
		s.logger.Debug("cflint scan cancelled", zap.String("file", filePath))
		return
	}

	if err != nil {
		s.logger.Warn("cflint scan failed", zap.String("file", filePath), zap.Error(err))
		return
	}

	if diags == nil {
		diags = []protocol.Diagnostic{}
	}

	s.logger.Debug("cflint scan complete", zap.String("file", filePath), zap.Int("issues", len(diags)))

	_ = s.conn.Notify(ctx, protocol.MethodTextDocumentPublishDiagnostics, &protocol.PublishDiagnosticsParams{
		URI:         protocol.DocumentURI(docURI),
		Diagnostics: diags,
	})
}

func (s *Server) reindexIfCFC(docURI uri.URI, content string) {
	if !isCFMLFile(string(docURI)) {
		return
	}
	if isCFCFile(string(docURI)) && len(s.WorkspaceFolders) > 0 && !s.isIncludedPath(string(docURI)) {
		return
	}
	s.index.IndexFile(docURI, content)
}

// reindexFromParseResult updates the index using an existing ParseResult.
func (s *Server) reindexFromParseResult(docURI uri.URI, pr *cfparser.ParseResult) {
	if !isCFMLFile(string(docURI)) {
		return
	}
	s.index.IndexFileFromResult(docURI, pr.Funcs, pr.Refs)
	s.index.SetThisVars(docURI, pr.ThisVars())
	// Only register as entity if within ORM scope and workspace
	if isCFCFile(string(docURI)) && pr.Persistent {
		filePath := strings.TrimPrefix(string(docURI), "file://")
		if s.isOrmPath(filePath) {
			s.index.SetEntity(cfcNameFromURI(docURI), docURI)
		}
	}
}

// resolverRefs scans content for assignments whose RHS matches a component resolver.
// scopesToFuncRanges converts ParseResult scopes to cache.FuncRange slice.
func scopesToFuncRanges(pr *cfparser.ParseResult) []cache.FuncRange {
	ranges := make([]cache.FuncRange, 0, len(pr.Scopes))
	for _, sc := range pr.Scopes {
		name := ""
		for _, f := range pr.Funcs {
			if int(f.Line) == sc.Start {
				name = f.Name
				break
			}
		}
		ranges = append(ranges, cache.FuncRange{
			Name:  name,
			Start: sc.Start,
			End:   sc.End,
		})
	}
	return ranges
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

// safeGo runs fn in a goroutine with panic recovery.
func (s *Server) safeGo(label string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				s.logger.Error("goroutine panic", zap.String("label", label), zap.Any("panic", r))
			}
		}()
		fn()
	}()
}
