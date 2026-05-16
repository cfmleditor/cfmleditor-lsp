// Package server implements the CFML Language Server Protocol handler.
package server

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/cfmleditor/cfmleditor-lsp/internal/cache"
	"github.com/cfmleditor/cfmleditor-lsp/internal/cfparser"
	"github.com/cfmleditor/cfmleditor-lsp/internal/cflint"
	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
	"go.uber.org/zap"
)

// Server implements the CFML Language Server Protocol handler.
type Server struct {
	conn        jsonrpc2.Conn
	logger      *zap.Logger
	initialized bool
	Version     string

	mu               sync.RWMutex
	documents        map[uri.URI]string
	workspaceRoots   []string
	WorkspaceFolders []string // project folders from config
	IndexGlobs       []string // optional glob filters (absolute paths)
	Mappings           map[string]string      // component path mappings (key -> abs path)
	ComponentResolvers []ComponentResolver    // custom method-to-component resolvers
	resolveCache       map[string]string      // cached component path resolutions
	index            *index.Index
	linter           *cflint.Runner
	lintCancels      map[uri.URI]context.CancelFunc
	compCache        *cache.Cache
	funcRanges       map[uri.URI][]cache.FuncRange // cached function line ranges per file
	cacheTimers      map[uri.URI]*time.Timer           // debounce timers for completion cache rebuild
	parseResults     map[uri.URI]*cfparser.ParseResult  // cached parse results per file
}

// NewServer creates a new LSP server. If sharedIndex is non-nil it is used
// instead of creating a private index, allowing multiple sessions to share one.
func NewServer(conn jsonrpc2.Conn, logger *zap.Logger, sharedIndex ...*index.Index) *Server {
	idx := index.New()
	if len(sharedIndex) > 0 && sharedIndex[0] != nil {
		idx = sharedIndex[0]
	}
	return &Server{
		conn:         conn,
		logger:       logger,
		documents:    make(map[uri.URI]string),
		index:        idx,
		lintCancels:  make(map[uri.URI]context.CancelFunc),
		compCache:    cache.New(),
		funcRanges:   make(map[uri.URI][]cache.FuncRange),
		cacheTimers:  make(map[uri.URI]*time.Timer),
		parseResults: make(map[uri.URI]*cfparser.ParseResult),
	}
}

func (s *Server) capabilities() protocol.ServerCapabilities {
	return protocol.ServerCapabilities{
		TextDocumentSync: protocol.TextDocumentSyncOptions{
			OpenClose: true,
			Change:    protocol.TextDocumentSyncKindIncremental,
			Save:      &protocol.SaveOptions{},
		},
		CompletionProvider: &protocol.CompletionOptions{
			TriggerCharacters: []string{"<", "/", ".", ">"},
		},
		DocumentFormattingProvider:      true,
		DocumentOnTypeFormattingProvider: &protocol.DocumentOnTypeFormattingOptions{
			FirstTriggerCharacter: ">",
		},
		DefinitionProvider:         true,
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

func (s *Server) initLinter() {
	runner, err := cflint.NewRunner()
	if err != nil {
		s.logger.Warn("cflint unavailable", zap.Error(err))
		return
	}
	s.mu.Lock()
	s.linter = runner
	s.mu.Unlock()
	s.logger.Info("cflint ready")
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
	return slices.Contains(s.WorkspaceFolders, root)
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

func (s *Server) invalidateResolveCache() {
	s.mu.Lock()
	s.resolveCache = nil
	s.mu.Unlock()
}

func (s *Server) cfResolvers() []cfparser.Resolver {
	if len(s.ComponentResolvers) == 0 {
		return nil
	}
	r := make([]cfparser.Resolver, len(s.ComponentResolvers))
	for i, cr := range s.ComponentResolvers {
		r[i] = cfparser.Resolver{Match: cr.Match, Resolve: cr.Resolve, Prefix: cr.Prefix}
	}
	return r
}

// ComponentResolver maps a method call pattern to a component path.
// Match is a pattern like getService("$1") and Resolve is a path template like packages/$1/service.cfc.
// Prefix is a fast-check string that must appear in a line before attempting the full match.
type ComponentResolver struct {
	Match   string
	Resolve string
	Prefix  string
}

// resolveComponentFromCall matches a call expression against configured resolvers.
func resolveComponentFromCall(expr string, resolvers []ComponentResolver) string {
	if len(resolvers) == 0 {
		return ""
	}
	cfr := make([]cfparser.Resolver, len(resolvers))
	for i, r := range resolvers {
		cfr[i] = cfparser.Resolver{Match: r.Match, Resolve: r.Resolve, Prefix: r.Prefix}
	}
	return cfparser.ResolveFromCall(expr, cfr)
}
