// Package server implements the CFML Language Server Protocol handler.
package server

import (
	"context"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
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
	ServicePropertyResolvers map[string]string         // "@serviceproperty" annotation kind → dot-path template
	ComponentResolvers       []config.Resolver         // custom method-to-component resolvers
	PropertyResolvers        []config.PropResolver     // custom property-to-component resolvers
	resolverMu               sync.Mutex                // guards resolver, cachedResolvers, cachedResolverSet
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
	reindexTimers            map[uri.URI]*time.Timer         // deferred-reindex timers armed by a rapid-change burst
	docLocks                 map[uri.URI]*sync.Mutex         // serialises access to each document's ParseResult
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
		reindexTimers:     make(map[uri.URI]*time.Timer),
		docLocks:          make(map[uri.URI]*sync.Mutex),
		parseResults:      make(map[uri.URI]*parser.ParseResult),
		changeCount:       make(map[uri.URI]int),
		changeWindowStart: make(map[uri.URI]time.Time),
	}
}

func (s *Server) capabilities() protocol.ServerCapabilities {
	openClose := true
	change := protocol.TextDocumentSyncKindIncremental
	resolveProvider := true
	supported := true

	return protocol.ServerCapabilities{
		TextDocumentSync: &protocol.TextDocumentSyncOptions{
			OpenClose: &openClose,
			Change:    &change,
			Save:      &protocol.SaveOptions{},
		},
		CompletionProvider: &protocol.CompletionOptions{
			TriggerCharacters: []string{"<", "/", ".", ">"},
		},
		DocumentFormattingProvider: protocol.Boolean(true),
		DocumentOnTypeFormattingProvider: protocol.DocumentOnTypeFormattingOptions{
			FirstTriggerCharacter: ">",
		},
		DefinitionProvider: protocol.Boolean(true),
		SignatureHelpProvider: &protocol.SignatureHelpOptions{
			TriggerCharacters: []string{"(", ","},
		},
		DocumentSymbolProvider:  protocol.Boolean(true),
		WorkspaceSymbolProvider: protocol.Boolean(true),
		HoverProvider:           protocol.Boolean(true),
		DocumentLinkProvider:    &protocol.DocumentLinkOptions{ResolveProvider: &resolveProvider},
		CodeActionProvider:      protocol.Boolean(true),
		ExecuteCommandProvider: protocol.ExecuteCommandOptions{
			Commands: []string{"cfmleditor.reindex", "cfmleditor.format", "cfmleditor.showComponentPath", "cfmleditor.restartDaemon", "cfmleditor.showResolvers", "cfmleditor.showFileIndex", "cfmleditor.showConnections", "cfmleditor.openActiveApplicationFile", "cfmleditor.goToMatchingTag", "cfmleditor.copyPackage", "cfmleditor.findRefs", "cfmleditor.exportDeps", "cfmleditor.scanWorkspace"},
		},
		Workspace: &protocol.WorkspaceOptions{
			WorkspaceFolders: &protocol.WorkspaceFoldersServerCapabilities{
				Supported:           &supported,
				ChangeNotifications: protocol.Boolean(true),
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

// ensureFuncRefsIndexed lazily indexes resolver refs for the function enclosing the given line.
// Called when LookupComponentRefInFile returns nil and the cursor is inside a function.
//
// Callers hold the document's lock (see lockDoc): pr.FuncRefs memoises in place.
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
				s.index.SetFuncRefs(docURI, strconv.Itoa(sc.Start)+":"+strconv.Itoa(sc.End), refs)
			}

			return
		}
	}
}

// lockDoc serialises every access to one document's *parser.ParseResult, and
// returns the function that releases it.
//
// A ParseResult is not a value that gets swapped in and out — it is mutated in
// place by ApplyEdit/ApplyFullReplace, and even its read accessors (ThisVars,
// FuncRefs, FuncCalls) mutate, because each memoises lazily on first use. One
// pointer therefore cannot be shared across goroutines at all, and the server
// has several: the LSP read goroutine, the rapid-change reindex timer, the
// completion-cache debounce timer, and the background cache rebuild. A burst of
// typing arms the reindex timer, and 200ms later it calls ApplyFullReplace on
// exactly the ParseResult the next keystroke is reading — `go test -race`
// reports the pair as reparseShallow against computeScopedVars.
//
// The lock is per document rather than global so that work on one file never
// waits on another. Within a connection the LSP requests are already serialised
// (the handler never calls jsonrpc2.Async), so in practice this contends only
// between a handler and one of the timers — which is the point.
//
// Lock ordering: acquire a doc lock without holding s.mu; s.mu may be taken
// while holding one. Doc locks are not reentrant, so take one only at the entry
// points — an LSP handler, a timer body, a background goroutine — never in a
// helper that those call.
func (s *Server) lockDoc(docURI uri.URI) func() {
	s.mu.Lock()

	mu, ok := s.docLocks[docURI]
	if !ok {
		mu = &sync.Mutex{}
		s.docLocks[docURI] = mu
	}

	s.mu.Unlock()
	mu.Lock()

	return mu.Unlock
}

// getResolver returns the shared resolver, building it on first use.
//
// The resolver and the two cached resolver forms below are rebuilt lazily and
// discarded wholesale by invalidateResolver when config changes, so they are
// written from whichever goroutine happens to ask first. That is not only the
// LSP read goroutine: indexWorkspace reaches one through isOrmPath, and
// didSave's cache rebuild reaches one through parseContent, both on their own
// goroutines. Unguarded, two of them build competing resolvers, and a rebuild
// racing invalidateResolver can hand back the nil it just stored. safeGo's
// recover() turns the resulting nil dereference into a logged panic rather
// than a crash, which means the visible symptom is a workspace that quietly
// stops indexing partway through.
//
// resolverMu covers exactly these three fields and is never held across a
// parse or an index call, so it cannot participate in a lock cycle with s.mu.
// Call invalidateResolver() when config changes.
func (s *Server) getResolver() *resolve.Resolver {
	s.resolverMu.Lock()
	defer s.resolverMu.Unlock()

	if s.resolver == nil {
		s.resolver = &resolve.Resolver{
			FS:                 s.FS,
			WorkspaceFolders:   s.WorkspaceFolders,
			Mappings:           s.Mappings,
			ExpressionMappings: s.ExpressionMappings,
			Index:              s.index,
			Resolvers:          s.buildResolvers(),
		}
	}

	return s.resolver
}

// invalidateResolver drops the resolver and both cached resolver forms, so the
// next caller rebuilds all three from the current configuration.
//
// The caches have to go too, not just the resolver: applyConfig invalidates and
// then appends the config file's componentResolvers to s.ComponentResolvers. If
// cachedResolvers survived that, the appended entries would never be converted
// and every resolver a .cfmleditor.json contributed would be silently ignored
// for the rest of the session.
func (s *Server) invalidateResolver() {
	s.resolverMu.Lock()
	s.resolver = nil
	s.cachedResolvers = nil
	s.cachedResolverSet = nil
	s.resolverMu.Unlock()
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
	s.resolverMu.Lock()
	defer s.resolverMu.Unlock()

	return s.buildResolvers()
}

func (s *Server) cfResolverSet() *parser.ResolverSet {
	s.resolverMu.Lock()
	defer s.resolverMu.Unlock()

	s.buildResolvers() // ensure built

	return s.cachedResolverSet
}

// buildResolvers converts the configured resolvers once and memoises both the
// slice and the pre-grouped set. Callers must hold resolverMu.
//
// cachedResolvers is published last so that it alone signals "both are built".
// resolverMu already makes the order unobservable; the point is that the
// "already cached?" check keys off the field written second, so the invariant
// survives if anyone later reaches for these without the lock. Publishing the
// slice first was how a concurrent cfResolverSet came to see the slice present,
// skip the build, and return a still-nil set.
func (s *Server) buildResolvers() []parser.Resolver {
	if len(s.ComponentResolvers) == 0 {
		return nil
	}

	if s.cachedResolvers != nil {
		return s.cachedResolvers
	}

	r := make([]parser.Resolver, len(s.ComponentResolvers))
	for i, cr := range s.ComponentResolvers {
		r[i] = parser.Resolver{Match: cr.Match, Resolve: cr.Resolve, Prefix: cr.Prefix, NoFollow: cr.NoFollow, Anchored: cr.Anchored}
	}

	s.cachedResolverSet = parser.BuildResolverSet(r)
	s.cachedResolvers = r

	return r
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

	baseDir := filepath.Dir(strings.TrimPrefix(string(fileURI), "file://"))
	resolver := s.getResolver()

	return parser.ParseWithOptions(fileURI, content, parser.ParseOptions{
		Logger:                   s.log,
		Resolvers:                s.cfResolvers(),
		PropertyResolvers:        s.cfPropertyResolvers(),
		BeanLookup:               s.index.LookupBean,
		BuiltinReturnLookup:      docs.LookupBuiltinReturnComponent,
		FuncLookup:               funcLookup(resolver, baseDir),
		ExpressionMappings:       s.ExpressionMappings,
		ServicePropertyResolvers: s.ServicePropertyResolvers,
		ExtractLinks:             true,
		ExtractCalls:             true,
	})
}

// funcLookup resolves a method's declared return-type component across files
// (e.g. a Java stub's getInstance() modeling its own return type), so the parser
// can prefer a verified cross-file answer over a componentResolver's guess on
// the call-site text. Shared shape with cmd/cfmleditor-lsp/unresolved.go.
func funcLookup(resolver *resolve.Resolver, baseDir string) func(component, funcName string) string {
	return func(component, funcName string) string {
		fd := resolver.ResolveFunc(component, funcName, baseDir)
		if fd == nil {
			return ""
		}

		if fd.ReturnComponent != "" {
			return fd.ReturnComponent
		}

		if fd.ReturnType != "" && strings.Contains(fd.ReturnType, ".") {
			return fd.ReturnType
		}

		return ""
	}
}

// parseContentForIndex parses CFC content for indexing (signatures only, no resolvers/links).
func (s *Server) parseContentForIndex(fileURI uri.URI, content string) *parser.ParseResult {
	return parser.ParseWithOptions(fileURI, content, parser.ParseOptions{Shallow: true})
}
