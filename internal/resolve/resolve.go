// Package resolve provides component and function resolution logic.
package resolve

import (
	"encoding/json"
	"fmt"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/cfmleditor/cfmleditor-lsp/internal/docs"
	"github.com/cfmleditor/cfmleditor-lsp/internal/frameworkapi"
	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
	"go.lsp.dev/uri"
)

// Resolver resolves component dot-paths to files and functions.
type Resolver struct {
	Discovery         *DiscoveryDependencies
	configurationOnce sync.Once
	// Context views keep runtime mappings while traversing physical library files.
	callerMappings     map[string]string
	contextViews       map[string]*Resolver
	indexer            *Resolver
	FS                 vfs.FS
	WorkspaceFolders   []string
	Mappings           map[string]string
	BeanPaths          map[string]string // configured factory-managed bean roots
	StartupFiles       []string          // configured startup templates, absolute; see startup.go
	ExpressionMappings map[string]string
	Index              *index.Index
	Resolvers          []parser.Resolver
	// ImplicitExtends names the component a file extends when it names none,
	// from its path: a framework preset's rule that a ColdBox handler is an
	// EventHandler without saying so. Nil for none. See config.ImplicitExtends.
	ImplicitExtends func(path string) string
	// HelperScope reports whether a framework mixes its helper templates into
	// the file at path: a ColdBox handler, view, layout or interceptor. Nil for
	// none. See helpers.go and config.HelperScope.
	HelperScope func(path string) bool
	// Stubs is the API of the frameworks the configuration names, answering
	// a framework component nothing on disk does: calls on it are checked,
	// and completion and hover see its methods. Nil for none. See
	// internal/frameworkapi.
	Stubs          *frameworkapi.Set
	stubFS         vfs.FS
	mu             sync.RWMutex
	appRootCache   map[string]string   // dir → Application.cfc root
	slugCache      map[string]string   // dir → its box.json slug, "" for none
	resolveCache   map[string]string   // component+"\t"+baseDir → file path
	dirCache       *cfpath.DirCache    // directory listings behind those resolutions
	incGraph       *includeGraph       // the index's cfincludes, rebuilt when they change
	exprKeys       []string            // ExpressionMappings' keys in the order they apply
	implicitCache  map[string]string   // path → ImplicitExtends(path)
	helpers        *helperSet          // application helper templates, per set of config files
	wb             *wireboxWorkspace   // what ModuleConfig.cfc and config/WireBox.cfc say about ids, per set of files
	beanPathsCache map[string]string   // merged application/configured bean roots
	fw1Scopes      map[string]fw1Scope // nearest application's source-defined injection scope
	diOnce         sync.Once
	diPolicies     []diPolicy                 // source-backed DI/1 injection contracts
	discoveringDI  bool                       // private policy discovery never re-enters injection lookup
	startupCache   map[string][]startupAssign // app root → its startup templates' shared-scope assignments
	wheelsSources  map[string]wheelsSource    // source-checked method bodies; refreshed when bytes change
	returnCache    returnCache                // ReturnComponentOf answers, for one index generation
}

// returnCache holds ReturnComponentOf's answers. An answer reads the index and
// the declaring file's source, so it is kept only while the index generation
// is the one it was computed under and the declaring file's stamp is
// unchanged. Keys are the definitions themselves: a re-indexed file gets new
// ones, and a cache that outgrows maxReturnCache (definitions from transient
// parses never repeat) starts again rather than holding them.
type returnCache struct {
	gen     uint64
	entries map[*parser.FunctionDef]returnEntry
}

type returnEntry struct {
	component string
	size      int64
	modTime   time.Time
}

const maxReturnCache = 1 << 16

// describeResolver names the resolver at idx for trace output, so a wrong component can be
// traced back to the exact componentResolvers entry that produced it rather than just to
// "a componentResolver".
func (r *Resolver) describeResolver(idx int) string {
	if idx < 0 || idx >= len(r.Resolvers) {
		return "unknown resolver"
	}

	return r.Resolvers[idx].Describe()
}

// ComponentPath resolves a component dot-path to an absolute .cfc file path
// using the standard fallback chain: baseDir → Application.cfc root → workspace folders.
// If component contains pipe characters, each alternative is tried left-to-right.
func (r *Resolver) ComponentPath(component, baseDir string) string {
	// Apply expression mappings (replace runtime expressions with static values).
	// A key may list multiple pipe-delimited alternatives that all map to the same value.
	for _, key := range r.expressionKeys() {
		value := r.ExpressionMappings[key]

		for expr := range strings.SplitSeq(key, "|") {
			if expr != "" && strings.Contains(component, expr) {
				component = strings.ReplaceAll(component, expr, value)
			}
		}
	}

	component = strings.TrimPrefix(component, parser.MockPrefix)

	key := component + "\t" + baseDir

	r.mu.RLock()

	if r.resolveCache != nil {
		if p, ok := r.resolveCache[key]; ok {
			r.mu.RUnlock()

			return p
		}
	}

	r.mu.RUnlock()

	var result string

	if strings.Contains(component, "|") {
		for alt := range strings.SplitSeq(component, "|") {
			if p := r.componentPathUncached(alt, baseDir); p != "" {
				result = p

				break
			}
		}
	} else {
		result = r.componentPathUncached(component, baseDir)
	}

	r.mu.Lock()
	if r.resolveCache == nil {
		r.resolveCache = make(map[string]string)
	}

	r.resolveCache[key] = result
	r.mu.Unlock()

	return result
}

// expressionKeys returns ExpressionMappings' keys in the order they apply
// (parser.ExpressionMappingOrder), worked out once: the resolver is dropped
// wherever its configuration changes.
func (r *Resolver) expressionKeys() []string {
	if len(r.ExpressionMappings) == 0 {
		return nil
	}

	r.mu.RLock()
	keys := r.exprKeys
	r.mu.RUnlock()

	if keys != nil {
		return keys
	}

	keys = parser.ExpressionMappingOrder(r.ExpressionMappings)

	r.mu.Lock()
	r.exprKeys = keys
	r.mu.Unlock()

	return keys
}

// dirs returns the resolver's directory-listing cache, creating it on first
// use. Its lifetime is the resolver's: the server drops the whole Resolver
// wherever it decides a path answer may have gone stale, which is what the
// cache needs and what resolveCache beside it already relies on.
func (r *Resolver) dirs() *cfpath.DirCache {
	r.mu.RLock()
	c := r.dirCache
	r.mu.RUnlock()

	if c != nil {
		return c
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.dirCache == nil {
		r.dirCache = cfpath.NewDirCache()
	}

	return r.dirCache
}

// fs is the resolver's filesystem, with the framework stubs mounted: a
// preset's, and the ones a fully namespaced path reaches without one
// (frameworkapi.Namespaced).
func (r *Resolver) fs() vfs.FS {
	r.mu.RLock()
	f := r.stubFS
	r.mu.RUnlock()

	if f != nil {
		return f
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.stubFS == nil {
		r.stubFS = frameworkapi.Wrap(r.FS)
	}

	return r.stubFS
}

func (r *Resolver) componentPathUncached(component, baseDir string) string {
	// A component already named by its file: what a function returning
	// `this` returns.
	if filepath.IsAbs(component) && strings.HasSuffix(strings.ToLower(component), ".cfc") {
		if info, err := r.fs().Stat(component); err == nil && !info.IsDir() {
			return component
		}
	}

	// A WireBox id with its module: `Name@module`.
	if strings.Contains(component, "@") {
		p, _ := r.wireboxID(component, baseDir)

		return p
	}

	mappings := r.effectiveMappings(baseDir)
	dirs := r.dirs()

	if p := cfpath.ResolvePathCached(component, baseDir, mappings, dirs); p != "" {
		return p
	}

	if appDir := r.FindApplicationRoot(baseDir); appDir != "" {
		if p := cfpath.ResolvePathCached(component, appDir, mappings, dirs); p != "" {
			return p
		}
	}

	for _, root := range r.WorkspaceFolders {
		if p := cfpath.ResolvePathCached(component, root, mappings, dirs); p != "" {
			return p
		}
	}

	if p := r.inFolderNamed(component, dirs); p != "" {
		return p
	}

	// A module's cfmapping is an application mapping ColdBox registers when
	// it loads the module.
	if p := r.inModuleMapping(component); p != "" {
		return p
	}

	if root, rest := r.slugRoot(component, baseDir); root != "" {
		if p := cfpath.ResolvePathCached(rest, root, nil, dirs); p != "" {
			return p
		}
	}

	// A framework component nothing on disk answers: its API, from the stubs.
	// Last of the dot-path lookups, so a workspace with the framework checked
	// out resolves to the real file exactly as it did.
	if p := r.Stubs.Path(component); p != "" {
		return p
	}

	// A path in a framework's own namespace names that framework whether or
	// not a preset does: `extends="testbox.system.compat.framework.TestCase"`
	// is TestBox, and Lucee's test suite extends it in 1,885 files with no
	// preset named.
	if p := frameworkapi.Namespaced(component); p != "" {
		return p
	}

	// An id a WireBox binder maps, `map( "Mailer" ).to( … )`: asked only
	// once no path answered, since reading the binders lists the index.
	if p, ok := r.wireboxID(component, baseDir); ok && p != "" {
		return p
	}

	return r.lastResortPath(component, baseDir)
}

// lastResortPath is componentPathUncached once no path has answered: an
// engine component, a file of the name nearest baseDir, an ORM entity, a
// bean, or a class a framework's WireBox maps by file name.
func (r *Resolver) lastResortPath(component, baseDir string) string {
	// A component the engine ships is the engine's, not whatever file of that
	// name the workspace happens to hold, so the file-name search below never
	// answers for one.
	if qualified, ok := engineComponent(component); ok {
		return r.engineSource(qualified)
	}

	// For bare names (no dots/slashes), search the index by filename as a last resort.
	// This handles `extends="BaseAssertionsTest"` where the file isn't in the same
	// directory or workspace root but is somewhere in the indexed workspace.
	if !strings.Contains(component, ".") && !strings.Contains(component, "/") && r.Index != nil {
		candidates := r.Index.FindFilesByBasename(component)
		// Lazy indexing of a namespaced stub must not make its bare name
		// visible to the workspace. Otherwise a parallel scan's answer depends
		// on whether another file has already used that framework component.
		// Preset ids still resolve through IDPackages below.
		candidates = slices.DeleteFunc(candidates, frameworkapi.IsStub)

		if len(candidates) == 1 {
			return candidates[0]
		}

		if len(candidates) > 1 {
			// Multiple matches — pick the one with the shortest relative path
			// from baseDir, and the lowest path among equals, as
			// Index.LookupPreferred does for a function. Several copies of a
			// component in sibling directories — ContentBox keeps one
			// RailoDBInfo.cfc per patch — are all equally near a sibling that
			// has none, and taking whichever came first made the answer, and
			// every code-map edge through it, change from run to run.
			best := ""
			bestDist := -1

			for _, c := range candidates {
				rel, err := filepath.Rel(baseDir, c)
				if err != nil {
					continue
				}

				dist := strings.Count(rel, string(filepath.Separator))

				if bestDist < 0 || dist < bestDist || dist == bestDist && c < best {
					best = c
					bestDist = dist
				}
			}

			if best != "" {
				return best
			}
		}
	}

	// An ORM entity by its entityname: entityNew( "cbAuthor" ) is the
	// Author.cfc that declares that name.
	if !strings.ContainsAny(component, "./") && r.Index != nil {
		if u := r.Index.LookupEntity(component); u != "" {
			return cfpath.FromURI(string(u))
		}
	}

	// A bean by name: DI/1's getBean( "userService" ) and the beanPaths a
	// configuration names, aliases included (cfpath.BuildBeanMap).
	if !strings.ContainsAny(component, "./") && r.Index != nil {
		if p := r.Index.LookupBean(component); p != "" && filepath.IsAbs(p) {
			return p
		}
	}

	// A bare id a framework's WireBox maps by file name: a CommandBox
	// module's `inject="FileSystem"` is commandbox.system.util.FileSystem.
	// After the workspace's own files, so a project's FileSystem.cfc wins.
	if !strings.ContainsAny(component, "./") {
		for _, pkg := range r.Stubs.IDPackages() {
			if p := r.ComponentPath(pkg+"."+component, baseDir); p != "" {
				return p
			}
		}
	}

	return ""
}

// slugRoot finds the package a dot-path names itself by: the nearest
// directory at or above baseDir whose box.json slug is the path's first
// segment. It returns that directory and the rest of the path.
//
// A CommandBox package is installed under its slug, so its own code spells
// its components that way: coldbox-platform's handlers extend
// `coldbox.system.EventHandler`, which is its own system/EventHandler.cfc. A
// checkout of the package has no mapping for it, and every such path was a
// component that did not exist — over ten thousand calls between
// coldbox-platform and TestBox. A configured mapping still comes first.
func (r *Resolver) slugRoot(component, baseDir string) (root, rest string) {
	first, rest, ok := strings.Cut(component, ".")
	if !ok || first == "" || rest == "" || r.FS == nil {
		return "", ""
	}

	for dir := baseDir; ; {
		if slug := r.boxSlug(dir); slug != "" && strings.EqualFold(slug, first) {
			return dir, rest
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ""
		}

		dir = parent
	}
}

// boxSlug is the slug of dir's box.json, "" when it has none; remembered per
// directory, since every unresolved path walks the same ancestors.
func (r *Resolver) boxSlug(dir string) string {
	r.mu.RLock()
	slug, ok := r.slugCache[dir]
	r.mu.RUnlock()

	if ok {
		return slug
	}

	if data, err := r.fs().ReadFile(filepath.Join(dir, "box.json")); err == nil {
		var box struct {
			Slug string `json:"slug"`
		}

		if json.Unmarshal(data, &box) == nil {
			slug = strings.TrimSpace(box.Slug)
		}
	}

	r.mu.Lock()
	if r.slugCache == nil {
		r.slugCache = make(map[string]string)
	}

	r.slugCache[dir] = slug
	r.mu.Unlock()

	return slug
}

// engineImports are the components an engine imports implicitly, so that
// `new Query()` and `new http()` need no path: Lucee's org.lucee.cfml.* and
// Adobe ColdFusion's com.adobe.coldfusion.*, which script-tag components
// share names across.
var engineImports = map[string]bool{
	"dbinfo":     true,
	"feed":       true,
	"ftp":        true,
	"http":       true,
	"ldap":       true,
	"mail":       true,
	"pop":        true,
	"query":      true,
	"storedproc": true,
}

const (
	luceeComponents = "org.lucee.cfml."
	adobeComponents = "com.adobe.coldfusion."
)

// engineComponent reports whether component names one the engine provides,
// and how Lucee's source tree spells it: bare `Query` is
// org.lucee.cfml.Query. An Adobe component has no source to find, so its
// spelling is "".
//
// It is asked only once nothing configured resolved the path, so a project's
// own Query.cfc beside the caller, or a mapping for org.lucee.cfml, still
// wins, as it does at runtime.
func engineComponent(component string) (qualified string, ok bool) {
	lower := strings.ToLower(component)

	switch {
	case engineImports[lower]:
		return luceeComponents + component, true
	case strings.HasPrefix(lower, luceeComponents) && len(lower) > len(luceeComponents):
		return component, true
	case strings.HasPrefix(lower, adobeComponents):
		return "", true
	}

	return "", false
}

// engineSource finds an engine component's own source in the workspace: an
// indexed file whose path ends in the component's path, as a Lucee checkout
// keeps org/lucee/cfml/Query.cfc and the org/lucee/cfml/test/LuceeTestCase
// its tests extend. Calls on it are then checked against the real methods.
// Without the source the component is still the engine's, and canResolveCall
// accepts calls on it as dynamic (engineComponent) rather than reporting a
// component that does not exist.
func (r *Resolver) engineSource(qualified string) string {
	if qualified == "" || r.Index == nil {
		return ""
	}

	candidates := r.Index.FindFilesByBasename(strings.ReplaceAll(qualified, ".", "/"))
	if len(candidates) == 0 {
		return ""
	}

	return candidates[0]
}

// inFolderNamed resolves a dot-path whose first segment names a workspace
// folder, inside that folder: with ../tassweb among the workspace folders,
// tassweb.packages.tass.core.kernel2 is <tassweb>/packages/tass/core/kernel2.cfc.
//
// That is what a mapping from a folder's name to the folder says, and it was
// the only kind of mapping tassweb's config held: all three of its mappings,
// tassweb, tassreporting and tassdoc, named a workspace folder by its own name.
// Without them 250,000 of its calls stopped resolving. A workspace folder now
// implies that mapping, so a project needs one only where the name differs
// from the folder.
//
// It comes after every configured lookup and before slugRoot, so an explicit mapping,
// an Application.cfc mapping and a path relative to the file, the application
// or a workspace folder all come first, and nothing that resolved before
// resolves differently now. The folders are the resolver's, which are the
// config's workspacePaths, or the editor's folders when the config names none.
func (r *Resolver) inFolderNamed(component string, dirs *cfpath.DirCache) string {
	dotted := strings.ReplaceAll(strings.TrimPrefix(component, "/"), "/", ".")

	seg, rest, ok := strings.Cut(dotted, ".")
	if !ok || seg == "" || rest == "" {
		return ""
	}

	for _, root := range r.WorkspaceFolders {
		if !strings.EqualFold(filepath.Base(root), seg) {
			continue
		}

		if p := cfpath.ResolvePathCached(rest, root, nil, dirs); p != "" {
			return p
		}
	}

	return ""
}

// EnsureIndexed ensures a CFC file is indexed, loading from disk if needed.
//
// The "needed" test is whether the index holds the file (HasFile), not whether
// it holds any functions for it. Those differ for a component that legitimately
// declares none — a property-only bean, a DTO, a `this`-scope struct — and
// asking the second question re-read and re-parsed every such file on every
// single lookup, since re-indexing it produced the same empty result.
func (r *Resolver) EnsureIndexed(cfcPath string) []*parser.FunctionDef {
	if r.indexer != nil {
		return r.indexer.EnsureIndexed(cfcPath)
	}

	cfcURI := cfpath.ToURI(cfcPath)

	if !r.Index.HasFile(cfcURI) {
		data, err := r.fs().ReadFile(cfcPath)
		if err != nil {
			return nil
		}

		r.Index.IndexFileWithOptions(cfcURI, string(data), &parser.ParseOptions{Resolvers: r.Resolvers, SetterLookup: r.SetterLookup(cfcPath), ConstructorLookup: r.ConstructorLookup(cfcPath), BeanLookup: r.InjectionBeanLookup(cfcPath), PropertyBeanLookup: r.InjectionPropertyLookup(cfcPath)})
	}

	return r.Index.FunctionsForFile(cfcURI)
}

// LookupFuncWithExtends searches for a function in cfcPath, walking the
// extends chain, and then the methods WireBox delegates into any component of
// the chain.
func (r *Resolver) LookupFuncWithExtends(cfcPath, funcName string) *parser.FunctionDef {
	return r.lookupFunc(cfcPath, funcName, 0)
}

// maxDelegateDepth bounds a delegate's own delegates: two components that
// delegate to each other must not recurse forever.
const maxDelegateDepth = 3

func (r *Resolver) lookupFunc(cfcPath, funcName string, depth int) *parser.FunctionDef {
	seen := make(map[string]bool)

	var chain []string

	for cfcPath != "" && !seen[cfcPath] {
		seen[cfcPath] = true
		chain = append(chain, cfcPath)
		cfcURI := cfpath.ToURI(cfcPath)

		// Mapper copies its package methods over existing members. An ambiguous
		// copy must not fall back to the definition it may have overwritten.
		if d, copied := r.wheelsMapperFunc(cfcPath, funcName); copied {
			return d
		}

		defs := r.EnsureIndexed(cfcPath)
		for _, d := range defs {
			if strings.EqualFold(d.Name, funcName) {
				return d
			}
		}

		ext, ok := r.extendsOf(cfcPath, cfcURI)
		if !ok || ext == "" {
			break
		}

		baseDir := filepath.Dir(cfcPath)
		cfcPath = r.ComponentPath(ext, baseDir)
	}

	// A template a component includes declares its functions into the
	// component: Wheels' Global.cfc is little but includes of global/*.cfm,
	// and application.wo.$simpleLock() is one of theirs.
	for _, p := range chain {
		if d := r.includedFunc(p, funcName); d != nil {
			return d
		}
	}

	if d := r.wheelsControllerFunc(chain, funcName); d != nil {
		return d
	}

	if d := r.wheelsTestGlobalFunc(chain, funcName); d != nil {
		return d
	}

	// A delegated method never replaces one the object has (Injector.cfc
	// processDelegation skips a name the target already holds), so the whole
	// chain is searched first.
	for _, p := range chain {
		if d := r.delegatedFunc(r.delegatesOf(p), filepath.Dir(p), funcName, depth); d != nil {
			return d
		}
	}

	// A Wheels association's methods: `hasMany( "comments" )` in config()
	// gives the model comments(), commentCount(), newComment() and the rest.
	if len(chain) > 0 && r.isWheelsModel(chain[0]) {
		if d := r.associationFunc(chain[0], funcName); d != nil {
			return d
		}
	}

	if d := r.mementoFunc(chain, funcName); d != nil {
		return d
	}

	return nil
}

// maxIncludeDepth bounds how far includedFunc follows templates that
// include templates.
const maxIncludeDepth = 4

// includedFunc is funcName as declared by a template cfcPath includes, or by
// one those include, maxIncludeDepth deep. Only a component's own include
// statements are read, from the index, rather than the workspace's include
// graph: that graph is rebuilt whenever an include is indexed, which a lazy
// lookup during a scan would do once per component.
func (r *Resolver) includedFunc(cfcPath, funcName string) *parser.FunctionDef {
	seen := map[string]bool{pathKey(cfcPath): true}
	queue := []string{cfcPath}

	for depth := 0; depth < maxIncludeDepth && len(queue) > 0; depth++ {
		var next []string

		for _, from := range queue {
			if !r.Index.HasFile(cfpath.ToURI(from)) {
				r.EnsureIndexed(from)
			}

			for _, raw := range r.Index.IncludesForFile(cfpath.ToURI(from)) {
				target := r.IncludePath(raw, from)
				if target == "" || seen[pathKey(target)] {
					continue
				}

				seen[pathKey(target)] = true

				for _, d := range r.EnsureIndexed(target) {
					if strings.EqualFold(d.Name, funcName) {
						return d
					}
				}

				next = append(next, target)
			}
		}

		queue = next
	}

	return nil
}

// delegatedFunc is the method one of delegates gives the component under
// name: the delegate target's own method, found with its extends chain and
// its own delegates. dir is the declaring file's, for the target's id.
func (r *Resolver) delegatedFunc(delegates []parser.Delegate, dir, name string, depth int) *parser.FunctionDef {
	if depth >= maxDelegateDepth {
		return nil
	}

	for i := range delegates {
		d := &delegates[i]

		inner := d.DelegatedMethod(name)
		if inner == "" {
			continue
		}

		target := r.ComponentPath(parser.DelegateTarget(d.Target), dir)
		if target == "" {
			continue
		}

		if def := r.lookupFunc(target, inner, depth+1); def != nil {
			return def
		}
	}

	return nil
}

// delegatesOf is the WireBox delegations cfcPath declares, from the index or,
// failing that, the file itself — recorded with its extends, as
// declaredExtendsOf does.
func (r *Resolver) delegatesOf(cfcPath string) []parser.Delegate {
	cfcURI := cfpath.ToURI(cfcPath)
	if ds, ok := r.Index.DelegatesForFile(cfcURI); ok {
		return ds
	}

	data, err := r.fs().ReadFile(cfcPath)
	if err != nil {
		return nil
	}

	pr := parser.Parse(cfcURI, string(data))
	r.Index.SetDelegates(cfcURI, pr.Delegates)

	return pr.Delegates
}

// mockDecoration reports whether name is a method MockBox adds to a mock
// (parser.IsMockDecoration). A test mocks a real component in place —
// prepareMock( event ), getMockRequestContext() — and then calls these on it,
// so a component that is otherwise known is the wrong place to look for
// them: event.$( "getValue" ) was "method '$' not found in RequestContext".
func mockDecoration(name string) bool {
	return parser.IsMockDecoration(name)
}

// fileMissingBase is MissingBase for the file's own extends chain, and
// whether a framework preset implied a link of it — the file's own base or
// one further up. A break reached through an implied link is the framework's
// source not being in the workspace, which callers accept as dynamic.
func (r *Resolver) fileMissingBase(pr *parser.ParseResult, baseDir string) (base string, implied bool) {
	ext := r.fileExtends(pr)

	return r.chainBreak(ext, baseDir, ext != "" && impliedBase(pr))
}

// impliedBase reports whether the file's extends chain is one a framework
// preset implied rather than one the file wrote. A preset's resolvers answer
// a component missing from the workspace as dynamic — the framework's source
// is simply not checked out — and a base it implies does the same, so a view
// in a ColdBox app without ColdBox on disk is not a list of findings its own
// code never caused. A base the file names is still reported when it breaks.
func impliedBase(pr *parser.ParseResult) bool {
	return pr.Extends == ""
}

// fileExtends is what the parsed file extends: its own extends attribute, or
// the component a framework preset says a file at its path extends. Every
// reader of the extends chain goes through it, so a handler with no extends
// resolves a bare getInstance() exactly as one declaring EventHandler does.
func (r *Resolver) fileExtends(pr *parser.ParseResult) string {
	if pr.Extends != "" || r.ImplicitExtends == nil {
		return pr.Extends
	}

	return r.extendsFor("", cfpath.FromURI(string(pr.URI)))
}

// extendsFor is declared, or ImplicitExtends' answer for path when declared is
// empty. Remembered per path: bare-call resolution asks once per call site.
func (r *Resolver) extendsFor(declared, path string) string {
	if declared != "" || r.ImplicitExtends == nil || path == "" {
		return declared
	}

	r.mu.RLock()
	ext, ok := r.implicitCache[path]
	r.mu.RUnlock()

	if ok {
		return ext
	}

	ext = r.ImplicitExtends(path)

	r.mu.Lock()
	if r.implicitCache == nil {
		r.implicitCache = make(map[string]string)
	}

	r.implicitCache[path] = ext
	r.mu.Unlock()

	return ext
}

// extendsOf reports what cfcPath extends, reading and parsing the file only if
// the index cannot say.
//
// This used to read and fully parse the component every time, to take one
// field off the result and discard the rest. That is the single most expensive
// thing the open path did: LookupFuncWithExtends runs once per unresolved call
// site, so a component with hundreds of them re-read and re-parsed the same
// bases hundreds of times. On a 540KB service.cfc, textDocument/didOpen
// allocated 35.8MB and took 57.7ms; through the index it is 13.2MB and 16.7ms.
// The bare parse of that file is 2.25MB, which is what says the cost was never
// the parser.
//
// It is also why allocation tracked call sites rather than file size — a
// persist.cfc three times larger allocated a third as much per byte.
//
// Nothing needs to invalidate this. The record lives in the index and
// removeFileEntries drops it, so any re-index forgets it and the next call
// re-establishes it from the file as it then stands.
func (r *Resolver) extendsOf(cfcPath string, cfcURI uri.URI) (string, bool) {
	ext, ok := r.declaredExtendsOf(cfcPath, cfcURI)
	if !ok {
		return "", false
	}

	return r.extendsFor(ext, cfcPath), true
}

// declaredExtendsOf is what cfcPath's own extends attribute says, before a
// framework preset's implied base: extendsOf's lookup, for the walk that has
// to know which links a file wrote.
func (r *Resolver) declaredExtendsOf(cfcPath string, cfcURI uri.URI) (string, bool) {
	if ext, ok := r.Index.ExtendsForFile(cfcURI); ok {
		return ext, true
	}

	data, err := r.fs().ReadFile(cfcPath)
	if err != nil {
		return "", false
	}

	ext := parser.Parse(cfcURI, string(data)).Extends
	r.Index.SetExtends(cfcURI, ext)

	return ext, true
}

// ResolveFunc finds a function definition by component path and function name,
// handling pipe-separated alternatives, absolute paths, and the extends chain.
func (r *Resolver) ResolveFunc(component, funcName, baseDir string) *parser.FunctionDef {
	if context := r.forCaller(baseDir); context != r {
		return context.ResolveFunc(component, funcName, baseDir)
	}

	alternatives := []string{component}
	if strings.Contains(component, "|") {
		alternatives = strings.Split(component, "|")
	}

	for _, alt := range alternatives {
		var cfcPath string
		if filepath.IsAbs(alt) {
			cfcPath = alt
		} else {
			cfcPath = r.ComponentPath(alt, baseDir)
		}

		if cfcPath == "" {
			continue
		}

		if d := r.LookupFuncWithExtends(cfcPath, funcName); d != nil {
			return d
		}
	}

	return nil
}

// HasFunction returns true if the component has a function with the given name.
func (r *Resolver) HasFunction(component, funcName, baseDir string) bool {
	cfcPath := r.ComponentPath(component, baseDir)
	if cfcPath == "" {
		return false
	}

	for _, d := range r.EnsureIndexed(cfcPath) {
		if strings.EqualFold(d.Name, funcName) {
			return true
		}
	}

	return false
}

// FindApplicationRoot walks up from dir looking for Application.cfc or Application.cfm.
func (r *Resolver) FindApplicationRoot(dir string) string {
	r.mu.RLock()

	if r.appRootCache != nil {
		if v, ok := r.appRootCache[dir]; ok {
			r.mu.RUnlock()

			return v
		}
	}

	r.mu.RUnlock()

	result := r.findApplicationRootUncached(dir)

	r.mu.Lock()
	if r.appRootCache == nil {
		r.appRootCache = make(map[string]string)
	}

	r.appRootCache[dir] = result
	r.mu.Unlock()

	return result
}

func (r *Resolver) findApplicationRootUncached(dir string) string {
	d := dir

	for {
		for _, name := range []string{"Application.cfc", "Application.cfm"} {
			if _, err := r.fs().Stat(filepath.Join(d, name)); err == nil {
				return d
			}
		}

		parent := filepath.Dir(d)
		if parent == d {
			return ""
		}

		d = parent
	}
}

// EffectiveMappings returns config mappings merged with Application.cfc mappings.
func (r *Resolver) EffectiveMappings(baseDir string) map[string]string {
	return r.effectiveMappings(baseDir)
}

func (r *Resolver) effectiveMappings(baseDir string) map[string]string {
	if r.indexer != nil {
		return r.callerMappings
	}

	appDir := r.FindApplicationRoot(baseDir)
	if appDir == "" {
		appDir = baseDir
	}

	appMappings := cfpath.LoadAppMappings(appDir)
	if len(appMappings) == 0 {
		return r.Mappings
	}

	if len(r.Mappings) == 0 {
		return appMappings
	}

	merged := make(map[string]string, len(appMappings)+len(r.Mappings))
	maps.Copy(merged, appMappings)

	for key, value := range r.Mappings {
		for existing := range merged {
			if strings.EqualFold(existing, key) {
				delete(merged, existing)
			}
		}

		merged[key] = value
	}

	return merged
}

// ResolveFromCall resolves a call expression against configured resolvers.
func (r *Resolver) ResolveFromCall(expr string) string {
	return parser.ResolveFromCall(expr, r.Resolvers)
}

// CanResolveCall determines whether a function call can be resolved given the
// parse result context. Returns empty string if resolved, or a reason if not.
func (r *Resolver) CanResolveCall(call *parser.CallSite, pr *parser.ParseResult, baseDir string) string {
	return r.forCaller(baseDir).canResolveCall(call, pr, baseDir, nil)
}

// ExplainCall runs the same resolution logic as CanResolveCall but also returns a
// human-readable trace of every decision point that produced (or failed to produce)
// the final verdict — which mechanism set the call's component, which componentResolver
// or FuncLookup hop fired, and why the final method check succeeded or failed. Intended
// for the `explain` CLI command; not used on the hot lint path.
func (r *Resolver) ExplainCall(call *parser.CallSite, pr *parser.ParseResult, baseDir string) (string, []string) {
	tr := &callTrace{}
	reason := r.forCaller(baseDir).canResolveCall(call, pr, baseDir, tr)

	return reason, tr.steps
}

// callTrace accumulates what canResolveCall learned on the way to its verdict: a
// human-readable step list for [Resolver.ExplainCall], and the callee it landed on
// for [Resolver.ResolveCallTarget]. A nil *callTrace is always safe to call add or
// hit on (both no-op), so canResolveCall can be shared between the hot lint path
// (tr == nil) and its two introspecting callers without extra branching.
//
// Recording the target here rather than widening canResolveCall's return is
// deliberate: the function has twenty-odd `return ""` sites, and threading a second
// value through every one of them is exactly the kind of hand-maintained parallel
// list this codebase keeps getting bitten by. A recorder lets each site opt in where
// it already opts in to a trace step, and a site that forgets degrades to
// TargetNone — an edge the call graph reports as unresolved — rather than to a wrong
// edge. TestEveryAcceptPathRecordsATarget pins the set that must not forget.
type callTrace struct {
	steps  []string
	target CallTarget
}

func (t *callTrace) addf(format string, args ...any) {
	if t == nil {
		return
	}

	t.steps = append(t.steps, fmt.Sprintf(format, args...))
}

// hit records where a call site resolved to. def may be nil for the dynamic kinds,
// where there is a component (or not even that) but no definition to point at.
func (t *callTrace) hit(kind TargetKind, component string, def *parser.FunctionDef) {
	if t == nil {
		return
	}

	t.target.Kind = kind
	t.target.Component = component

	if def != nil {
		t.target.URI = def.URI
		t.target.FuncName = def.Name
		t.target.Line = def.Line
	}
}

func (r *Resolver) canResolveCall(call *parser.CallSite, pr *parser.ParseResult, baseDir string, tr *callTrace) string {
	funcName := call.FuncName
	variable := call.Variable

	// Unqualified call — check same file, then extends chain.
	// Skip if call.Component is already set (e.g. resolved via chained new/createObject).
	if variable == "" && call.Component == "" {
		if len(call.Chain) > 0 {
			return r.resolveBareChain(call, pr, baseDir, tr)
		}

		return r.resolveBareCall(call, pr, baseDir, tr)
	}

	// this. qualifier — refers to the current component
	if strings.EqualFold(variable, "this") {
		return r.resolveThisCall(funcName, pr, baseDir, tr)
	}

	// super. qualifier
	if strings.EqualFold(variable, "super") {
		return r.resolveSuperCall(funcName, pr, baseDir, tr)
	}

	// Bare VARIABLES qualifier (no further dot, e.g. "VARIABLES.someName(...)") —
	// a scope can't itself be a component ref, so treating "VARIABLES" as the thing
	// needing a ComponentRef is a category error. What's actually being called is a
	// property assigned onto the scope (e.g. "VARIABLES.someName = ARGUMENTS.callback;
	// ... VARIABLES.someName(...)"), a function-reference value passed in and stored —
	// same reasoning as the ARGUMENTS-as-function-reference case below, just for a
	// VARIABLES-scoped property instead of a bare argument. Accept if funcName was
	// ever assigned in VARIABLES scope anywhere in the file (assignments inside any
	// function are visible from every other function, unlike a var-scoped local).
	if strings.EqualFold(variable, "VARIABLES") {
		tr.addf("VARIABLES.%s called as a function reference — checking for a VARIABLES-scoped assignment", funcName)

		if pr.HasScopedAssignment(parser.ScopeVariables, funcName) {
			tr.addf("%q was assigned in VARIABLES scope — treating as a call through a function-reference property", funcName)
			tr.hit(TargetDynamic, "", nil)

			return ""
		}
	}

	// ARGUMENTS qualifier — funcName is being called as a function-reference argument.
	// These are dynamic (type="any"), so we can't verify them statically; accept if the
	// argument is declared in the enclosing function.
	if strings.EqualFold(variable, "ARGUMENTS") {
		tr.addf("ARGUMENTS.%s called as a function reference — checking caller %q's argument list", funcName, call.Caller)

		if declaresArgument(pr, call.Caller, funcName) {
			tr.hit(TargetDynamic, "", nil)

			return ""
		}
	}

	// Qualified call — find the component from refs
	comp := call.Component

	// softComp is a component a dynamicIfMissing resolver produced during this
	// resolution; the parse records its own (ParseResult.IsSoftComponent).
	softComp := ""

	if comp != "" {
		tr.addf("call.Component already set to %q (resolved earlier via chained new/createObject)", comp)
	}

	if comp == "" {
		var member bool

		comp, member = r.receiverComponent(variable, call.Line, call.Caller, funcName, pr, baseDir, tr)
		if member {
			tr.hit(TargetMember, "", nil)

			return ""
		}
	}

	if comp == "" {
		// Try component resolvers
		var noFollow, soft bool

		comp, noFollow, soft = r.matchResolver(variable, tr, func(c, desc string, nf bool) {
			tr.addf("resolved %q to %q via componentResolver matching the variable name [%s] (noFollow=%v)",
				variable, c, desc, nf)
		})
		if soft {
			softComp = comp
		}

		if noFollow && comp != "" {
			tr.hit(TargetDynamic, comp, nil)

			return ""
		}
	}

	if comp == "" && call.Text != "" {
		// Try resolvers against the full line text (handles chained calls like x.method().prop.func())
		var noFollow, soft bool

		comp, noFollow, soft = r.matchResolver(call.Text, tr, func(c, desc string, nf bool) {
			tr.addf("resolved %q to %q via componentResolver matching the full line text %q [%s] (noFollow=%v)",
				variable, c, call.Text, desc, nf)
		})
		if soft {
			softComp = comp
		}

		if noFollow && comp != "" {
			tr.hit(TargetDynamic, comp, nil)

			return ""
		}
	}

	if comp == "" {
		tr.addf("no ComponentRef and no componentResolver matched %q", variable)
	}

	// Walk any intermediate .method() hops between the resolved base and this
	// call (e.g. "kpg.generateKeyPair().getPublic().getParams()" — comp here is
	// kpg's own component; call.Chain lists "generateKeyPair", "getPublic", each
	// needing its own declared return type applied before checking funcName below).
	if comp != "" && comp != "$any" && !strings.HasPrefix(comp, "$builtin.") {
		var (
			reason string
			done   bool
		)

		if comp, softComp, reason, done = r.walkHops(comp, softComp, call, pr, baseDir, tr); done {
			return reason
		}
	}

	return r.checkMethodOn(comp, softComp, call, pr, baseDir, tr)
}

// walkHops follows call.Chain from comp through each hop's return type. It
// reports the component the chain reaches, or done with the answer when a hop
// settles the call itself.
func (r *Resolver) walkHops(comp, softComp string, call *parser.CallSite, pr *parser.ParseResult, baseDir string, tr *callTrace) (reached, soft, reason string, done bool) {
	funcName := call.FuncName

	for i, hop := range call.Chain {
		// A hop carries its call's arguments when the parser could read
		// them; every check and every reason below is about its name.
		expression, callHop := parser.CallExpression(hop)
		if callHop {
			hop = parser.CallHopName(hop)
		}

		// A mock of a class is the class, or dynamic where it names none.
		if cls, ok := strings.CutPrefix(comp, parser.MockPrefix); ok {
			comp = cls
			if r.ComponentPath(cls, baseDir) == "" {
				comp = "$any"
			}
		}

		// A hop that returned "$any" makes the rest of the chain dynamic.
		// Walking on asked "$any" for the next method, which it can never
		// define: getSandBox("x").getAttendanceObj().getLog() was reported
		// as getAttendanceObj missing from $any. The accept after the walk
		// is the answer for a dynamic receiver.
		if comp == "$any" {
			tr.addf("chain hop %q is on a dynamic ($any) value — the rest of the chain is dynamic", hop)

			break
		}

		if property, ok := parser.PropertyName(hop); ok {
			ret := r.publicPropertyComponent(comp, property, baseDir)
			if ret == "" {
				return comp, softComp, "property '" + property + "' in " + displayComponent(comp) + " has no component type (chain to '" + funcName + "')", true
			}

			comp = ret

			continue
		}

		fd := r.ResolveFunc(comp, hop, baseDir)

		// A decoration returns the mock it is called on, so the chain goes on
		// from the same component: model.$( "getCache", c ).getFoo() is
		// checked against model's class.
		if fd == nil && mockDecoration(hop) {
			tr.addf("chain hop %q is a method MockBox adds to a mock, and returns the mock — %q", hop, comp)

			continue
		}

		if fd == nil {
			return comp, softComp, r.missingChainHop(comp, softComp, hop, funcName, pr, baseDir, tr), true
		}

		ret, noFollow, soft := r.hopReturn(comp, hop, fd, baseDir, tr)
		if ret == "" && !callHop {
			ret = r.expressionReturn(fd, factoryCallExpression(pr.Content, hop, int(call.Line)), baseDir, comp)
		}

		if ret == "" && callHop {
			ret = r.expressionReturn(fd, expression, baseDir, comp)
		}

		if soft {
			softComp = ret
		}

		if noFollow && ret != "" {
			tr.hit(TargetDynamic, ret, nil)

			return comp, softComp, "", true
		}

		if ret == "" {
			// What an untyped hop returns is a mock when the next call on it
			// is one MockBox adds: `c.getRequestService().$( "getContext", x )`.
			next := funcName
			if i+1 < len(call.Chain) {
				next = parser.CallHopName(call.Chain[i+1])
			}

			if mockDecoration(next) {
				tr.addf("chain hop %q declares no component, and %q is called on what it returns — a mock, so the rest of the chain is dynamic", hop, next)
				tr.hit(TargetDynamic, "", nil)

				return comp, softComp, "", true
			}

			return comp, softComp, "method '" + hop + "' in " + displayComponent(comp) + " has no component return type (chain to '" + funcName + "')", true
		}

		comp = ret
	}

	return comp, softComp, "", false
}

// hopReturn is chainHopReturn, and failing that what the receiver binds the
// method to: a cborm service's entity, a Wheels model's own type.
func (r *Resolver) hopReturn(comp, hop string, fd *parser.FunctionDef, baseDir string, tr *callTrace) (ret string, noFollow, soft bool) {
	ret, noFollow, soft = r.chainHopReturn(comp, hop, fd, tr)
	if sub := r.selfTyped(comp, baseDir, fd, ret); sub != "" {
		tr.addf("chain hop %q returns the class declaring it, and is called on its subclass %q", hop, sub)
		ret = sub
	}

	if ret == "" {
		if e := r.receiverReturn(r.ComponentPath(comp, baseDir), hop); e != "" {
			tr.addf("chain hop %q returns what it is called on binds it to: %q", hop, e)
			ret = e
		}
	}

	return ret, noFollow, soft
}

// resolveBareCall is canResolveCall for an unqualified call: this file, a
// function-reference local, the extends chain, cfinclude, and a
// variables-scope function reference.
func (r *Resolver) resolveBareCall(call *parser.CallSite, pr *parser.ParseResult, baseDir string, tr *callTrace) string {
	funcName := call.FuncName

	tr.addf("unqualified call to %q — checking same file", funcName)

	for i := range pr.Funcs {
		if strings.EqualFold(pr.Funcs[i].Name, funcName) {
			tr.hit(TargetSameFile, "", &pr.Funcs[i])
			tr.addf("found %q defined in this file", funcName)

			return ""
		}
	}

	// A bare call whose name matches a declared local variable or argument in
	// the enclosing function is a call through a function-reference value (e.g.
	// "var columnfn = ARGUMENTS.extend[x].column; ... columnfn(...)"), not a call
	// to a missing/undefined function — CFML supports first-class function
	// values assigned to locals/arguments. This is a genuinely different case
	// from "no qualifier, not in file": the identifier IS declared, its value is
	// just dynamic (unknown until runtime), so — same reasoning as the
	// ARGUMENTS.x-as-function-reference case below — there's nothing further to
	// verify statically, and it should not be reported as if it were a missing
	// function.
	if scope := parser.FindFuncScopeAt(int(call.Line), pr.Scopes); scope.Start != -1 {
		for _, v := range pr.FuncVars(scope.Start, scope.End) {
			if strings.EqualFold(v, funcName) {
				tr.addf("%q is a declared local variable/argument in the enclosing function — treating as a call through a function-reference value, not a missing function", funcName)
				tr.hit(TargetDynamic, "", nil)

				return ""
			}
		}
	}

	// Check extends chain
	if r.fileExtends(pr) != "" {
		tr.addf("not in this file — checking extends chain (%s)", r.fileExtends(pr))

		if def := r.inheritedFunc(pr, funcName, baseDir); def != nil {
			tr.hit(TargetExtends, r.fileExtends(pr), def)
			tr.addf("found %q in extends chain", funcName)

			return ""
		}
	}

	if def := r.delegatedFunc(pr.Delegates, baseDir, funcName, 0); def != nil {
		tr.hit(TargetExtends, "", def)
		tr.addf("found %q among the methods a WireBox delegate gives this component", funcName)

		return ""
	}

	if def, via := r.findThroughIncludes(pr, funcName); def != nil {
		tr.hit(TargetInclude, "", def)
		tr.addf("found %q through cfinclude, in %s", funcName, via)

		return ""
	}

	// CFML looks a bare name up in the variables scope, so a bare call to
	// one the file assigns there — VARIABLES.render = ARGUMENTS.render —
	// is a call through that function-reference property, the same as the
	// qualified VARIABLES.render() accepted below. The parser records
	// VARIABLES.name() as a bare call wherever it reads it as a member of
	// this component, which is what left `#VARIABLES._renderTemplate()#`
	// in a string "no qualifier, not in file".
	if pr.HasScopedAssignment(parser.ScopeVariables, funcName) {
		tr.addf("%q is assigned in VARIABLES scope — a call through a function-reference property", funcName)
		tr.hit(TargetDynamic, "", nil)

		return ""
	}

	if module, ok := r.moduleHelper(pr, funcName, baseDir); ok {
		tr.addf("%q is a helper the %s module mixes into ColdBox components", funcName, module)
		tr.hit(TargetDynamic, "", nil)

		return ""
	}

	if r.thisCallAnswered(call, pr, baseDir) {
		tr.addf("%q is called through this., and the component answers a missing method with onMissingMethod", funcName)
		tr.hit(TargetDynamic, "", nil)

		return ""
	}

	if r.fileExtends(pr) != "" {
		if base, implied := r.fileMissingBase(pr, baseDir); base != "" {
			if implied {
				tr.addf("the framework base %s implies does not resolve (%s) — accepted as dynamic", r.fileExtends(pr), base)
				tr.hit(TargetDynamic, "", nil)

				return ""
			}

			return MissingBaseReason(base)
		}

		return "not found in extends chain"
	}

	return "no qualifier, not in file"
}

// resolveBareChain is canResolveCall for a call chained onto a bare one,
// `expect( x ).toBe( 1 )`: the first call is looked up as a bare call is —
// this file, its extends chain, what it includes — and the rest of the chain
// is checked on what it returns.
//
// It was resolved as a bare call to the *last* name, which looked for toBe
// among the spec's own methods and reported it "not found in extends chain"
// — once the chain resolved, in every TestBox assertion.
func (r *Resolver) resolveBareChain(call *parser.CallSite, pr *parser.ParseResult, baseDir string, tr *callTrace) string {
	first := parser.CallHopName(call.Chain[0])

	// The hop carries its arguments when the parser read them. Recovering
	// them from the line instead finds nothing when the name is called
	// twice there.
	expression, ok := parser.CallExpression(call.Chain[0])
	if !ok {
		expression = factoryCallExpression(pr.Content, first, int(call.Line))
	}

	tr.addf("chained on a call to %q — looking it up as an unqualified call", first)

	def := r.bareFunc(first, pr, baseDir)
	if def == nil {
		if r.fileExtends(pr) != "" {
			if base, implied := r.fileMissingBase(pr, baseDir); base != "" {
				if implied {
					tr.addf("the framework base %s implies does not resolve (%s) — the chain is dynamic", r.fileExtends(pr), base)
					tr.hit(TargetDynamic, "", nil)

					return ""
				}

				return MissingBaseReason(base)
			}
		}

		// A built-in function returns a value, not a component:
		// getPageContext().getRequest() and now().format() are calls on what
		// the engine hands back, which the method lists here only partly
		// know. The file's own function of that name was looked for first.
		if docs.IsBuiltinFunction(first) {
			tr.addf("%q is a built-in function — the chain on what it returns is dynamic", first)
			tr.hit(TargetDynamic, "", nil)

			return ""
		}

		return "chained on '" + first + "', which is not found (calling '" + call.FuncName + "')"
	}

	ret, noFollow, soft := r.chainHopReturn("this component", first, def, tr)
	if ret == "" && expression != "" {
		ret = r.expressionReturn(def, expression, baseDir, pr.URI.Path())
	}

	if ret == "" {
		if e := r.receiverReturn(cfpath.FromURI(string(pr.URI)), first); e != "" {
			tr.addf("%q on this component returns what it binds it to: %q", first, e)
			ret = e
		}
	}

	if noFollow && ret != "" {
		tr.hit(TargetDynamic, ret, nil)

		return ""
	}

	// A dynamicIfMissing resolver's answer that names no file is dynamic here
	// as on every other path; recursing lost the flag, and the next hop
	// reported the component as one that does not exist.
	if soft && ret != "" && !r.componentExists(ret, baseDir) {
		tr.addf("%q names no file and came from a dynamicIfMissing resolver — the rest of the chain is dynamic", ret)
		tr.hit(TargetDynamic, ret, nil)

		return ""
	}

	if ret == "" {
		return "method '" + first + "' has no component return type (chain to '" + call.FuncName + "')"
	}

	rest := *call
	rest.Chain = call.Chain[1:]
	rest.Component = ret

	return r.canResolveCall(&rest, pr, baseDir, tr)
}

// bareFunc finds what an unqualified call to name reaches: this file's own
// function, one its extends chain declares, or one a file it includes or is
// included by declares.
func (r *Resolver) bareFunc(name string, pr *parser.ParseResult, baseDir string) *parser.FunctionDef {
	for i := range pr.Funcs {
		if strings.EqualFold(pr.Funcs[i].Name, name) {
			return &pr.Funcs[i]
		}
	}

	if r.fileExtends(pr) != "" {
		if def := r.inheritedFunc(pr, name, baseDir); def != nil {
			return def
		}
	}

	if def := r.delegatedFunc(pr.Delegates, baseDir, name, 0); def != nil {
		return def
	}

	def, _ := r.findThroughIncludes(pr, name)

	return def
}

// resolveThisCall is canResolveCall for `this.name()`.
func (r *Resolver) resolveThisCall(funcName string, pr *parser.ParseResult, baseDir string, tr *callTrace) string {
	tr.addf("'this' qualifier — checking same file")

	for i := range pr.Funcs {
		if strings.EqualFold(pr.Funcs[i].Name, funcName) {
			tr.hit(TargetSameFile, "", &pr.Funcs[i])

			return ""
		}
	}

	if r.fileExtends(pr) != "" {
		tr.addf("not in this file — checking extends chain (%s)", r.fileExtends(pr))

		if def := r.inheritedFunc(pr, funcName, baseDir); def != nil {
			tr.hit(TargetExtends, r.fileExtends(pr), def)

			return ""
		}
	}

	if def := r.delegatedFunc(pr.Delegates, baseDir, funcName, 0); def != nil {
		tr.hit(TargetExtends, "", def)
		tr.addf("found %q among the methods a WireBox delegate gives this component", funcName)

		return ""
	}

	if def, via := r.findThroughIncludes(pr, funcName); def != nil {
		tr.hit(TargetInclude, "", def)
		tr.addf("found %q through cfinclude, in %s", funcName, via)

		return ""
	}

	if r.fileExtends(pr) != "" {
		if base, implied := r.fileMissingBase(pr, baseDir); base != "" {
			if implied {
				tr.addf("the framework base %s implies does not resolve (%s) — accepted as dynamic", r.fileExtends(pr), base)
				tr.hit(TargetDynamic, "", nil)

				return ""
			}

			return MissingBaseReason(base)
		}
	}

	return "method '" + funcName + "' not found in current component"
}

// resolveSuperCall is canResolveCall for `super.name()`.
func (r *Resolver) resolveSuperCall(funcName string, pr *parser.ParseResult, baseDir string, tr *callTrace) string {
	if r.fileExtends(pr) == "" {
		return "super used but no extends"
	}

	tr.addf("'super' qualifier — checking extends chain (%s)", r.fileExtends(pr))

	if def := r.ResolveFunc(r.fileExtends(pr), funcName, baseDir); def != nil {
		tr.hit(TargetExtends, r.fileExtends(pr), def)

		return ""
	}

	if base, implied := r.fileMissingBase(pr, baseDir); base != "" {
		if implied {
			tr.addf("the framework base %s implies does not resolve (%s) — accepted as dynamic", r.fileExtends(pr), base)
			tr.hit(TargetDynamic, "", nil)

			return ""
		}

		return MissingBaseReason(base)
	}

	return "not found in parent component"
}

// missingBasePrefix starts MissingBaseReason's answer, which MissingBaseOf
// reads back.
const missingBasePrefix = "extends chain breaks at "

// MissingBaseReason is canResolveCall's answer for an inherited call when a
// component in the file's extends chain does not resolve. The method may well
// be declared there; what is missing is the base, and a report that blames the
// call names the wrong problem once per call — 56,000 times over the six-project
// corpus, for a dozen missing bases.
func MissingBaseReason(base string) string {
	return missingBasePrefix + "'" + base + "', which does not resolve"
}

// MissingBaseOf reports the base a MissingBaseReason names.
func MissingBaseOf(reason string) (string, bool) {
	rest, ok := strings.CutPrefix(reason, missingBasePrefix+"'")
	if !ok {
		return "", false
	}

	base, _, ok := strings.Cut(rest, "'")

	return base, ok
}

// inheritedFromMissingBase is the base a receiver with no component is taken
// to come from: the missing link of the file's extends chain, when the
// receiver is an unscoped or variables./this. name the file never declares.
// `print` in a CommandBox command and `$assert` in a TestBox spec are the
// base's, injected or declared there, and with the base missing they are as
// unchecked as its methods. A name in any other scope, or one the file
// declares, is not the base's to explain.
func (r *Resolver) inheritedFromMissingBase(variable string, line uint32, pr *parser.ParseResult, baseDir string) (base string, implied bool) {
	if r.fileExtends(pr) == "" || variable == "" || strings.ContainsAny(variable, "[(") {
		return "", false
	}

	root, rest, _ := strings.Cut(variable, ".")
	if strings.EqualFold(root, "variables") || strings.EqualFold(root, "this") {
		root, _, _ = strings.Cut(rest, ".")
	} else if isScopeName(root) {
		return "", false
	}

	if root == "" || pr.Declares(root) {
		return "", false
	}

	// A closure's locals and parameters and a catch variable are the
	// enclosing function's to the body scan, and not in the file-wide
	// declarations.
	if scope := parser.FindFuncScopeAt(int(line), pr.Scopes); scope.Start != -1 {
		for _, v := range pr.FuncVars(scope.Start, scope.End) {
			if strings.EqualFold(v, root) {
				return "", false
			}
		}
	}

	return r.fileMissingBase(pr, baseDir)
}

// isScopeName reports whether name is a CFML scope other than variables and
// this, whose members a base component does not supply.
func isScopeName(name string) bool {
	switch strings.ToLower(name) {
	case "arguments", "local", "request", "session", "application", "server",
		"url", "form", "cgi", "cookie", "client", "super", "attributes", "caller", "thread":
		return true
	default:
		return false
	}
}

// MissingBase returns the first component in the extends chain starting at
// extends, from a file in baseDir, that does not resolve to a file; "" when
// every link resolves, or the chain loops.
func (r *Resolver) MissingBase(extends, baseDir string) string {
	base, _ := r.chainBreak(extends, baseDir, false)

	return base
}

// chainBreak walks the extends chain from extends and names the first link
// that does not resolve, and whether any link up to it was one a framework
// preset implied rather than one a file wrote — implied says so of the first.
// A list of alternatives is a component a file cannot declare, so it answers
// nothing, unless it was implied: a Wheels view's base is its controller and
// every mixin, and any of them missing is the framework's source missing.
func (r *Resolver) chainBreak(extends, baseDir string, implied bool) (string, bool) {
	seen := make(map[string]bool)

	for extends != "" {
		if strings.Contains(extends, "|") {
			if !implied {
				return "", false
			}

			for alt := range strings.SplitSeq(extends, "|") {
				if base, _ := r.chainBreak(alt, baseDir, true); base != "" {
					return base, true
				}
			}

			return "", false
		}

		p := r.ComponentPath(extends, baseDir)
		if p == "" {
			return extends, implied
		}

		if seen[p] {
			return "", false
		}

		seen[p] = true

		// A chain that reaches a framework's stubs and breaks beyond them is
		// the framework's source not being here — BaseTestCase's TestBox base
		// in a workspace naming ColdBox but not TestBox — not a finding.
		implied = implied || frameworkapi.IsStub(p)

		declared, ok := r.declaredExtendsOf(p, cfpath.ToURI(p))
		if !ok {
			return "", false
		}

		extends, baseDir = declared, filepath.Dir(p)
		if declared == "" {
			extends = r.extendsFor("", p)
			implied = implied || extends != ""
		}
	}

	return "", false
}

// matchResolver tries the componentResolvers against text. It reports the
// component, the resolver's noFollow, and whether that resolver is
// dynamicIfMissing; describe is told about a match, for the trace.
func (r *Resolver) matchResolver(text string, tr *callTrace, describe func(comp, desc string, noFollow bool)) (comp string, noFollow, soft bool) {
	comp, noFollow, idx := parser.ResolveFromCallMatch(text, r.Resolvers)
	soft = idx >= 0 && r.Resolvers[idx].DynamicIfMissing

	if comp != "" && tr != nil {
		describe(comp, r.describeResolver(idx), noFollow)
	}

	return comp, noFollow, soft
}

// missingChainHop is canResolveCall's answer when a hop of a chained call is
// not a method of the component the chain has reached.
func (r *Resolver) missingChainHop(comp, softComp, hop, funcName string, pr *parser.ParseResult, baseDir string, tr *callTrace) string {
	if !r.componentExists(comp, baseDir) {
		if softMissing(comp, softComp, pr) {
			tr.addf("%q names no file and came from a dynamicIfMissing resolver — the rest of the chain is dynamic", comp)
			tr.hit(TargetDynamic, comp, nil)

			return ""
		}

		if r.uninstalledModule(comp) {
			tr.addf("%q is a module's model and no such module is in the workspace — the rest of the chain is dynamic", comp)
			tr.hit(TargetDynamic, comp, nil)

			return ""
		}

		if _, ok := engineComponent(comp); ok {
			tr.addf("%q is a component the engine provides and its source is not in the workspace — the rest of the chain is dynamic", comp)
			tr.hit(TargetDynamic, comp, nil)

			return ""
		}

		return "component '" + displayComponent(comp) + "' does not exist (chain hop '" + hop + "' to '" + funcName + "')"
	}

	if mockDecoration(hop) {
		tr.addf("%q is a method MockBox adds to a mock — chain hop accepted, the rest of the chain is dynamic", hop)
		tr.hit(TargetDynamic, comp, nil)

		return ""
	}

	// A hop onMissingMethod answers is as valid as a last call it answers, and
	// what it returns is whatever the dispatcher decides. Lucee's own Http and
	// Query build their setters this way: new Http().setUrl(u).send().
	if r.ResolveFunc(comp, "onMissingMethod", baseDir) != nil {
		tr.addf("%q defines onMissingMethod — chain hop %q accepted, the rest of the chain is dynamic", comp, hop)
		tr.hit(TargetDynamic, comp, nil)

		return ""
	}

	if r.isInterface(r.ComponentPath(comp, baseDir)) {
		tr.addf("%q is an interface, and what implements it may have %q — the rest of the chain is dynamic", comp, hop)
		tr.hit(TargetDynamic, comp, nil)

		return ""
	}

	if base, implied := r.componentMissingBase(comp, baseDir); base != "" {
		if implied {
			tr.addf("%q's framework base does not resolve (%s) — chain hop %q accepted, the rest of the chain is dynamic", comp, base, hop)
			tr.hit(TargetDynamic, comp, nil)

			return ""
		}

		return MissingBaseReason(base)
	}

	return "method '" + hop + "' not found in " + displayComponent(comp) + " (chain to '" + funcName + "')"
}

// componentMissingBase is MissingBase for the chain comp extends: a method
// comp does not declare may be its base's, and when a link of that chain
// names no file the method was never looked for. ContentBox's services extend
// cborm's VirtualEntityService, and without cborm on disk every findWhere,
// save and list was "not found in" the service.
func (r *Resolver) componentMissingBase(comp, baseDir string) (base string, implied bool) {
	if strings.Contains(comp, "|") {
		return "", false
	}

	p := comp
	if !filepath.IsAbs(p) {
		p = r.ComponentPath(comp, baseDir)
	}

	if p == "" {
		return "", false
	}

	declared, ok := r.declaredExtendsOf(p, cfpath.ToURI(p))
	if !ok {
		return "", false
	}

	ext := declared
	if ext == "" {
		ext = r.extendsFor("", p)
	}

	return r.chainBreak(ext, filepath.Dir(p), declared == "" && ext != "")
}

// chainHopReturn is the component a chain hop's method returns: its declared
// or inferred component, a dotted return type, the receiver for an untyped
// init(), or a componentResolver matching the hop. It reports the resolver's
// noFollow and whether the resolver is dynamicIfMissing, for the last.
func (r *Resolver) chainHopReturn(comp, hop string, fd *parser.FunctionDef, tr *callTrace) (ret string, noFollow, soft bool) {
	ret = r.ReturnComponentOf(fd)
	if ret != "" {
		tr.addf("chain hop %q on %q: returns %q (declared %q)", hop, comp, ret, fd.ReturnType)
	}

	// An init() that declares nothing returns the object it was called
	// on: the CFC constructor convention, and what the parser assumes
	// when it types a variable assigned through one. Falling through to
	// the resolvers offered them init(), which nothing can answer.
	if ret == "" && strings.EqualFold(hop, "init") {
		ret = comp

		tr.addf("chain hop %q on %q: an untyped init() returns the object it is called on", hop, comp)
	}

	if ret != "" {
		return ret, false, false
	}

	// The real function's declared return type isn't a component (e.g.
	// a generic returntype="struct" on a factory method that actually
	// returns a specific component instance) — fall back to a
	// componentResolver matching the hop's own call shape, same as the
	// non-chain "altComp" fallback below.
	return r.matchResolver(hop+"()", tr, func(c, _ string, nf bool) {
		tr.addf("chain hop %q on %q: no declared return type — componentResolver matched %q(): %q (noFollow=%v)", hop, comp, hop, c, nf)
	})
}

// ReturnComponentOf is the component fd returns: the one the parse inferred
// from its return statements, a dotted return type, or a bare-word return type
// naming a component beside the declaring one. Every question about what a
// function returns asks this, so a chain hop and a variable assigned from the
// same call cannot disagree.
//
// An inferred `$any` does not outrank a declared component, dotted or a bare
// word naming one beside the declaring file. It means the body
// builds its result from a runtime expression (`getCustomObject(type="tassui")`
// goes through `createObject("component", "customobjects.#type#")`), while
// `returntype="pkg.tassui"` is a contract the engine enforces. Any other
// inferred component is kept, since it may be the more specific of the two.
func (r *Resolver) ReturnComponentOf(fd *parser.FunctionDef) string {
	if r.Index == nil || fd == nil {
		budget := 128

		return r.returnComponentOf(fd, 0, &budget)
	}

	// The generation is read before the answer is computed, so a write made
	// meanwhile moves it past the one the answer is stored under.
	gen := r.Index.Generation()

	var stamp returnEntry

	if fd.URI.IsFile() {
		if info, err := r.fs().Stat(fd.URI.Path()); err == nil {
			stamp.size, stamp.modTime = info.Size(), info.ModTime()
		}
	}

	r.mu.RLock()
	entry, ok := r.returnCache.entries[fd]
	ok = ok && r.returnCache.gen == gen
	r.mu.RUnlock()

	if ok && entry.size == stamp.size && entry.modTime.Equal(stamp.modTime) {
		return entry.component
	}

	budget := 128
	component := r.returnComponentOf(fd, 0, &budget)

	r.mu.Lock()
	if r.returnCache.gen != gen || r.returnCache.entries == nil || len(r.returnCache.entries) >= maxReturnCache {
		r.returnCache = returnCache{gen: gen, entries: make(map[*parser.FunctionDef]returnEntry)}
	}

	r.returnCache.entries[fd] = returnEntry{component: component, size: stamp.size, modTime: stamp.modTime}
	r.mu.Unlock()

	return component
}

func (r *Resolver) returnComponentOf(fd *parser.FunctionDef, depth int, budget *int) string {
	if depth > 8 {
		return ""
	}

	if fd.ReturnComponent == "" || fd.ReturnComponent == "$any" {
		if ret := r.wheelsFixedMapperReturn(fd); ret != "" {
			return ret
		}
	}

	if fd.URI.IsFile() && (fd.ReturnType == "" || strings.EqualFold(fd.ReturnType, "any") || strings.EqualFold(fd.ReturnType, "component") || strings.EqualFold(fd.ReturnType, "object")) {
		if ret := r.sharedGetterReturn(fd); ret != "" {
			return ret
		}

		method := r.producerFor(fd)
		if method != nil && (producerNeedsSpecialization(method, fd) || (fd.ReturnComponent == "" || fd.ReturnComponent == "$any") && len(fd.ReturnSources) == 0 && fd.DocReturn == "") {
			eval := producerEvaluation{resolver: r, fd: fd, baseDir: filepath.Dir(fd.URI.Path()), depth: depth, budget: budget}

			ret := eval.call(fd, "", nil, false).component()
			if ret != "" || producerNeedsSpecialization(method, fd) {
				return ret
			}
		}
	}

	if len(fd.ReturnSources) > 0 {
		switch strings.ToLower(fd.ReturnType) {
		case "", "any", "component", "object":
		default:
			if strings.Contains(fd.ReturnType, ".") {
				if p := r.besideDeclaring(fd, fd.ReturnType); p != "" {
					return p
				}

				return fd.ReturnType
			}

			return r.bareReturnComponent(fd)
		}

		return r.collectionReturnOf(fd, depth, budget)
	}

	switch {
	case fd.ReturnComponent != "" && fd.ReturnComponent != "$any":
		// `return new Expectation( … )` names the component beside the
		// declaring file, as a bare return type does; read from the caller's
		// directory it named nothing, and every expect( x ).toBe() in a
		// spec extending the Wheels test BaseSpec was a missing component.
		if p := r.besideDeclaring(fd, fd.ReturnComponent); p != "" {
			return p
		}

		return fd.ReturnComponent
	case strings.Contains(fd.ReturnType, "."):
		// A relative path is the declaring file's too: ColdBox's AsyncManager
		// returns `new tasks.Future()`, which is system/async/tasks/Future.cfc
		// and nothing from the caller's directory.
		if p := r.besideDeclaring(fd, fd.ReturnType); p != "" {
			return p
		}

		return fd.ReturnType
	}

	if p := r.bareReturnComponent(fd); p != "" {
		return p
	}

	// An inferred $any, with nothing declared, is still the answer; a doc
	// comment is not a contract, so it does not outrank one.
	if fd.ReturnComponent != "" {
		return fd.ReturnComponent
	}

	return r.docReturnComponent(fd)
}

// docReturnComponent is the component fd's doc comment says it returns, when
// its declaration says nothing more — no type, `any` or `component` — and
// only where the documented path names a file. ColdBox documents most return
// types and declares few, and some of its documented paths are wrong
// (execute()'s names coldbox.system.context.RequestContext): taken on trust,
// one of those would be reported as a component that does not exist. The
// bundled stubs read the same comments (cmd/cfstubgen docReturn).
func (r *Resolver) docReturnComponent(fd *parser.FunctionDef) string {
	switch strings.ToLower(fd.ReturnType) {
	case "", "any", "component":
	default:
		return ""
	}

	if fd.DocReturn == "" {
		return ""
	}

	p := r.besideDeclaring(fd, fd.DocReturn)

	if p == "" {
		file := cfpath.FromURI(string(fd.URI))
		if file == "" {
			return ""
		}

		p = r.ComponentPath(fd.DocReturn, filepath.Dir(file))
	}

	return p
}

var interfaceRe = regexp.MustCompile(`(?im)^\s*interface\b|<cfinterface\b`)

// isInterface reports whether the file at path declares an interface.
func (r *Resolver) isInterface(path string) bool {
	if path == "" {
		return false
	}

	data, err := r.fs().ReadFile(path)

	return err == nil && interfaceRe.Match(data)
}

// FuncLookup is the parser's hook for what a method of a component returns,
// resolved from baseDir: ParseOptions.FuncLookup, which types a variable
// assigned from a call on another component.
func (r *Resolver) FuncLookup(baseDir string) func(component, funcName string) string {
	r = r.forCaller(baseDir)

	return func(component, funcName string) string {
		if property, ok := parser.PropertyName(funcName); ok {
			return r.publicPropertyComponent(component, property, baseDir)
		}

		expression, callHop := parser.CallExpression(funcName)
		if callHop {
			funcName = wheelsCallName(expression)
		}

		fd := r.ResolveFunc(component, funcName, baseDir)
		if fd == nil {
			return ""
		}

		if ret := r.ReturnComponentOf(fd); ret != "" {
			if sub := r.selfTyped(component, baseDir, fd, ret); sub != "" {
				return sub
			}

			return ret
		}

		if callHop {
			if ret := r.expressionReturn(fd, expression, baseDir, component); ret != "" {
				return ret
			}
		}

		return r.receiverReturn(r.ComponentPath(component, baseDir), funcName)
	}
}

// cfmlTypes are the return types CFML itself defines. A component of one of
// these names beside the declaring file does not make returntype="query" a
// component: the engine's type wins, as it does at runtime.
var cfmlTypes = map[string]bool{
	"any": true, "array": true, "binary": true, "boolean": true, "component": true,
	"date": true, "datetime": true, "email": true, "function": true, "closure": true,
	"guid": true, "numeric": true, "integer": true, "number": true, "query": true,
	"string": true, "struct": true, "uuid": true, "variablename": true, "void": true,
	"xml": true, "object": true, "regex": true, "url": true, "double": true,
}

// bareReturnComponent is the component a bare-word return type names:
// ColdBox declares `ColdBoxScheduledTask function task( name )`, and CFML
// finds that component beside the declaring one. Only there — the resolver's
// wider search would take any file of the name anywhere in the workspace, and
// a bare word is far more often a type than a component, which is why a
// dotted return type was the only one taken. The answer is the file's path,
// as a method returning `this` is typed.
func (r *Resolver) bareReturnComponent(fd *parser.FunctionDef) string {
	t := strings.TrimSpace(fd.ReturnType)
	if cfmlTypes[strings.ToLower(t)] {
		return ""
	}

	return r.besideDeclaring(fd, t)
}

// besideDeclaring is the file a bare component name t names beside the file
// that declares fd, or "".
func (r *Resolver) besideDeclaring(fd *parser.FunctionDef, t string) string {
	t = strings.TrimSpace(t)
	if t == "" || r.FS == nil || strings.ContainsAny(t, "/\\[]<> $#:") || filepath.IsAbs(t) {
		return ""
	}

	file := cfpath.FromURI(string(fd.URI))
	if file == "" {
		return ""
	}

	return cfpath.ResolvePathCached(t, filepath.Dir(file), nil, r.dirs())
}

// bareArgumentComponent is the component a bare argument type names beside
// the declaring file, as a bare return type is read (besideDeclaring). A CFML
// type name, and a word naming no file there, is not one.
func (r *Resolver) bareArgumentComponent(t string, pr *parser.ParseResult) string {
	if t == "" || cfmlTypes[strings.ToLower(t)] {
		return ""
	}

	return r.besideDeclaring(&parser.FunctionDef{URI: pr.URI}, t)
}

// checkMethodOn is canResolveCall's last step: whether the component the
// receiver resolved to defines the method, with the dynamic, builtin,
// member-method, onMissingMethod and altComp fallbacks.
func (r *Resolver) checkMethodOn(comp, softComp string, call *parser.CallSite, pr *parser.ParseResult, baseDir string, tr *callTrace) string {
	funcName, variable := call.FuncName, call.Variable

	if comp == "" {
		if base, implied := r.inheritedFromMissingBase(variable, call.Line, pr, baseDir); base != "" {
			if implied {
				tr.addf("%q is the framework base's, which does not resolve (%s) — accepted as dynamic", variable, base)
				tr.hit(TargetDynamic, "", nil)

				return ""
			}

			return MissingBaseReason(base)
		}

		return "variable '" + variable + "' has no component ref"
	}

	// A MockBox mock of a class: the class's methods, where the class
	// resolves, and dynamic where it does not — mocking a class of a module
	// the workspace lacks is not a finding.
	if cls, ok := strings.CutPrefix(comp, parser.MockPrefix); ok {
		if r.ComponentPath(cls, baseDir) == "" {
			tr.addf("a mock of %q, which does not resolve — accepted as dynamic", cls)
			tr.hit(TargetDynamic, comp, nil)

			return ""
		}

		tr.addf("a mock of %q — checking the class", cls)
		comp = cls
	}

	// Dynamic return type — method called on a result of a function returning "any".
	if comp == "$any" {
		tr.addf("component is $any (dynamic) — accepted without a method check")
		tr.hit(TargetDynamic, "$any", nil)

		return ""
	}

	// Builtin return type — check method exists in-memory
	if strings.HasPrefix(comp, "$builtin.") {
		builtinName := comp[9:]
		if docs.LookupBuiltinMethod(builtinName, funcName) {
			tr.hit(TargetBuiltin, comp, nil)

			return ""
		}

		return "method '" + funcName + "' not found on builtin " + builtinName
	}

	tr.addf("checking whether %q defines method %q", comp, funcName)

	if def := r.ResolveFunc(comp, funcName, baseDir); def != nil {
		tr.hit(TargetComponent, comp, def)

		return ""
	}

	if mockDecoration(funcName) {
		tr.addf("%q is a method MockBox adds to a mock of %q — accepted", funcName, comp)
		tr.hit(TargetDynamic, comp, nil)

		return ""
	}

	// `a.m = f;` earlier in the function stored something on a, and this
	// calls it: the method is the program's, not the component's.
	if variable != "" && len(call.Chain) == 0 && pr.AssignsMember(variable, funcName, call.Line) {
		tr.addf("%q is assigned onto %q earlier in the function — accepted as dynamic", funcName, variable)
		tr.hit(TargetDynamic, comp, nil)

		return ""
	}

	if parser.IsMemberMethod(funcName) {
		tr.addf("%q is a known member/Java-interop method — accepted without finding it in %q", funcName, comp)
		tr.hit(TargetMember, comp, nil)

		return ""
	}

	// Component defines onMissingMethod — any method call is valid.
	if r.ResolveFunc(comp, "onMissingMethod", baseDir) != nil {
		tr.addf("%q defines onMissingMethod — any method call accepted", comp)
		tr.hit(TargetDynamic, comp, nil)

		return ""
	}

	// An interface declares less than the object behind it: ColdBox types
	// its cache providers as ICacheProvider and calls getOrSet() on them,
	// which every provider has and the interface does not declare. A method
	// the interface does declare was found above.
	if r.isInterface(r.ComponentPath(comp, baseDir)) {
		tr.addf("%q is an interface, and what implements it may have %q — accepted as dynamic", comp, funcName)
		tr.hit(TargetDynamic, comp, nil)

		return ""
	}

	// The ref-derived component didn't have the method. Try the variable-name resolver
	// as a fallback — a pendingCall propagation may have assigned the wrong component
	// (e.g. var objFile = _parent.getFile() inherits _parent's component, but the
	// "objFile" resolver names the real type).
	if altComp, altNoFollow := parser.ResolveFromCallFull(variable, r.Resolvers); altComp != "" && altComp != comp {
		tr.addf("%q not found in %q — trying altComp fallback: componentResolver on variable name gives %q (noFollow=%v)", funcName, comp, altComp, altNoFollow)

		if altNoFollow {
			tr.hit(TargetDynamic, altComp, nil)

			return ""
		}

		if altDef := r.ResolveFunc(altComp, funcName, baseDir); altDef != nil {
			tr.hit(TargetComponent, altComp, altDef)

			return ""
		}
	}

	// A component that names no file is a different finding from a method a
	// real component lacks, and reporting it as the second hid it. It is
	// almost always a componentResolver producing a path — a broad get$1()
	// catch-all turning getInjectorController() into tass.injectorcontroller —
	// so saying so points at the config rather than at the code.
	if !r.componentExists(comp, baseDir) {
		// Unless a resolver marked dynamicIfMissing produced it: that is a
		// broad pattern's guess, and a guess that names nothing says the value
		// is not one of the components it was written for — not that one is
		// missing. A component named any other way is still reported.
		if softMissing(comp, softComp, pr) {
			tr.addf("%q names no file and came from a dynamicIfMissing resolver — accepted as dynamic", comp)
			tr.hit(TargetDynamic, comp, nil)

			return ""
		}

		// Nor a model of a module the workspace does not have: that is the
		// module not being installed.
		if r.uninstalledModule(comp) {
			tr.addf("%q is a module's model and no such module is in the workspace — accepted as dynamic", comp)
			tr.hit(TargetDynamic, comp, nil)

			return ""
		}

		// Nor one the engine provides: `new Query()` is Lucee's or Adobe's
		// own component, whose methods are the engine's to answer for.
		if _, ok := engineComponent(comp); ok {
			tr.addf("%q is a component the engine provides and its source is not in the workspace — accepted as dynamic", comp)
			tr.hit(TargetDynamic, comp, nil)

			return ""
		}

		return "component '" + displayComponent(comp) + "' does not exist (calling '" + funcName + "')"
	}

	if base, implied := r.componentMissingBase(comp, baseDir); base != "" {
		if implied {
			tr.addf("%q's framework base does not resolve (%s) — accepted as dynamic", comp, base)
			tr.hit(TargetDynamic, comp, nil)

			return ""
		}

		return MissingBaseReason(base)
	}

	return "method '" + funcName + "' not found in " + displayComponent(comp)
}

// displayComponent is how a reason names comp. A component named by its file
// — what a function returning `this` returns — is named by the file's name:
// a reason is written into a known-issues file that is committed and read on
// other machines, where an absolute path would not mean anything.
func displayComponent(comp string) string {
	if filepath.IsAbs(comp) {
		return strings.TrimSuffix(filepath.Base(comp), filepath.Ext(comp))
	}

	return comp
}

// ComponentOf reports the component variable holds at line, or "" when that
// is not known or is not a component. It is the answer go-to-type-definition
// wants, and it runs the receiver lookup CanResolveCall runs, so the two cannot
// disagree about what a variable holds.
//
// Two of CanResolveCall's fallbacks are left out on purpose. A
// componentResolver is tried against the variable name, but not against the
// whole line, which exists for chained calls and would answer for whatever
// else the line holds. And "$any" and "$builtin." are not components, so they
// are reported as unknown.
func (r *Resolver) ComponentOf(variable string, line uint32, pr *parser.ParseResult, baseDir string) string {
	caller := parser.FindFuncScopeAt(int(line), pr.Scopes).Name

	comp, _ := r.receiverComponent(variable, line, caller, "", pr, baseDir, nil)

	if comp == "" {
		comp, _, _ = parser.ResolveFromCallMatch(variable, r.Resolvers)
	}

	if comp == "$any" || strings.HasPrefix(comp, "$builtin.") {
		return ""
	}

	return comp
}

func (r *Resolver) recordReceiver(variable, name string, scope parser.RefScope, line uint32, caller string, pr *parser.ParseResult) string {
	if ref := funcScopedRef(pr, line, name, scope); ref != nil {
		return ref.Component
	}

	root, _, _ := strings.Cut(name, ".")

	explicit := strings.HasPrefix(strings.ToLower(variable), "variables.") || strings.HasPrefix(strings.ToLower(variable), "this.")
	if !explicit && argumentOf(pr, caller, root) != nil {
		return ""
	}

	if !explicit {
		if strings.HasPrefix(strings.ToLower(variable), "local.") || strings.HasPrefix(strings.ToLower(variable), "arguments.") {
			return ""
		}

		if fs := parser.FindFuncScopeAt(int(line), pr.Scopes); fs.Start != -1 {
			for _, local := range pr.FuncVars(fs.Start, fs.End) {
				if strings.EqualFold(local, root) {
					return ""
				}
			}
		}
	}

	if ref := fileLevelRef(pr, line, name, scope); ref != nil {
		return ref.Component
	}

	return ""
}

// receiverComponent finds the component a qualified call's receiver holds at
// line: a ref scoped to the enclosing function, the nearest preceding ref in the
// file, an Application.cfc ref, an ARGUMENTS.x type, and refs up the extends
// chain, in that order. It is canResolveCall's lookup, shared with
// ComponentOf so that go-to-type-definition answers exactly as call
// resolution does; the componentResolver fallbacks stay with each caller.
//
// member reports the one case that is not a component at all: an ARGUMENTS.x
// of primitive type calling a known member method, which canResolveCall
// accepts outright. It needs funcName; ComponentOf passes none.
func (r *Resolver) receiverComponent(variable string, line uint32, caller, funcName string, pr *parser.ParseResult, baseDir string, tr *callTrace) (comp string, member bool) {
	if name, scope, record := parser.MemberReceiverName(variable); record {
		return r.recordReceiver(variable, name, scope, line, caller, pr), false
	}
	// Strip scope prefix for matching (VARIABLES.x -> x). Bracket-aware: a "."
	// inside a "[...]" subscript (e.g. "linkMap[arguments.startSource]") is not a
	// scope prefix and must not be stripped there.
	lookupVar := parser.StripReceiverScope(variable)
	scope := parser.ReceiverRefScope(variable)

	// Each lookup below is tried while comp is still empty. A matching ref
	// with no component is logged and passed over, as it always was.

	// Try function-scoped refs first
	if ref := funcScopedRef(pr, line, lookupVar, scope); ref != nil {
		comp = ref.Component

		tr.addf("resolved %q to %q via function-scoped ComponentRef", variable, comp)
	}

	// An argument shadows a same-named component field. In particular, an
	// untyped parameter in a sibling method must not borrow an injected field.
	argumentReceiver := !strings.Contains(variable, ".") || strings.HasPrefix(strings.ToLower(variable), "arguments.")
	if comp == "" && argumentReceiver {
		if arg := argumentOf(pr, caller, lookupVar); arg != nil {
			if arg.Component != "" {
				return arg.Component, false
			}

			if strings.Contains(arg.Type, ".") {
				tr.addf("resolved %q via enclosing function argument", variable)

				return arg.Type, false
			}

			// A bare type is read as a bare return type is: a component
			// beside the declaring file. `required Duration otherDuration`
			// was left untyped, so every call on the argument was unresolved.
			if path := r.bareArgumentComponent(arg.Type, pr); path != "" {
				tr.addf("resolved %q to %q via its declared type %q, a component beside the declaring file", variable, path, arg.Type)

				return path, false
			}

			if parser.IsMemberMethod(funcName) {
				return "$any", true
			}

			tr.addf("argument %q shadows component fields", lookupVar)

			return "", false
		}
	}

	if comp == "" {
		if best := fileLevelRef(pr, line, lookupVar, scope); best != nil {
			comp = best.Component

			tr.addf("resolved %q to %q via file-level ComponentRef (nearest preceding assignment at line %d)", variable, comp, best.Line+1)
		}
	}

	// Fall back to Application.cfc component refs
	if comp == "" {
		for _, hit := range r.applicationRefs(baseDir, lookupVar) {
			comp = hit.ref.Component

			tr.addf("resolved %q to %q via %s ComponentRef", variable, comp, hit.file)

			if comp != "" {
				break
			}
		}
	}

	// ARGUMENTS.x qualifier — resolve directly from the enclosing function's argument list.
	// This handles cases where the argument has a component type (via hint promotion or
	// explicit type) without requiring a ComponentRef to have been created.
	if comp == "" && strings.HasPrefix(strings.ToUpper(variable), "ARGUMENTS.") {
		argName := variable[10:]

		if arg := argumentOf(pr, caller, argName); arg != nil {
			if strings.Contains(arg.Type, ".") {
				comp = arg.Type

				tr.addf("resolved %q to %q via <cfargument type>", variable, comp)
			} else if parser.IsMemberMethod(funcName) {
				// Primitive-typed argument (string/numeric/array/etc.)
				// calling a known member/Java-interop method (e.g.
				// a string argument's .toCharArray()) — no component
				// is needed to verify it.
				tr.addf("ARGUMENTS.%s has primitive type %q, but %q is a known member method — accepted without a component", argName, arg.Type, funcName)

				return "", true
			}
		}
	}

	// Fall back to extends chain component refs (e.g. variables.$assert assigned in a parent)
	if comp == "" && r.fileExtends(pr) != "" {
		tr.addf("no ref found in this file — checking extends chain (%s) for a ComponentRef", r.fileExtends(pr))

		r.walkExtendsRefs(r.fileExtends(pr), baseDir, lookupVar, func(ref *parser.ComponentRef, parent string) bool {
			comp = ref.Component

			tr.addf("resolved %q to %q via ComponentRef in parent %s", variable, comp, parent)

			return comp != ""
		})
	}

	// Last, a shared-scope variable set up by a template the application's
	// Application.cfc includes (see startup.go).
	if comp == "" {
		comp = r.startupComponent(variable, baseDir, tr)
	}

	return comp, false
}

// funcScopedRef is the first ref for name inside the function enclosing line,
// among those scope admits.
func funcScopedRef(pr *parser.ParseResult, line uint32, name string, in parser.RefScope) *parser.ComponentRef {
	for _, scope := range pr.Scopes {
		if int(line) < scope.Start || int(line) > scope.End {
			continue
		}

		refs, _ := pr.FuncRefs(scope.Start, scope.End)

		// A closure holding line may declare the name itself, which shadows
		// the function's: the innermost closure's latest declaration at or
		// before line wins. Failing that, the function's own, and of those the
		// latest at or before line, as fileLevelRef chooses: `var exporter` in
		// one <cfcase> and again in the next is two variables in practice.
		// Only a forward reference takes the first.
		var closure, wide *parser.ComponentRef

		for i := range refs {
			ref := &refs[i]
			if !strings.EqualFold(ref.Variable, name) || !ref.VisibleAt(line) || !in.Admits(ref) {
				continue
			}

			switch {
			case ref.VisibleTo == 0:
				if wide == nil || ref.Line <= line && (wide.Line > line || ref.Line > wide.Line) {
					wide = ref
				}
			case ref.Line <= line && (closure == nil || ref.VisibleFrom > closure.VisibleFrom ||
				ref.VisibleFrom == closure.VisibleFrom && ref.Line > closure.Line):
				closure = ref
			}
		}

		if closure != nil {
			return closure
		}

		return wide
	}

	return nil
}

// fileLevelRef is the file-level ref for name that is in force at line.
//
// A scratch variable can be reassigned multiple times in the same file (e.g.
// once per <cfswitch>/<cfcase> branch) — using the first matching ref in file
// order would lock onto whichever branch happens to appear earliest,
// regardless of which branch the call site is actually in. Prefer the ref
// with the highest line number at or before the call site (the assignment
// that's actually in scope there); only fall back to file order for a genuine
// forward reference, where no preceding ref exists.
func fileLevelRef(pr *parser.ParseResult, line uint32, name string, scope parser.RefScope) *parser.ComponentRef {
	var best *parser.ComponentRef

	for i := range pr.ComponentRefs {
		ref := &pr.ComponentRefs[i]
		if !strings.EqualFold(ref.Variable, name) || ref.Line > line || !scope.Admits(ref) {
			continue
		}

		if best == nil || ref.Line > best.Line {
			best = ref
		}
	}

	if best != nil {
		return best
	}

	for i := range pr.ComponentRefs {
		if strings.EqualFold(pr.ComponentRefs[i].Variable, name) && scope.Admits(&pr.ComponentRefs[i]) {
			return &pr.ComponentRefs[i]
		}
	}

	return nil
}

// appRef is a ref for a name in one of the Application files.
type appRef struct {
	ref  *parser.ComponentRef
	file string
}

// applicationRefs is the first ref for name in the governing Application.cfc
// and then Application.cfm, for each that has one.
func (r *Resolver) applicationRefs(baseDir, name string) []appRef {
	appDir := r.FindApplicationRoot(baseDir)
	if appDir == "" {
		return nil
	}

	var out []appRef

	for _, appName := range []string{"Application.cfc", "Application.cfm"} {
		appURI := cfpath.ToURI(filepath.Join(appDir, appName))
		for _, ref := range r.Index.RefsForFile(appURI) {
			if strings.EqualFold(ref.Variable, name) {
				out = append(out, appRef{ref: ref, file: appName})

				break
			}
		}
	}

	return out
}

// declaresArgument reports whether any function called caller declares an
// argument called name.
func declaresArgument(pr *parser.ParseResult, caller, name string) bool {
	for i := range pr.Funcs {
		f := &pr.Funcs[i]
		if !strings.EqualFold(f.Name, caller) {
			continue
		}

		for _, arg := range f.Arguments {
			if strings.EqualFold(arg.Name, name) {
				return true
			}
		}
	}

	return false
}

// argumentOf is the argument called name of the function called caller, or
// nil. Only the first function with that name is consulted.
func argumentOf(pr *parser.ParseResult, caller, name string) *parser.Argument {
	for i := range pr.Funcs {
		f := &pr.Funcs[i]
		if !strings.EqualFold(f.Name, caller) {
			continue
		}

		for j := range f.Arguments {
			if strings.EqualFold(f.Arguments[j].Name, name) {
				return &f.Arguments[j]
			}
		}

		return nil
	}

	return nil
}

// walkExtendsRefs walks the extends chain from extends, handing visit the
// first ref for name in each ancestor that has one, until visit returns true.
func (r *Resolver) walkExtendsRefs(extends, baseDir, name string, visit func(ref *parser.ComponentRef, parent string) bool) {
	seen := make(map[string]bool)

	for extends != "" && !seen[extends] {
		seen[extends] = true

		cfcPath := r.ComponentPath(extends, baseDir)
		if cfcPath == "" {
			return
		}

		parentURI := cfpath.ToURI(cfcPath)

		// Ensure the parent is indexed so RefsForFile returns its component refs.
		// (EnsureIndexed is a fast no-op if already indexed.)
		r.EnsureIndexed(cfcPath)

		for _, ref := range r.Index.RefsForFile(parentURI) {
			if strings.EqualFold(ref.Variable, name) {
				if visit(ref, extends) {
					return
				}

				break
			}
		}

		// Walk up the extends chain
		data, err := r.fs().ReadFile(cfcPath)
		if err != nil {
			return
		}

		extends = r.extendsFor(parser.Parse(parentURI, string(data)).Extends, cfcPath)
	}
}

// softMissing reports whether comp, which names no file, came from a
// dynamicIfMissing resolver: during this resolution or during the parse.
func softMissing(comp, softComp string, pr *parser.ParseResult) bool {
	return (softComp != "" && strings.EqualFold(comp, softComp)) || pr.IsSoftComponent(comp)
}

// componentExists reports whether component, or any of its pipe-delimited
// alternatives, names a file. A dynamic or builtin marker counts as existing:
// it names no file by design.
func (r *Resolver) componentExists(component, baseDir string) bool {
	for alt := range strings.SplitSeq(component, "|") {
		switch {
		case alt == "":
			continue
		case strings.HasPrefix(alt, "$"):
			return true
		case filepath.IsAbs(alt):
			for _, p := range []string{alt, alt + ".cfc"} {
				if info, err := r.fs().Stat(p); err == nil && !info.IsDir() {
					return true
				}
			}
		}

		if r.ComponentPath(alt, baseDir) != "" {
			return true
		}
	}

	return false
}

// sharedGetterReturn is what a function whose whole body is `return
// application.x` (or another shared scope) returns: what the startup
// templates assign x, the type it already has as a receiver. Mura's
// getPluginManager() and getServiceFactory() are this. Anything else in the
// body withholds it, a call above all, since a call can write the scope:
// TestAbsentOptionalArgumentBoundaries holds the guarded and computed shapes
// to no answer.
func (r *Resolver) sharedGetterReturn(fd *parser.FunctionDef) string {
	method := r.producerFor(fd)
	if method == nil || len(method.body) != 1 || method.body[0].kind != "return" {
		return ""
	}

	path := producerPath(producerUnwrap(producerTokens(method.body[0].expression)))
	if _, shared := splitSharedScope(path); !shared {
		return ""
	}

	dir := filepath.Dir(fd.URI.Path())

	comp := r.startupComponent(path, dir, nil)
	if comp == "" || strings.HasPrefix(comp, "$") {
		return ""
	}

	if p := r.ComponentPath(comp, dir); p != "" {
		return p
	}

	return comp
}
