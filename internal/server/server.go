// Package server implements the CFML Language Server Protocol handler.
package server

import (
	"context"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/cfmleditor/cfmleditor-lsp/internal/cache"
	"github.com/cfmleditor/cfmleditor-lsp/internal/cflint"
	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
	"github.com/cfmleditor/cfmleditor-lsp/internal/docs"
	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"github.com/cfmleditor/cfmleditor-lsp/internal/resolve"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// Server implements the CFML Language Server Protocol handler.
type Server struct {
	conn        jsonrpc2.Conn
	log         cflog.Logger
	initialized bool
	Version     string
	FS          vfs.FS // filesystem abstraction for portability

	mu                       sync.RWMutex
	documents                map[uri.URI]string
	workspaceRoots           []string
	WorkspaceFolders         []string                  // project folders from config
	IndexGlobs               []string                  // optional glob filters (absolute paths)
	Mappings                 map[string]string         // component path mappings (key -> abs path)
	ExpressionMappings       map[string]string         // runtime expression → static value substitutions
	ComponentResolvers       []config.Resolver         // custom method-to-component resolvers
	PropertyResolvers        []config.PropResolver     // custom property-to-component resolvers
	cachedResolvers          []parser.Resolver         // cached parser.Resolver slice
	cachedResolverSet        *parser.ResolverSet       // pre-grouped for fast matching
	BeanPaths                map[string]string         // namespace → abs directory path for bean scanning
	Formatting               config.ResolvedFormatting // formatting settings
	Linting                  bool                      // enable cflint diagnostics
	TagSnippets              bool                      // insert snippets for tags
	FunctionSnippets         bool                      // insert snippets for functions
	GlobalFunctionResolution bool                      // resolve unqualified functions via global index
	changeCount              map[uri.URI]int           // rapid change counter per file
	changeWindowStart        map[uri.URI]time.Time     // start of current rapid-change window
	resolveCache             map[string]string         // cached component path resolutions
	beansLoaded              bool                      // whether bean map has been built
	index                    *index.Index
	resolver                 *resolve.Resolver
	linter                   *cflint.Runner
	lintCancels              map[uri.URI]context.CancelFunc
	compCache                *cache.Cache
	funcRanges               map[uri.URI][]cache.FuncRange   // cached function line ranges per file
	cacheTimers              map[uri.URI]*time.Timer         // debounce timers for completion cache rebuild
	parseResults             map[uri.URI]*parser.ParseResult // cached parse results per file
	lastResolveKey           string                          // dedup key for hover/definition (uri:line:char)
	lastResolveDef           *parser.FunctionDef             // cached result
}

// NewServer creates a new LSP server. If sharedIndex is non-nil it is used
// instead of creating a private index, allowing multiple sessions to share one.
func NewServer(conn jsonrpc2.Conn, log cflog.Logger, sharedIndex ...*index.Index) *Server {
	idx := index.New()
	if len(sharedIndex) > 0 && sharedIndex[0] != nil {
		idx = sharedIndex[0]
	}

	return &Server{
		conn:              conn,
		log:               log,
		FS:                vfs.OS{},
		documents:         make(map[uri.URI]string),
		index:             idx,
		lintCancels:       make(map[uri.URI]context.CancelFunc),
		compCache:         cache.New(),
		funcRanges:        make(map[uri.URI][]cache.FuncRange),
		cacheTimers:       make(map[uri.URI]*time.Timer),
		parseResults:      make(map[uri.URI]*parser.ParseResult),
		changeCount:       make(map[uri.URI]int),
		changeWindowStart: make(map[uri.URI]time.Time),
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
		DocumentFormattingProvider: true,
		DocumentOnTypeFormattingProvider: &protocol.DocumentOnTypeFormattingOptions{
			FirstTriggerCharacter: ">",
		},
		DefinitionProvider: true,
		SignatureHelpProvider: &protocol.SignatureHelpOptions{
			TriggerCharacters: []string{"(", ","},
		},
		DocumentSymbolProvider:  true,
		WorkspaceSymbolProvider: true,
		HoverProvider:           true,
		DocumentLinkProvider:    &protocol.DocumentLinkOptions{ResolveProvider: true},
		CodeActionProvider:      true,
		ExecuteCommandProvider: &protocol.ExecuteCommandOptions{
			Commands: []string{"cfmleditor.reindex", "cfmleditor.format", "cfmleditor.showComponentPath", "cfmleditor.restartDaemon", "cfmleditor.showResolvers", "cfmleditor.showFileIndex", "cfmleditor.showConnections", "cfmleditor.openActiveApplicationFile", "cfmleditor.goToMatchingTag", "cfmleditor.copyPackage", "cfmleditor.findRefs", "cfmleditor.exportDeps", "cfmleditor.scanWorkspace"},
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
	if !s.Linting {
		return
	}

	runner, err := cflint.NewRunner()
	if err != nil {
		s.log.Warn("cflint unavailable", cflog.Err(err))

		return
	}

	s.mu.Lock()
	s.linter = runner
	s.mu.Unlock()
	s.log.Info("cflint ready")
}

// getResolver returns the shared resolver, creating it if needed.
// ensureFuncRefsIndexed lazily indexes resolver refs for the function enclosing the given line.
// Called when LookupComponentRefInFile returns nil and the cursor is inside a function.
func (s *Server) ensureFuncRefsIndexed(docURI uri.URI, line int) {
	s.mu.RLock()
	pr := s.parseResults[docURI]
	s.mu.RUnlock()

	if pr == nil {
		return
	}

	for _, sc := range pr.Scopes {
		if line > sc.Start && line < sc.End {
			refs, _ := pr.FuncRefs(sc.Start, sc.End)
			if len(refs) > 0 {
				s.index.AddRefs(refs)
			}

			return
		}
	}
}

// Call invalidateResolver() when config changes.
func (s *Server) getResolver() *resolve.Resolver {
	if s.resolver == nil {
		s.resolver = &resolve.Resolver{
			FS:                 s.FS,
			WorkspaceFolders:   s.WorkspaceFolders,
			Mappings:           s.Mappings,
			ExpressionMappings: s.ExpressionMappings,
			Index:              s.index,
			Resolvers:          s.cfResolvers(),
		}
	}

	return s.resolver
}

func (s *Server) invalidateResolver() {
	s.resolver = nil
}

// ensureBeansLoaded lazily builds the bean map on first access.
func (s *Server) ensureBeansLoaded() {
	s.mu.RLock()

	if s.beansLoaded {
		s.mu.RUnlock()

		return
	}

	s.mu.RUnlock()

	s.mu.Lock()
	if s.beansLoaded {
		s.mu.Unlock()

		return
	}

	allBeanPaths := make(map[string]string)

	for _, root := range s.WorkspaceFolders {
		appDir := s.getResolver().FindApplicationRoot(root)
		if appDir != "" {
			for ns, dir := range cfpath.LoadAppBeanPaths(appDir) {
				if _, exists := allBeanPaths[ns]; !exists {
					allBeanPaths[ns] = dir
				}
			}
		}
	}

	maps.Copy(allBeanPaths, s.BeanPaths)

	if len(allBeanPaths) > 0 {
		beans := buildBeanMap(allBeanPaths, s.FS)
		s.index.SetBeans(beans)
		s.log.Info("bean map built (lazy)", cflog.Int("beans", len(beans)))
	}

	s.beansLoaded = true
	s.mu.Unlock()
}

func (s *Server) notify(ctx context.Context, method string, params any) {
	if s.conn != nil {
		_ = s.conn.Notify(ctx, method, params)
	}
}

func (s *Server) call(ctx context.Context, method string, params, result any) {
	if s.conn != nil {
		_, _ = s.conn.Call(ctx, method, params, result)
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
	s.lastResolveKey = ""
	s.lastResolveDef = nil
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
		return cfpath.MatchesGlob(filePath, s.IndexGlobs)
	}
	// Otherwise, any .cfc under a workspace folder is included
	for _, f := range s.WorkspaceFolders {
		if strings.HasPrefix(filePath, f+"/") {
			return true
		}
	}

	return false
}

func (s *Server) invalidateResolveCache() {
	s.mu.Lock()
	s.resolveCache = nil
	s.mu.Unlock()
	s.invalidateResolver()
}

func (s *Server) cfResolvers() []parser.Resolver {
	if len(s.ComponentResolvers) == 0 {
		return nil
	}

	if s.cachedResolvers != nil {
		return s.cachedResolvers
	}

	r := make([]parser.Resolver, len(s.ComponentResolvers))
	for i, cr := range s.ComponentResolvers {
		r[i] = parser.Resolver{Match: cr.Match, Resolve: cr.Resolve, Prefix: cr.Prefix}
	}

	s.cachedResolvers = r
	s.cachedResolverSet = parser.BuildResolverSet(r)

	return r
}

func (s *Server) cfResolverSet() *parser.ResolverSet {
	s.cfResolvers() // ensure built

	return s.cachedResolverSet
}

func (s *Server) cfPropertyResolvers() []parser.PropertyResolver {
	if len(s.PropertyResolvers) == 0 {
		return nil
	}

	r := make([]parser.PropertyResolver, len(s.PropertyResolvers))
	for i, pr := range s.PropertyResolvers {
		r[i] = parser.PropertyResolver{Match: pr.Match, Resolve: pr.Resolve, Attribute: pr.Attribute}
	}

	return r
}

// parseContent parses CFC content with all configured resolvers and link extraction.
func (s *Server) parseContent(fileURI uri.URI, content string) *parser.ParseResult {
	s.ensureBeansLoaded()

	return parser.ParseWithOptions(fileURI, content, parser.ParseOptions{
		Logger:              s.log,
		Resolvers:           s.cfResolvers(),
		PropertyResolvers:   s.cfPropertyResolvers(),
		BeanLookup:          s.index.LookupBean,
		BuiltinReturnLookup: docs.LookupBuiltinReturnComponent,
		ExpressionMappings:  s.ExpressionMappings,
		ExtractLinks:        true,
		ExtractCalls:        true,
	})
}

// parseContentForIndex parses CFC content for indexing (signatures only, no resolvers/links).
func (s *Server) parseContentForIndex(fileURI uri.URI, content string) *parser.ParseResult {
	return parser.ParseWithOptions(fileURI, content, parser.ParseOptions{Shallow: true})
}
