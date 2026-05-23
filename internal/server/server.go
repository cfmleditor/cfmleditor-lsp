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
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
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
	FS          vfs.FS // filesystem abstraction for portability

	mu               sync.RWMutex
	documents        map[uri.URI]string
	workspaceRoots   []string
	WorkspaceFolders []string // project folders from config
	IndexGlobs       []string // optional glob filters (absolute paths)
	Mappings           map[string]string      // component path mappings (key -> abs path)
	ComponentResolvers []ComponentResolver    // custom method-to-component resolvers
	PropertyResolvers  []PropertyResolver     // custom property-to-component resolvers
	BeanPaths          map[string]string    // namespace → abs directory path for bean scanning
	Formatting         FormattingConfig       // formatting settings
	changeCount        map[uri.URI]int        // rapid change counter per file
	changeWindowStart  map[uri.URI]time.Time  // start of current rapid-change window
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
		conn:          conn,
		logger:        logger,
		FS:            vfs.OS{},
		documents:     make(map[uri.URI]string),
		index:         idx,
		lintCancels:   make(map[uri.URI]context.CancelFunc),
		compCache:     cache.New(),
		funcRanges:    make(map[uri.URI][]cache.FuncRange),
		cacheTimers:   make(map[uri.URI]*time.Timer),
		parseResults:      make(map[uri.URI]*cfparser.ParseResult),
		changeCount:       make(map[uri.URI]int),
		changeWindowStart: make(map[uri.URI]time.Time),
	}
}

// isCFMLFile returns true if the path/URI refers to a CFML file (.cfc, .cfm, .cfml, .cfs).
func isCFMLFile(path string) bool {
	if len(path) < 4 {
		return false
	}
	// Check last char first as a fast reject
	end := path[len(path)-1]
	switch end | 0x20 {
	case 'c': // .cfc
		return len(path) > 4 && path[len(path)-4] == '.' &&
			(path[len(path)-3]|0x20) == 'c' && (path[len(path)-2]|0x20) == 'f'
	case 'm': // .cfm
		return path[len(path)-4] == '.' && (path[len(path)-3]|0x20) == 'c' && (path[len(path)-2]|0x20) == 'f'
	case 'l': // .cfml
		return len(path) > 5 && path[len(path)-5] == '.' && (path[len(path)-4]|0x20) == 'c' &&
			(path[len(path)-3]|0x20) == 'f' && (path[len(path)-2]|0x20) == 'm'
	case 's': // .cfs
		return path[len(path)-4] == '.' && (path[len(path)-3]|0x20) == 'c' && (path[len(path)-2]|0x20) == 'f'
	}
	return false
}

// isCFCFile returns true if the path/URI refers to a CFC file.
func isCFCFile(path string) bool {
	return len(path) > 4 && path[len(path)-4] == '.' &&
		(path[len(path)-3]|0x20) == 'c' && (path[len(path)-2]|0x20) == 'f' && (path[len(path)-1]|0x20) == 'c'
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
		SignatureHelpProvider: &protocol.SignatureHelpOptions{
			TriggerCharacters: []string{"(", ","},
		},
		DocumentSymbolProvider:  true,
		WorkspaceSymbolProvider: true,
		HoverProvider:           true,
		DocumentLinkProvider:    &protocol.DocumentLinkOptions{ResolveProvider: true},
		ExecuteCommandProvider: &protocol.ExecuteCommandOptions{
			Commands: []string{"cfmleditor.reindex", "cfmleditor.format", "cfmleditor.showComponentPath", "cfmleditor.restartDaemon", "cfmleditor.showResolvers", "cfmleditor.showFileIndex", "cfmleditor.showConnections", "cfmleditor.openActiveApplicationFile", "cfmleditor.goToMatchingTag", "cfmleditor.copyPackage"},
		},
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

func (s *Server) cfPropertyResolvers() []cfparser.PropertyResolver {
	if len(s.PropertyResolvers) == 0 {
		return nil
	}
	r := make([]cfparser.PropertyResolver, len(s.PropertyResolvers))
	for i, pr := range s.PropertyResolvers {
		r[i] = cfparser.PropertyResolver{Match: pr.Match, Resolve: pr.Resolve, Attribute: pr.Attribute}
	}
	return r
}

// parseContent parses CFC content with all configured resolvers and link extraction.
func (s *Server) parseContent(fileURI uri.URI, content string) *cfparser.ParseResult {
	return cfparser.ParseWithOptions(fileURI, content, cfparser.ParseOptions{
		Resolvers:         s.cfResolvers(),
		PropertyResolvers: s.cfPropertyResolvers(),
		BeanLookup:        s.index.LookupBean,
		ExtractLinks:      true,
	})
}

// parseContentForIndex parses CFC content for indexing (no link extraction).
func (s *Server) parseContentForIndex(fileURI uri.URI, content string) *cfparser.ParseResult {
	return cfparser.ParseWithOptions(fileURI, content, cfparser.ParseOptions{
		Resolvers:         s.cfResolvers(),
		PropertyResolvers: s.cfPropertyResolvers(),
		BeanLookup:        s.index.LookupBean,
	})
}

// FormattingConfig holds formatting settings from .cfmleditor.json.
type FormattingConfig struct {
	Enabled               bool
	Debug                 bool
	SelfCloseTags         bool
	WhitespaceOnly         bool
	QueryFormat            bool
	LowercaseTags          bool
	LowercaseAttributes    bool
	DoubleQuoteAttributes  bool
	QueryUppercaseKeywords bool
	ScopeCase              string
	CommaPosition          string
	QueryCommaPosition     string
	LineWidth              int
	AttrBreakThreshold     int
	IndentWidth            int
}

// ComponentResolver maps a method call pattern to a component path.
// Match is a pattern like getService("$1") and Resolve is a path template like packages/$1/service.cfc.
// Prefix is a fast-check string that must appear in a line before attempting the full match.
type ComponentResolver struct {
	Match   string
	Resolve string
	Prefix  string
}

// PropertyResolver resolves a property declaration to a component path based on
// an attribute value (e.g. inject="model.UserDAO" → models.UserDAO).
type PropertyResolver struct {
	Match     string // pattern to match against the attribute value, $1 is capture placeholder
	Resolve   string // component dot-path template, $1 replaced with captured value
	Attribute string // property attribute to inspect (e.g. "inject")
}

// resolveComponentFromCall matches a call expression against configured resolvers.
// ensureIndexed ensures a CFC file is indexed, loading from disk if needed.
// Returns the functions defined in the file.
func (s *Server) ensureIndexed(cfcPath string) []*cfparser.FunctionDef {
	cfcURI := uri.URI("file://" + cfcPath)
	defs := s.index.FunctionsForFile(cfcURI)
	if len(defs) == 0 {
		content, ok := s.getDocument(cfcURI)
		if !ok {
			data, err := s.FS.ReadFile(cfcPath)
			if err != nil {
				return nil
			}
			content = string(data)
		}
		pr := s.parseContent(cfcURI, content)
		s.index.IndexFileFromResult(cfcURI, pr.Funcs, pr.Refs)
		s.index.SetThisVars(cfcURI, pr.ThisVars())
		defs = s.index.FunctionsForFile(cfcURI)
	}
	return defs
}

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
