package server

import (
	"context"
	"fmt"
	json "github.com/go-json-experiment/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cfmleditor/cfmleditor-lsp/internal/cache"
	"github.com/cfmleditor/cfmleditor-lsp/internal/deps"
	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"github.com/cfmleditor/cfmleditor-lsp/internal/refs"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// Handler returns a jsonrpc2.Handler that dispatches LSP method calls.
func (s *Server) Handler() jsonrpc2.Handler {
	return func(ctx context.Context, req *jsonrpc2.Request) (result any, err error) {
		start := time.Now()

		defer func() {
			if r := recover(); r != nil {
				s.log.Error("handler panic", cflog.String("method", req.Method()), cflog.Any("panic", r))
				result, err = nil, fmt.Errorf("internal error: %v", r)
			}

			if dur := time.Since(start); dur > 100*time.Millisecond {
				s.log.Warn("slow request", cflog.String("method", req.Method()), cflog.Duration("dur", dur))
			}
		}()

		switch req.Method() {
		case protocol.MethodInitialize:
			return s.handleInitialize(ctx, req.Params())
		case protocol.MethodInitialized:
			return nil, nil
		case protocol.MethodShutdown:
			return nil, nil
		case protocol.MethodExit:
			return nil, nil
		case protocol.MethodTextDocumentDidOpen:
			return s.handleDidOpen(ctx, req.Params())
		case protocol.MethodTextDocumentDidChange:
			return s.handleDidChange(ctx, req.Params())
		case protocol.MethodTextDocumentDidClose:
			return s.handleDidClose(ctx, req.Params())
		case protocol.MethodTextDocumentDidSave:
			return s.handleDidSave(ctx, req.Params())
		case protocol.MethodTextDocumentCompletion:
			return s.handleCompletion(ctx, req.Params())
		case protocol.MethodTextDocumentDefinition:
			return s.handleDefinition(ctx, req.Params())
		case protocol.MethodTextDocumentFormatting:
			return s.handleFormatting(ctx, req.Params())
		case protocol.MethodTextDocumentOnTypeFormatting:
			return s.handleOnTypeFormatting(ctx, req.Params())
		case protocol.MethodTextDocumentDocumentSymbol:
			return s.handleDocumentSymbol(ctx, req.Params())
		case protocol.MethodWorkspaceSymbol:
			return s.handleWorkspaceSymbol(ctx, req.Params())
		case protocol.MethodTextDocumentHover:
			return s.handleHover(ctx, req.Params())
		case protocol.MethodTextDocumentSignatureHelp:
			return s.handleSignatureHelp(ctx, req.Params())
		case protocol.MethodTextDocumentDocumentLink:
			return s.handleDocumentLink(ctx, req.Params())
		case protocol.MethodDocumentLinkResolve:
			return s.handleDocumentLinkResolve(ctx, req.Params())
		case protocol.MethodTextDocumentCodeAction:
			return s.handleCodeAction(ctx, req.Params())
		case protocol.MethodWorkspaceDidChangeWorkspaceFolders:
			return s.handleDidChangeWorkspaceFolders(ctx, req.Params())
		case protocol.MethodWorkspaceExecuteCommand:
			return s.handleExecuteCommand(ctx, req.Params())
		default:
			return jsonrpc2.MethodNotFoundHandler(ctx, req)
		}
	}
}

func (s *Server) handleInitialize(_ context.Context, rawParams []byte) (any, error) {
	var params protocol.InitializeParams
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return nil, err
	}

	s.initialized = true

	folders, _ := params.WorkspaceFolders.Get()

	s.log.Debug("initialize params workspace folders", cflog.Int("count", len(folders)))

	for i, folder := range folders {
		s.log.Debug("workspace folder", cflog.Int("index", i), cflog.String("name", folder.Name), cflog.String("uri", string(folder.URI)))
	}

	for _, folder := range folders {
		s.workspaceRoots = append(s.workspaceRoots, folder.URI.Path())
	}

	if len(s.workspaceRoots) == 0 && params.RootURI != nil && *params.RootURI != "" { //nolint:all // this is for compatibility
		s.workspaceRoots = append(s.workspaceRoots, params.RootURI.Path()) //nolint:all // this is for compatibility
	}

	s.safeGo("indexWorkspace", s.indexWorkspace)
	s.safeGo("initLinter", s.initLinter)

	// In standalone mode, load config from workspace roots if not already configured
	if len(s.ComponentResolvers) == 0 {
		s.loadConfigFromRoots()
	}

	s.log.Info("CFML LSP initialized", cflog.Strings("workspaceRoots", s.workspaceRoots))

	return protocol.InitializeResult{
		Capabilities: s.capabilities(),
		ServerInfo: protocol.ServerInfo{
			Name:    "cfmleditor-lsp",
			Version: optStr(s.Version),
		},
	}, nil
}

func (s *Server) handleDidOpen(_ context.Context, rawParams []byte) (any, error) { //nolint:unparam // notifications have no result; kept for uniform dispatch signature
	var params protocol.DidOpenTextDocumentParams
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return nil, err
	}

	docURI := params.TextDocument.URI

	if !cfpath.IsCFMLFile(string(docURI)) {
		return nil, nil
	}

	s.setDocument(docURI, params.TextDocument.Text)

	pr := s.parseContent(docURI, params.TextDocument.Text)
	s.log.Debug("document opened: parse result",
		cflog.String("uri", string(docURI)),
		cflog.Int("funcs", len(pr.Funcs)),
		cflog.Int("refs", len(pr.ComponentRefs)),
		cflog.Int("resolvers", len(pr.Resolvers)))

	for _, ref := range pr.ComponentRefs {
		s.log.Debug("document opened: ref", cflog.String("var", ref.Variable), cflog.String("component", ref.Component))
	}

	s.mu.Lock()
	s.parseResults[docURI] = pr
	s.funcRanges[docURI] = scopesToFuncRanges(pr)
	s.mu.Unlock()

	s.reindexFromParseResult(docURI, pr)

	s.safeGo("rebuildFileCompletionCacheFromPR", func() { s.rebuildFileCompletionCacheFromPR(docURI, pr) })
	s.log.Debug("document opened", cflog.String("uri", string(docURI)))

	return nil, nil
}

func (s *Server) handleDidChange(_ context.Context, rawParams []byte) (any, error) { //nolint:unparam // notifications have no result; kept for uniform dispatch signature
	var params protocol.DidChangeTextDocumentParams
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return nil, err
	}

	docURI := params.TextDocument.URI

	if len(params.ContentChanges) == 0 {
		return nil, nil
	}

	if !cfpath.IsCFMLFile(string(docURI)) {
		return nil, nil
	}

	s.mu.RLock()
	pr := s.parseResults[docURI]
	s.mu.RUnlock()

	content, ok := s.getDocument(docURI)
	if !ok {
		return nil, nil
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
		_, text, _ := changeRangeAndText(c)
		totalBytes += len(text)
	}

	s.log.Debug("didChange",
		cflog.String("uri", string(docURI)),
		cflog.Int("changeCount", s.changeCount[docURI]),
		cflog.Int("edits", len(params.ContentChanges)),
		cflog.Int("bytes", totalBytes),
	)

	var editLine int

	var lastKind parser.EditKind

	if rapidChanges {
		// Too many changes in quick succession — just apply text and defer reindex.
		for _, change := range params.ContentChanges {
			r, text, isFull := changeRangeAndText(change)
			if isFull {
				content = text
			} else {
				content = applyEdit(content, r, text)
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
					s.log.Error("goroutine panic", cflog.String("label", "rapidChangeTimer"), cflog.Any("panic", r))
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

		return nil, nil
	}

	for _, change := range params.ContentChanges {
		r, text, isFull := changeRangeAndText(change)
		if isFull { //nolint:gocritic // ifElseChain: intentional for clarity
			// Full document replacement
			content = text
			if pr != nil {
				pr.ApplyFullReplace(content)

				lastKind = parser.EditFull
			}
		} else {
			content = applyEdit(content, r, text)
			editLine = int(r.Start.Line)

			if pr != nil {
				lastKind = pr.ApplyEdit(
					int(r.Start.Line), int(r.Start.Character),
					int(r.End.Line), int(r.End.Character),
					text,
				)
			}
		}
	}

	s.setDocument(docURI, content)

	if pr == nil {
		// No cached parse result — fall back to full parse
		pr = s.parseContent(docURI, content)
		s.mu.Lock()
		s.parseResults[docURI] = pr
		s.funcRanges[docURI] = scopesToFuncRanges(pr)
		s.mu.Unlock()
		s.reindexFromParseResult(docURI, pr)

		return nil, nil
	}

	// Update funcRanges from the parse result
	switch lastKind {
	case parser.EditGlobal, parser.EditFull:
		// Signatures changed — update funcRanges and the index
		s.mu.Lock()
		s.funcRanges[docURI] = scopesToFuncRanges(pr)
		s.mu.Unlock()
		s.reindexFromParseResult(docURI, pr)
	case parser.EditInFunc:
		// Only function body changed — shift index lines and rebuild local vars
		lineDelta := 0

		for _, change := range params.ContentChanges {
			r, text, isFull := changeRangeAndText(change)
			if !isFull {
				oldLines := int(r.End.Line) - int(r.Start.Line)
				newLines := strings.Count(text, "\n")
				lineDelta += newLines - oldLines
			}
		}

		if lineDelta != 0 {
			s.index.ShiftLines(docURI, editLine, lineDelta)
		}

		s.debounceCacheRebuild(docURI, content, editLine)
	}

	return nil, nil
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
				s.log.Error("goroutine panic", cflog.String("label", "cacheRebuild"), cflog.Any("panic", r))
			}
		}()

		s.rebuildCompletionCache(docURI, content, editLine)
	})
	s.mu.Unlock()
}

// applyEdit replaces the text in the given range with newText.
func applyEdit(content string, r protocol.Range, newText string) string {
	return parser.ApplyEdit(content, int(r.Start.Line), int(r.Start.Character), int(r.End.Line), int(r.End.Character), newText)
}

func (s *Server) handleDidClose(ctx context.Context, rawParams []byte) (any, error) { //nolint:unparam // notifications have no result; kept for uniform dispatch signature
	var params protocol.DidCloseTextDocumentParams
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return nil, err
	}

	docURI := params.TextDocument.URI
	s.removeDocument(docURI)
	s.mu.Lock()
	delete(s.parseResults, docURI)
	s.mu.Unlock()
	s.log.Debug("document closed", cflog.String("uri", string(docURI)))

	// Clear diagnostics on close
	if s.conn != nil {
		s.notify(ctx, protocol.MethodTextDocumentPublishDiagnostics, &protocol.PublishDiagnosticsParams{
			URI:         docURI,
			Diagnostics: []protocol.Diagnostic{},
		})
	}

	return nil, nil
}

func (s *Server) handleDidSave(_ context.Context, rawParams []byte) (any, error) { //nolint:unparam // notifications have no result; kept for uniform dispatch signature
	var params protocol.DidSaveTextDocumentParams
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return nil, err
	}

	docURI := params.TextDocument.URI

	s.invalidateResolveCache()

	// Invalidate Application.cfc mappings cache if an Application file was saved
	filePath := docURI.Path()

	baseName := filepath.Base(filePath)
	if strings.EqualFold(baseName, "Application.cfc") || strings.EqualFold(baseName, "Application.cfm") {
		cfpath.InvalidateAppMappingsCache()
	}

	if cfpath.IsCFMLFile(filePath) {
		// runDiagnostics outlives this handler, so it must not carry the
		// request-scoped ctx: jsonrpc2 pools/resets incoming request contexts
		// once the handler returns, and using the stale context afterward
		// panics ("cannot create context from nil parent").
		s.safeGo("runDiagnostics", func() { s.runDiagnostics(context.Background(), docURI) })
		s.safeGo("rebuildFileCompletionCache", func() { s.rebuildFileCompletionCache(docURI) })
	}

	return nil, nil
}

func (s *Server) runDiagnostics(ctx context.Context, docURI uri.URI) {
	if s.linter == nil || s.conn == nil {
		s.log.Debug("cflint diagnostics skipped", cflog.String("reason", "linter not available (linting disabled or cflint not found)"))

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

	filePath := docURI.Path()
	s.log.Info("cflint scan starting", cflog.String("file", filePath))

	// Show progress
	s.notify(scanCtx, protocol.MethodProgress, map[string]any{
		"token": "cflint",
		"value": map[string]any{"kind": "begin", "title": "CFLint", "message": filepath.Base(filePath)},
	})

	diags, err := s.linter.Scan(scanCtx, filePath)

	s.notify(scanCtx, protocol.MethodProgress, map[string]any{
		"token": "cflint",
		"value": map[string]any{"kind": "end"},
	})

	if scanCtx.Err() != nil {
		s.log.Debug("cflint scan cancelled", cflog.String("file", filePath))

		return
	}

	if err != nil {
		s.log.Warn("cflint scan failed", cflog.String("file", filePath), cflog.Err(err))

		return
	}

	if diags == nil {
		diags = []protocol.Diagnostic{}
	}

	s.log.Info("cflint scan complete", cflog.String("file", filePath), cflog.Int("issues", len(diags)))

	s.notify(ctx, protocol.MethodTextDocumentPublishDiagnostics, &protocol.PublishDiagnosticsParams{
		URI:         docURI,
		Diagnostics: diags,
	})
}

func (s *Server) reindexIfCFC(docURI uri.URI, content string) {
	if !cfpath.IsCFMLFile(string(docURI)) {
		return
	}

	if cfpath.IsCFCFile(string(docURI)) && len(s.WorkspaceFolders) > 0 && !s.isIncludedPath(string(docURI)) {
		return
	}

	s.index.IndexFile(docURI, content)
}

// reindexFromParseResult updates the index using an existing ParseResult.
func (s *Server) reindexFromParseResult(docURI uri.URI, pr *parser.ParseResult) {
	if !cfpath.IsCFMLFile(string(docURI)) {
		return
	}

	s.index.IndexFileFromResult(docURI, pr.Funcs, pr.ComponentRefs)
	s.index.SetThisVars(docURI, pr.ThisVars())
	// Only register as entity if within ORM scope and workspace
	if cfpath.IsCFCFile(string(docURI)) && pr.Persistent {
		filePath := docURI.Path()
		if s.isOrmPath(filePath) {
			s.index.SetEntity(cfpath.CfcNameFromURI(string(docURI)), docURI)
		}
	}
}

// resolverRefs scans content for assignments whose RHS matches a component resolver.
// scopesToFuncRanges converts ParseResult scopes to cache.FuncRange slice.
func scopesToFuncRanges(pr *parser.ParseResult) []cache.FuncRange {
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

func (s *Server) handleDidChangeWorkspaceFolders(_ context.Context, rawParams []byte) (any, error) { //nolint:unparam // notifications have no result; kept for uniform dispatch signature
	var params protocol.DidChangeWorkspaceFoldersParams
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return nil, err
	}

	for _, removed := range params.Event.Removed {
		root := removed.URI.Path()
		if !s.isWorkspaceFolder(root) {
			s.index.RemoveFilesUnder(string(removed.URI))
		}

		s.mu.Lock()
		for i, r := range s.workspaceRoots {
			if r == root {
				s.workspaceRoots = append(s.workspaceRoots[:i], s.workspaceRoots[i+1:]...)

				break
			}
		}
		s.mu.Unlock()
		s.log.Info("workspace folder removed", cflog.String("uri", string(removed.URI)))
	}

	for _, added := range params.Event.Added {
		root := added.URI.Path()

		s.mu.Lock()
		s.workspaceRoots = append(s.workspaceRoots, root)
		s.mu.Unlock()
		s.indexRoot(root)
		s.log.Info("workspace folder added", cflog.String("uri", string(added.URI)))
	}

	return nil, nil
}

// safeGo runs fn in a goroutine with panic recovery.
func (s *Server) handleExecuteCommand(ctx context.Context, rawParams []byte) (any, error) {
	var params protocol.ExecuteCommandParams
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return nil, err
	}

	switch params.Command {
	case "cfmleditor.reindex":
		s.invalidateResolveCache()
		cfpath.InvalidateAppMappingsCache()
		s.safeGo("reindex", s.indexWorkspace)
		s.log.Info("reindex triggered via command")

		return nil, nil
	case "cfmleditor.format":
		if len(params.Arguments) == 0 {
			return nil, fmt.Errorf("cfmleditor.format requires a document URI argument")
		}

		docURI, _ := argString(params.Arguments, 0)
		if docURI == "" {
			return nil, fmt.Errorf("cfmleditor.format: invalid URI argument")
		}

		content, ok := s.getDocument(uri.URI(docURI))
		if !ok {
			return nil, nil
		}

		formatted, err := formatDocument(content, protocol.FormattingOptions{InsertSpaces: true, TabSize: uint32(s.Formatting.IndentWidth)}, s.Formatting)
		if err != nil {
			return nil, err
		}

		if formatted == content {
			return nil, nil
		}

		lines := parser.CountNewlines(content)
		formatLabel := "Format document"
		s.call(ctx, protocol.MethodWorkspaceApplyEdit, &protocol.ApplyWorkspaceEditParams{
			Label: &formatLabel,
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

		return nil, nil
	case "cfmleditor.showComponentPath":
		if len(params.Arguments) == 0 {
			return nil, fmt.Errorf("cfmleditor.showComponentPath requires a dot-path argument")
		}

		dotPath, _ := argString(params.Arguments, 0)
		if dotPath == "" {
			return nil, fmt.Errorf("cfmleditor.showComponentPath: invalid argument")
		}

		var baseDir string

		if len(params.Arguments) > 1 {
			if docURI, ok := argString(params.Arguments, 1); ok {
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

		return resolved, nil
	case "cfmleditor.restartDaemon":
		s.notify(ctx, protocol.MethodWindowShowMessage, &protocol.ShowMessageParams{
			Type:    protocol.MessageTypeInfo,
			Message: "Restarting daemon: clearing all caches and re-indexing",
		})
		s.invalidateResolveCache()
		cfpath.InvalidateAppMappingsCache()
		s.mu.Lock()
		s.parseResults = make(map[uri.URI]*parser.ParseResult)
		s.funcRanges = make(map[uri.URI][]cache.FuncRange)
		s.mu.Unlock()
		s.compCache.InvalidateAll()
		s.safeGo("reindex", s.indexWorkspace)
		s.log.Info("daemon restart triggered via command")

		return nil, nil
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

		return msg, nil
	case "cfmleditor.showFileIndex":
		if len(params.Arguments) == 0 {
			return nil, fmt.Errorf("cfmleditor.showFileIndex requires a document URI argument")
		}

		docURI, _ := argString(params.Arguments, 0)
		if docURI == "" {
			return nil, fmt.Errorf("cfmleditor.showFileIndex: invalid argument")
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

		return msg, nil
	case "cfmleditor.showConnections":
		s.mu.RLock()
		openDocs := len(s.documents)
		s.mu.RUnlock()
		msg := fmt.Sprintf("Open documents: %d\nWorkspace folders: %d\nIndex globs: %d", openDocs, len(s.WorkspaceFolders), len(s.IndexGlobs))
		s.notify(ctx, protocol.MethodWindowShowMessage, &protocol.ShowMessageParams{
			Type:    protocol.MessageTypeInfo,
			Message: msg,
		})

		return msg, nil
	case "cfmleditor.openActiveApplicationFile":
		if len(params.Arguments) == 0 {
			return nil, fmt.Errorf("cfmleditor.openActiveApplicationFile requires a document URI argument")
		}

		docURI, _ := argString(params.Arguments, 0)
		if docURI == "" {
			return nil, nil
		}

		baseDir := filepath.Dir(strings.TrimPrefix(docURI, "file://"))

		appDir := s.getResolver().FindApplicationRoot(baseDir)
		if appDir == "" {
			s.notify(ctx, protocol.MethodWindowShowMessage, &protocol.ShowMessageParams{
				Type:    protocol.MessageTypeInfo,
				Message: "No Application.cfc found",
			})

			return nil, nil
		}
		// Find the actual file
		for _, name := range []string{"Application.cfc", "Application.cfm"} {
			if _, err := s.FS.Stat(filepath.Join(appDir, name)); err == nil {
				target := "file://" + filepath.Join(appDir, name)
				s.call(ctx, "window/showDocument", map[string]any{
					"uri":       target,
					"takeFocus": true,
				}, nil)

				return target, nil
			}
		}

		return nil, nil
	case "cfmleditor.goToMatchingTag":
		if len(params.Arguments) < 2 {
			return nil, fmt.Errorf("cfmleditor.goToMatchingTag requires [documentURI, line, char]")
		}

		docURI, _ := argString(params.Arguments, 0)
		if docURI == "" {
			return nil, nil
		}

		content, ok := s.getDocument(uri.URI(docURI))
		if !ok {
			return nil, nil
		}

		var line, char int

		if len(params.Arguments) >= 3 {
			if v, ok := argFloat(params.Arguments, 1); ok {
				line = int(v)
			}

			if v, ok := argFloat(params.Arguments, 2); ok {
				char = int(v)
			}
		}

		pos := parser.FindMatchingTag(content, line, char)
		if pos == nil {
			return nil, nil
		}

		return pos, nil
	case "cfmleditor.copyPackage":
		if len(params.Arguments) == 0 {
			return nil, fmt.Errorf("cfmleditor.copyPackage requires a document URI argument")
		}

		docURI, _ := argString(params.Arguments, 0)
		if docURI == "" {
			return nil, nil
		}

		filePath := strings.TrimPrefix(docURI, "file://")
		dotPath := s.fileToPackage(filePath)

		return dotPath, nil
	case "cfmleditor.findRefs":
		if len(params.Arguments) == 0 {
			return nil, fmt.Errorf("cfmleditor.findRefs requires a function name argument")
		}

		funcName, _ := argString(params.Arguments, 0)
		if funcName == "" {
			return nil, nil
		}

		sourceURI := ""
		if len(params.Arguments) > 1 {
			sourceURI, _ = argString(params.Arguments, 1)
		}

		s.log.Debug("findRefs: searching", cflog.String("funcName", funcName), cflog.Strings("roots", s.WorkspaceFolders))
		r := s.getResolver()
		sourceFile := uri.URI(sourceURI).Path()
		findOpts := refs.Options{
			FuncName:          funcName,
			Resolvers:         s.cfResolvers(),
			PropertyResolvers: s.cfPropertyResolvers(),
			VerifyCall: func(component, fn, fileDir string) bool {
				return r.HasFunction(component, fn, fileDir)
			},
			VerifyTarget: func(component, fileDir, sourceFile string) bool {
				resolved := r.ComponentPath(component, fileDir)

				return cfpath.SamePath(resolved, sourceFile)
			},
			Reason: func(call parser.CallSite, pr *parser.ParseResult, fileDir string) string {
				return r.CanResolveCall(call, pr, fileDir)
			},
			SourceFile: sourceFile,
		}
		entries := refs.Trace(s.FS, s.WorkspaceFolders, findOpts)
		result := refs.FormatResult(entries, funcName, sourceURI, s.WorkspaceFolders)

		s.log.Debug("findRefs: complete", cflog.String("funcName", funcName), cflog.Int("results", len(entries)))

		output := result.Summary + "\n\n```mermaid\n" + result.Graph.Mermaid() + "\n```"

		outDir := filepath.Dir(sourceFile)
		if outDir == "" || outDir == "." {
			outDir = os.TempDir()
		}

		outFile := filepath.Join(outDir, "refs-"+funcName+".md")
		if err := os.WriteFile(outFile, []byte(output), 0o644); err != nil {
			s.log.Error("failed to write file", cflog.String("path", outFile), cflog.Err(err))
		}

		dotFile := filepath.Join(outDir, "refs-"+funcName+".dot")
		if err := os.WriteFile(dotFile, []byte(result.Graph.DOT()), 0o644); err != nil {
			s.log.Error("failed to write file", cflog.String("path", dotFile), cflog.Err(err))
		}

		s.notify(ctx, protocol.MethodWindowShowMessage, &protocol.ShowMessageParams{
			Type:    protocol.MessageTypeInfo,
			Message: "Wrote " + outFile,
		})

		return result.Summary, nil
	case "cfmleditor.exportDeps":
		if len(params.Arguments) == 0 {
			return nil, fmt.Errorf("cfmleditor.exportDeps requires a document URI")
		}

		docURI, _ := argString(params.Arguments, 0)
		if docURI == "" {
			return nil, nil
		}

		funcName := ""
		if len(params.Arguments) > 1 {
			funcName, _ = argString(params.Arguments, 1)
		}

		fileURI := uri.URI(docURI)

		var depsCalls []parser.CallSite

		s.mu.RLock()
		pr := s.parseResults[fileURI]
		s.mu.RUnlock()

		if pr != nil {
			if funcName != "" {
				// Function-level: FuncCalls for the specific function
				for _, sc := range pr.Scopes {
					for _, f := range pr.Funcs {
						if strings.EqualFold(f.Name, funcName) && int(f.Line) == sc.Start {
							depsCalls = pr.FuncCalls(sc.Start, sc.End)

							break
						}
					}

					if len(depsCalls) > 0 {
						break
					}
				}
			} else {
				// File-level: FuncCalls for all functions
				for _, sc := range pr.Scopes {
					depsCalls = append(depsCalls, pr.FuncCalls(sc.Start, sc.End)...)
				}
			}
		}

		var depsRefs []parser.ComponentRef

		if len(depsCalls) == 0 {
			// Fallback to component refs from index
			ptrs := s.index.RefsForFile(fileURI)
			for _, p := range ptrs {
				depsRefs = append(depsRefs, *p)
			}
		}

		result := deps.Build(deps.Options{
			DocURI:   docURI,
			FuncName: funcName,
			Calls:    depsCalls,
			Refs:     depsRefs,
			Index:    s.index,
			Resolver: s.getResolver(),
			MaxDepth: 10,
		})

		filePath := strings.TrimPrefix(docURI, "file://")

		suffix := funcName
		if suffix == "" {
			suffix = strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
		}

		mermaid := result.Graph.Mermaid()

		outFile := filepath.Join(filepath.Dir(filePath), "deps-"+suffix+".md")
		if err := os.WriteFile(outFile, []byte("```mermaid\n"+mermaid+"\n```\n"), 0o644); err != nil {
			s.log.Error("failed to write file", cflog.String("path", outFile), cflog.Err(err))
		}

		dotFile := filepath.Join(filepath.Dir(filePath), "deps-"+suffix+".dot")
		if err := os.WriteFile(dotFile, []byte(result.Graph.DOT()), 0o644); err != nil {
			s.log.Error("failed to write file", cflog.String("path", dotFile), cflog.Err(err))
		}

		s.notify(ctx, protocol.MethodWindowShowMessage, &protocol.ShowMessageParams{
			Type:    protocol.MessageTypeInfo,
			Message: "Wrote " + outFile,
		})

		return mermaid, nil
	case "cfmleditor.scanWorkspace":
		// Same reasoning as runDiagnostics: this goroutine outlives the
		// handler, so the request ctx (pooled/reset on return) is unsafe here.
		s.safeGo("scanWorkspace", func() { s.scanWorkspace(context.Background()) })

		return nil, nil
	default:
		return nil, fmt.Errorf("unknown command: %s", params.Command)
	}
}

func (s *Server) safeGo(label string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				s.log.Error("goroutine panic", cflog.String("label", label), cflog.Any("panic", r))
			}
		}()

		fn()
	}()
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
