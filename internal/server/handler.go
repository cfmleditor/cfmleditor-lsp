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
	"github.com/cfmleditor/cfmleditor-lsp/internal/refs"
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
		start := time.Now()
		defer func() {
			if r := recover(); r != nil {
				s.logger.Error("handler panic", zap.String("method", req.Method()), zap.Any("panic", r))
				err = reply(ctx, nil, fmt.Errorf("internal error: %v", r))
			}
			if dur := time.Since(start); dur > 100*time.Millisecond {
				s.logger.Warn("slow request", zap.String("method", req.Method()), zap.Duration("dur", dur))
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
		case protocol.MethodTextDocumentSignatureHelp:
			return s.handleSignatureHelp(ctx, reply, req)
		case protocol.MethodTextDocumentDocumentLink:
			return s.handleDocumentLink(ctx, reply, req)
		case protocol.MethodDocumentLinkResolve:
			return s.handleDocumentLinkResolve(ctx, reply, req)
		case protocol.MethodTextDocumentCodeAction:
			return s.handleCodeAction(ctx, reply, req)
		case protocol.MethodWorkspaceDidChangeWorkspaceFolders:
			return s.handleDidChangeWorkspaceFolders(ctx, reply, req)
		case protocol.MethodWorkspaceExecuteCommand:
			return s.handleExecuteCommand(ctx, reply, req)
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
		s.notify(ctx, protocol.MethodTextDocumentPublishDiagnostics, &protocol.PublishDiagnosticsParams{
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
	s.notify(scanCtx, protocol.MethodProgress, map[string]interface{}{
		"token": "cflint",
		"value": map[string]interface{}{"kind": "begin", "title": "CFLint", "message": filepath.Base(filePath)},
	})

	diags, err := s.linter.Scan(scanCtx, filePath)

	s.notify(scanCtx, protocol.MethodProgress, map[string]interface{}{
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

	s.notify(ctx, protocol.MethodTextDocumentPublishDiagnostics, &protocol.PublishDiagnosticsParams{
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
func (s *Server) handleExecuteCommand(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.ExecuteCommandParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	switch params.Command {
	case "cfmleditor.reindex":
		s.invalidateResolveCache()
		cfpath.InvalidateAppMappingsCache()
		s.safeGo("reindex", s.indexWorkspace)
		s.logger.Info("reindex triggered via command")
		return reply(ctx, nil, nil)
	case "cfmleditor.format":
		if len(params.Arguments) == 0 {
			return reply(ctx, nil, fmt.Errorf("cfmleditor.format requires a document URI argument"))
		}
		docURI, _ := params.Arguments[0].(string)
		if docURI == "" {
			return reply(ctx, nil, fmt.Errorf("cfmleditor.format: invalid URI argument"))
		}
		content, ok := s.getDocument(uri.URI(docURI))
		if !ok {
			return reply(ctx, nil, nil)
		}
		formatted, err := formatDocument(content, protocol.FormattingOptions{InsertSpaces: true, TabSize: uint32(s.Formatting.IndentWidth)}, s.Formatting)
		if err != nil {
			return reply(ctx, nil, err)
		}
		if formatted == content {
			return reply(ctx, nil, nil)
		}
		lines := countNewlines(content)
		s.call(ctx, protocol.MethodWorkspaceApplyEdit, &protocol.ApplyWorkspaceEditParams{
			Label: "Format document",
			Edit: protocol.WorkspaceEdit{
				Changes: map[uri.URI][]protocol.TextEdit{
					uri.URI(docURI): {{
						Range: protocol.Range{
							Start: protocol.Position{Line: 0, Character: 0},
							End:   protocol.Position{Line: uint32(lines + 1), Character: 0},
						},
						NewText: formatted,
					}},
				},
			},
		}, nil)
		return reply(ctx, nil, nil)
	case "cfmleditor.showComponentPath":
		if len(params.Arguments) == 0 {
			return reply(ctx, nil, fmt.Errorf("cfmleditor.showComponentPath requires a dot-path argument"))
		}
		dotPath, _ := params.Arguments[0].(string)
		if dotPath == "" {
			return reply(ctx, nil, fmt.Errorf("cfmleditor.showComponentPath: invalid argument"))
		}
		var baseDir string
		if len(params.Arguments) > 1 {
			if docURI, ok := params.Arguments[1].(string); ok {
				baseDir = filepath.Dir(strings.TrimPrefix(docURI, "file://"))
			}
		}
		if baseDir == "" && len(s.WorkspaceFolders) > 0 {
			baseDir = s.WorkspaceFolders[0]
		}
		resolved := s.getResolver().ComponentPath(dotPath, baseDir)
		if resolved == "" {
			s.notify(ctx, protocol.MethodWindowShowMessage, &protocol.ShowMessageParams{
				Type:    protocol.MessageTypeInfo,
				Message: fmt.Sprintf("Cannot resolve: %s", dotPath),
			})
		} else {
			s.notify(ctx, protocol.MethodWindowShowMessage, &protocol.ShowMessageParams{
				Type:    protocol.MessageTypeInfo,
				Message: fmt.Sprintf("%s → %s", dotPath, resolved),
			})
		}
		return reply(ctx, resolved, nil)
	case "cfmleditor.restartDaemon":
		s.notify(ctx, protocol.MethodWindowShowMessage, &protocol.ShowMessageParams{
			Type:    protocol.MessageTypeInfo,
			Message: "Restarting daemon: clearing all caches and re-indexing",
		})
		s.invalidateResolveCache()
		cfpath.InvalidateAppMappingsCache()
		s.mu.Lock()
		s.parseResults = make(map[uri.URI]*cfparser.ParseResult)
		s.funcRanges = make(map[uri.URI][]cache.FuncRange)
		s.mu.Unlock()
		s.compCache.InvalidateAll()
		s.safeGo("reindex", s.indexWorkspace)
		s.logger.Info("daemon restart triggered via command")
		return reply(ctx, nil, nil)
	case "cfmleditor.showResolvers":
		var lines []string
		lines = append(lines, fmt.Sprintf("Workspace folders: %v", s.WorkspaceFolders))
		if len(s.Mappings) > 0 {
			lines = append(lines, "Mappings:")
			for k, v := range s.Mappings {
				lines = append(lines, fmt.Sprintf("  %s → %s", k, v))
			}
		}
		if len(s.ComponentResolvers) > 0 {
			lines = append(lines, "Component resolvers:")
			for _, r := range s.ComponentResolvers {
				lines = append(lines, fmt.Sprintf("  match=%q resolve=%q prefix=%q", r.Match, r.Resolve, r.Prefix))
			}
		}
		if len(s.PropertyResolvers) > 0 {
			lines = append(lines, "Property resolvers:")
			for _, r := range s.PropertyResolvers {
				lines = append(lines, fmt.Sprintf("  match=%q resolve=%q attr=%q", r.Match, r.Resolve, r.Attribute))
			}
		}
		msg := strings.Join(lines, "\n")
		s.notify(ctx, protocol.MethodWindowShowMessage, &protocol.ShowMessageParams{
			Type:    protocol.MessageTypeInfo,
			Message: msg,
		})
		return reply(ctx, msg, nil)
	case "cfmleditor.showFileIndex":
		if len(params.Arguments) == 0 {
			return reply(ctx, nil, fmt.Errorf("cfmleditor.showFileIndex requires a document URI argument"))
		}
		docURI, _ := params.Arguments[0].(string)
		if docURI == "" {
			return reply(ctx, nil, fmt.Errorf("cfmleditor.showFileIndex: invalid argument"))
		}
		fileURI := uri.URI(docURI)
		funcs := s.index.FunctionsForFile(fileURI)
		refs := s.index.RefsForFile(fileURI)
		var lines []string
		lines = append(lines, fmt.Sprintf("File: %s", docURI))
		lines = append(lines, fmt.Sprintf("Functions (%d):", len(funcs)))
		for _, f := range funcs {
			lines = append(lines, fmt.Sprintf("  %s (line %d)", f.Name, f.Line))
		}
		lines = append(lines, fmt.Sprintf("Component refs (%d):", len(refs)))
		for _, r := range refs {
			lines = append(lines, fmt.Sprintf("  %s → %s (line %d)", r.Variable, r.Component, r.Line))
		}
		msg := strings.Join(lines, "\n")
		s.notify(ctx, protocol.MethodWindowShowMessage, &protocol.ShowMessageParams{
			Type:    protocol.MessageTypeInfo,
			Message: msg,
		})
		return reply(ctx, msg, nil)
	case "cfmleditor.showConnections":
		s.mu.RLock()
		openDocs := len(s.documents)
		s.mu.RUnlock()
		msg := fmt.Sprintf("Open documents: %d\nWorkspace folders: %d\nIndex globs: %d", openDocs, len(s.WorkspaceFolders), len(s.IndexGlobs))
		s.notify(ctx, protocol.MethodWindowShowMessage, &protocol.ShowMessageParams{
			Type:    protocol.MessageTypeInfo,
			Message: msg,
		})
		return reply(ctx, msg, nil)
	case "cfmleditor.openActiveApplicationFile":
		if len(params.Arguments) == 0 {
			return reply(ctx, nil, fmt.Errorf("cfmleditor.openActiveApplicationFile requires a document URI argument"))
		}
		docURI, _ := params.Arguments[0].(string)
		if docURI == "" {
			return reply(ctx, nil, nil)
		}
		baseDir := filepath.Dir(strings.TrimPrefix(docURI, "file://"))
		appDir := s.getResolver().FindApplicationRoot(baseDir)
		if appDir == "" {
			s.notify(ctx, protocol.MethodWindowShowMessage, &protocol.ShowMessageParams{
				Type:    protocol.MessageTypeInfo,
				Message: "No Application.cfc found",
			})
			return reply(ctx, nil, nil)
		}
		// Find the actual file
		for _, name := range []string{"Application.cfc", "Application.cfm"} {
			if _, err := s.FS.Stat(filepath.Join(appDir, name)); err == nil {
				target := "file://" + filepath.Join(appDir, name)
				s.call(ctx, "window/showDocument", map[string]interface{}{
					"uri":       target,
					"takeFocus": true,
				}, nil)
				return reply(ctx, target, nil)
			}
		}
		return reply(ctx, nil, nil)
	case "cfmleditor.goToMatchingTag":
		if len(params.Arguments) < 2 {
			return reply(ctx, nil, fmt.Errorf("cfmleditor.goToMatchingTag requires [documentURI, line, char]"))
		}
		docURI, _ := params.Arguments[0].(string)
		if docURI == "" {
			return reply(ctx, nil, nil)
		}
		content, ok := s.getDocument(uri.URI(docURI))
		if !ok {
			return reply(ctx, nil, nil)
		}
		var line, char int
		if len(params.Arguments) >= 3 {
			if v, ok := params.Arguments[1].(float64); ok {
				line = int(v)
			}
			if v, ok := params.Arguments[2].(float64); ok {
				char = int(v)
			}
		}
		pos := findMatchingTag(content, line, char)
		if pos == nil {
			return reply(ctx, nil, nil)
		}
		return reply(ctx, pos, nil)
	case "cfmleditor.copyPackage":
		if len(params.Arguments) == 0 {
			return reply(ctx, nil, fmt.Errorf("cfmleditor.copyPackage requires a document URI argument"))
		}
		docURI, _ := params.Arguments[0].(string)
		if docURI == "" {
			return reply(ctx, nil, nil)
		}
		filePath := strings.TrimPrefix(docURI, "file://")
		dotPath := s.fileToPackage(filePath)
		return reply(ctx, dotPath, nil)
	case "cfmleditor.findRefs":
		if len(params.Arguments) == 0 {
			return reply(ctx, nil, fmt.Errorf("cfmleditor.findRefs requires a function name argument"))
		}
		funcName, _ := params.Arguments[0].(string)
		if funcName == "" {
			return reply(ctx, nil, nil)
		}
		sourceURI := ""
		if len(params.Arguments) > 1 {
			sourceURI, _ = params.Arguments[1].(string)
		}
		s.logger.Debug("findRefs: searching", zap.String("funcName", funcName), zap.String("source", sourceURI), zap.Strings("roots", s.WorkspaceFolders))
		r := s.getResolver()
		entries := refs.Find(s.FS, s.WorkspaceFolders, refs.Options{
			FuncName:          funcName,
			Resolvers:         s.cfResolvers(),
			PropertyResolvers: s.cfPropertyResolvers(),
			VerifyCall: func(component, fn, fileDir string) bool {
				return r.HasFunction(component, fn, fileDir)
			},
		})
		s.logger.Debug("findRefs: complete", zap.String("funcName", funcName), zap.Int("results", len(entries)))
		for _, e := range entries {
			s.logger.Debug("findRefs: match", zap.String("file", e.File), zap.Uint32("line", e.Line), zap.Bool("resolved", e.Resolved), zap.String("call", e.Call))
		}
		var lines []string
		lines = append(lines, fmt.Sprintf("Calls to '%s': %d match(es)", funcName, len(entries)))
		for _, e := range entries {
			rel := e.File
			for _, root := range s.WorkspaceFolders {
				if r, err := filepath.Rel(root, e.File); err == nil && len(r) < len(rel) {
					rel = r
				}
			}
			marker := ""
			if !e.Resolved {
				marker = " [unresolved]"
			}
			lines = append(lines, fmt.Sprintf("  %s:%d%s  %s", rel, e.Line+1, marker, e.Call))
		}
		msg := strings.Join(lines, "\n")
		// Build Mermaid diagram
		var mermaid []string
		mermaid = append(mermaid, "graph LR")
		targetNode := strings.ReplaceAll(funcName, ".", "_")
		mermaid = append(mermaid, fmt.Sprintf("    %s[%s]", targetNode, funcName))
		seen := make(map[string]bool)
		for i, e := range entries {
			rel := e.File
			for _, root := range s.WorkspaceFolders {
				if r, err := filepath.Rel(root, e.File); err == nil && len(r) < len(rel) {
					rel = r
				}
			}
			label := rel
			if e.Function != "" {
				label += "::" + e.Function
			}
			key := label
			if seen[key] {
				continue
			}
			seen[key] = true
			nodeID := fmt.Sprintf("ref%d", i)
			style := "-->"
			if !e.Resolved {
				style = "-.->"
			}
			mermaid = append(mermaid, fmt.Sprintf("    %s[%s] %s %s", nodeID, label, style, targetNode))
		}
		diagram := strings.Join(mermaid, "\n")
		output := msg + "\n\n" + diagram
		s.call(ctx, "window/showDocument", map[string]interface{}{
			"uri":       "untitled:refs-" + funcName,
			"external":  false,
			"takeFocus": true,
		}, nil)
		s.call(ctx, protocol.MethodWorkspaceApplyEdit, &protocol.ApplyWorkspaceEditParams{
			Label: "Find all calls to " + funcName,
			Edit: protocol.WorkspaceEdit{
				DocumentChanges: []protocol.TextDocumentEdit{{
					TextDocument: protocol.OptionalVersionedTextDocumentIdentifier{
						TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI("untitled:refs-" + funcName)},
					},
					Edits: []protocol.TextEdit{{Range: protocol.Range{}, NewText: output}},
				}},
			},
		}, nil)
		return reply(ctx, msg, nil)
	case "cfmleditor.exportDeps":
		if len(params.Arguments) == 0 {
			return reply(ctx, nil, fmt.Errorf("cfmleditor.exportDeps requires a document URI"))
		}
		docURI, _ := params.Arguments[0].(string)
		if docURI == "" {
			return reply(ctx, nil, nil)
		}
		fileURI := uri.URI(docURI)
		funcs := s.index.FunctionsForFile(fileURI)
		refs := s.index.RefsForFile(fileURI)
		var lines []string
		lines = append(lines, "graph LR")
		fileName := filepath.Base(strings.TrimPrefix(docURI, "file://"))
		fileNode := strings.ReplaceAll(fileName, ".", "_")
		lines = append(lines, fmt.Sprintf("    %s[%s]", fileNode, fileName))
		seen := make(map[string]bool)
		for _, ref := range refs {
			if seen[ref.Component] {
				continue
			}
			seen[ref.Component] = true
			compNode := strings.ReplaceAll(ref.Component, ".", "_")
			lines = append(lines, fmt.Sprintf("    %s --> %s[%s]", fileNode, compNode, ref.Component))
		}
		_ = funcs // available for future use
		msg := strings.Join(lines, "\n")
		s.notify(ctx, protocol.MethodWindowShowMessage, &protocol.ShowMessageParams{
			Type: protocol.MessageTypeInfo, Message: msg,
		})
		return reply(ctx, msg, nil)
	default:
		return reply(ctx, nil, fmt.Errorf("unknown command: %s", params.Command))
	}
}

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

// findMatchingTag finds the matching open/close tag at the given position.
func findMatchingTag(content string, line, char int) map[string]interface{} {
	lineText := lineAtOffset(content, line)
	if lineText == "" {
		return nil
	}
	pos := min(char, len(lineText))

	tagStart := -1
	for i := pos; i >= 0; i-- {
		if i < len(lineText) && lineText[i] == '<' {
			tagStart = i
			break
		}
	}
	if tagStart < 0 {
		return nil
	}

	isClose := tagStart+1 < len(lineText) && lineText[tagStart+1] == '/'
	nameStart := tagStart + 1
	if isClose {
		nameStart = tagStart + 2
	}
	nameEnd := nameStart
	for nameEnd < len(lineText) && lineText[nameEnd] != ' ' && lineText[nameEnd] != '>' && lineText[nameEnd] != '/' {
		nameEnd++
	}
	if nameStart == nameEnd {
		return nil
	}
	tagName := strings.ToLower(lineText[nameStart:nameEnd])

	offset := 0
	for l := 0; l < line; l++ {
		idx := strings.IndexByte(content[offset:], '\n')
		if idx < 0 {
			return nil
		}
		offset += idx + 1
	}
	cursorOffset := offset + pos

	if isClose {
		depth := 0
		i := cursorOffset - 1
		for i >= 0 {
			if i > 0 && content[i-1] == '<' && content[i] == '/' {
				end := strings.IndexByte(content[i:], '>')
				if end > 0 {
					name := strings.ToLower(strings.TrimSpace(content[i+1 : i+end]))
					if name == tagName {
						depth++
					}
				}
			} else if content[i] == '<' && (i+1 >= len(content) || content[i+1] != '/') {
				end := i + 1
				for end < len(content) && content[end] != ' ' && content[end] != '>' && content[end] != '/' {
					end++
				}
				name := strings.ToLower(content[i+1 : end])
				if name == tagName {
					if depth == 0 {
						return offsetToPosition(content, i)
					}
					depth--
				}
			}
			i--
		}
	} else {
		searchStart := offset + nameEnd
		for searchStart < len(content) && content[searchStart] != '>' {
			searchStart++
		}
		searchStart++
		depth := 0
		i := searchStart
		for i < len(content) {
			if content[i] == '<' {
				if i+1 < len(content) && content[i+1] == '/' {
					end := i + 2
					for end < len(content) && content[end] != '>' && content[end] != ' ' {
						end++
					}
					name := strings.ToLower(content[i+2 : end])
					if name == tagName {
						if depth == 0 {
							return offsetToPosition(content, i)
						}
						depth--
					}
				} else {
					end := i + 1
					for end < len(content) && content[end] != ' ' && content[end] != '>' && content[end] != '/' {
						end++
					}
					name := strings.ToLower(content[i+1 : end])
					if name == tagName {
						depth++
					}
				}
			}
			i++
		}
	}
	return nil
}

func offsetToPosition(content string, offset int) map[string]interface{} {
	line := 0
	lastNL := -1
	for i := 0; i < offset && i < len(content); i++ {
		if content[i] == '\n' {
			line++
			lastNL = i
		}
	}
	char := offset - lastNL - 1
	return map[string]interface{}{"line": line, "character": char}
}

// fileToPackage converts a file path to a CFML dot-path relative to workspace.
func (s *Server) fileToPackage(filePath string) string {
	for _, root := range s.WorkspaceFolders {
		if strings.HasPrefix(filePath, root+"/") {
			rel := filePath[len(root)+1:]
			rel = strings.TrimSuffix(rel, filepath.Ext(rel))
			return strings.ReplaceAll(rel, "/", ".")
		}
	}
	base := filepath.Base(filePath)
	return strings.TrimSuffix(base, filepath.Ext(base))
}
