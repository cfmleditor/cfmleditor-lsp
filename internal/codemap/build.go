package codemap

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cfmleditor/cfmleditor-lsp/internal/docs"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"github.com/cfmleditor/cfmleditor-lsp/internal/resolve"
	"github.com/cfmleditor/cfmleditor-lsp/internal/route"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

// Phase names the stage a build is in, for progress reporting.
type Phase string

// The phases of a build.
const (
	// PhaseIndex is the first pass: every .cfc's signatures into the index, so the
	// second pass has something to resolve calls against.
	PhaseIndex Phase = "index"

	// PhaseScan is the second pass: parse each file and emit its edges.
	PhaseScan Phase = "scan"
)

// FileConfig is the resolution environment one file is read in.
//
// It is per file, not per build, because a workspace is routinely several
// applications side by side and each keeps its own .cfmleditor.json. Applying one
// application's componentResolvers to another's source does not fail loudly — it
// simply resolves nothing, and those files come out of the map with no edges at
// all. In one real workspace that was 11,387 of 11,682 functions in the sibling
// applications showing no caller, which reads as dead code rather than as the map
// having been built with the wrong resolvers.
//
// The Index inside Resolver is normally shared across every FileConfig: function
// signatures are a property of the workspace, not of whose config you read them
// under. Only the resolver chain and the mappings differ.
type FileConfig struct {
	Resolver                 *resolve.Resolver
	Resolvers                []parser.Resolver
	ExpressionMappings       map[string]string
	ServicePropertyResolvers map[string]string

	// Routes, when set, resolves the framework routes found in this file's
	// source. Nil leaves route edges out entirely.
	Routes *route.Resolver

	// InterpolateAllText is parser.ParseOptions.InterpolateAllText: the
	// features.outputContextInterpolation switch turned off.
	InterpolateAllText bool
}

// Options configures a build.
type Options struct {
	// Root is the directory node paths are reported relative to. Files outside it
	// still appear, addressed by a ../ path.
	Root string

	// Files are the absolute paths to scan. Only .cfc files are indexed in the
	// first pass; every CFML file is scanned in the second.
	Files []string

	FS       vfs.FS
	Resolver *resolve.Resolver

	Resolvers                []parser.Resolver
	ExpressionMappings       map[string]string
	ServicePropertyResolvers map[string]string

	// Workers bounds the scan's parallelism. Zero means GOMAXPROCS.
	Workers int

	// IncludeUnresolved keeps edges to calls that resolved to nothing, pointing at
	// a KindExternal node. Off by default: on a codebase without thorough resolver
	// config these outnumber the real edges and drown the map. The Stats count them
	// either way, so turning this off hides them from the picture, not from you.
	IncludeUnresolved bool

	// IncludeBuiltins keeps calls to built-in CFML functions. Almost never wanted —
	// every file calls len() and structNew(), and the edges say nothing about the
	// project's own structure.
	IncludeBuiltins bool

	// EntryGlobs marks files as entry points by path, in addition to the ones the
	// map works out for itself (.cfm pages, remote methods, Application.cfc hooks).
	//
	// It exists because plenty of real code is invoked by something the codebase
	// never names. A release-script directory whose files a runner loads by
	// constructing their name, a scheduled-task folder, a plugin directory: those
	// are reached at runtime and no static analysis can see it. In one workspace
	// 10,639 of 11,374 apparently-unreferenced functions were release scripts of
	// exactly this kind — correctly unreferenced, and wrong to read as dead code.
	//
	// Patterns are matched with path.Match against the node's slash-separated path
	// relative to Root, and against each path segment prefix, so "prs/*" and
	// "../prs/*" both work the way a reader expects.
	EntryGlobs []string

	// UtilityGlobs marks files as infrastructure rather than application code —
	// the logging, the PDF writer, the context accessor every request touches.
	//
	// Marked, not excluded. These are the most-depended-on code in a workspace and
	// removing them would misreport what depends on what; leaving them
	// unmarked makes every ranking a list of them and ties every part of the graph
	// to every other. The reader decides, and the label is what lets them.
	//
	// Matched exactly as EntryGlobs are.
	UtilityGlobs []string

	// ConfigExtra is mixed into the cache fingerprint. A caller that resolves files
	// under several .cfmleditor.json files puts their contents here: the resolver
	// chain decides what a call site resolves to, so a cached edge set computed
	// under one set of resolvers must not be served after they change. Leaving it
	// empty is right only when the configuration cannot vary.
	ConfigExtra string

	// ConfigFor returns the resolution environment for one file. When nil, every
	// file is read under the single Resolver/Resolvers/… set above, which is right
	// only when the whole scan is governed by one .cfmleditor.json.
	//
	// It is called once per file from several goroutines and must be safe for that.
	ConfigFor func(file string) FileConfig

	// Cache, if set, lets an unchanged file skip parsing entirely. See [Cache] for
	// why it is keyed by a workspace fingerprint as well as by file content.
	Cache Cache

	// Progress, if set, is called as the build advances. It must be safe to call
	// from several goroutines.
	Progress func(phase Phase, done, total int)
}

// Cache stores what one file contributed, so a rebuild can skip re-parsing it.
//
// The two-part key is the whole design. A file's own content hash is not enough:
// resolving a call site reads the index built from every other .cfc, so moving a
// method in file B changes what file A's unchanged text resolves to. Keying on
// content alone would serve a stale edge set and the map would quietly disagree
// with the code.
//
// The fingerprint covers every indexed .cfc, so editing any component invalidates
// every cached entry and the build is a full one — correct, and no worse than
// having no cache. Editing .cfm pages, which is most editing, leaves the
// fingerprint alone and the cache serves nearly everything. That asymmetry is the
// honest shape of the problem rather than a shortcut around it.
//
// Both methods must be safe to call from several goroutines.
type Cache interface {
	Load(fingerprint, contentHash string) (*FileGraph, bool)
	Save(fingerprint, contentHash string, g *FileGraph) error
}

// cfcLifecycle is the set of Application.cfc methods the engine calls rather than
// the codebase — the roots that make an Application.cfc reachable at all. Without
// them a perfectly live application looks entirely orphaned.
var cfcLifecycle = map[string]bool{
	"onapplicationstart": true, "onapplicationend": true,
	"onsessionstart": true, "onsessionend": true,
	"onrequeststart": true, "onrequest": true, "onrequestend": true,
	"onerror": true, "onmissingtemplate": true, "onabort": true,
	"oncfcrequest": true, "ondeactivate": true,
}

// Build makes a whole-project map.
//
// Two passes, for the reason `unresolved` makes two: resolving a call needs every
// component's signatures already indexed, and a single pass would resolve calls
// against whatever happened to be indexed by then — an answer that changes with
// file order.
//
// The scan streams. A parsed file's ParseResult is turned into edges and dropped
// before the next one is read, so peak memory is the worker count, not the
// workspace: holding a few thousand ExtractCalls parse trees at once is gigabytes,
// and holding eight is nothing.
func Build(opts *Options) *Map {
	// A copy: the defaults below are filled into it, and must not reach the
	// caller's options.
	o := *opts
	opts = &o

	start := time.Now()

	if opts.Workers <= 0 {
		opts.Workers = runtime.GOMAXPROCS(0)
	}

	root := opts.Root
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}

	if opts.ConfigFor == nil {
		single := FileConfig{
			Resolver:                 opts.Resolver,
			Resolvers:                opts.Resolvers,
			ExpressionMappings:       opts.ExpressionMappings,
			ServicePropertyResolvers: opts.ServicePropertyResolvers,
		}
		opts.ConfigFor = func(string) FileConfig { return single }
	}

	fingerprint := indexPass(opts)

	m := &Map{Root: root, Level: LevelFunction, Fingerprint: fingerprint}
	c := newCollector(m)

	results := make(chan *FileGraph, opts.Workers*2)

	var wg sync.WaitGroup

	jobs := make(chan string)

	for range opts.Workers {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for f := range jobs {
				results <- scanFile(opts, root, f, fingerprint)
			}
		}()
	}

	go func() {
		defer close(jobs)

		for _, f := range opts.Files {
			jobs <- f
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	done := 0
	total := len(opts.Files)

	for r := range results {
		done++

		if opts.Progress != nil {
			opts.Progress(PhaseScan, done, total)
		}

		c.merge(r)
	}

	c.finish()

	m.Stats.Files = len(opts.Files)
	m.Stats.Functions = countKind(m.Nodes, KindFunction)
	m.Stats.Utility = countUtility(m.Nodes)
	m.Stats.BuildMillis = time.Since(start).Milliseconds()

	// Islands, reachability and roots are properties of the finished graph, not of
	// any one file, so they can only be filled in once every file has been merged.
	m.Annotate()

	return m
}

// indexPass fills the resolver's index with every .cfc's signatures, and returns a
// fingerprint of everything it indexed.
//
// The fingerprint is what a [Cache] keys on alongside each file's own hash: it
// identifies the resolution environment a cached edge set was computed against.
// It is derived from the paths and content hashes of the indexed components in
// sorted order, so it is stable across runs and across the parallel scan that
// follows.
func indexPass(opts *Options) string {
	var cfcs []string

	for _, f := range opts.Files {
		if cfpath.IsCFCFile(f) {
			cfcs = append(cfcs, f)
		}
	}

	sort.Strings(cfcs)

	fp := sha256.New()

	// hash.Hash.Write never returns an error, so neither can a Fprintf to one.
	_, _ = fmt.Fprintf(fp, "config\x00%s\n", opts.ConfigExtra)

	for i, f := range cfcs {
		data, err := opts.FS.ReadFile(f)
		if err != nil || cfpath.IsBinary(data) {
			continue
		}

		opts.Resolver.Index.IndexFile(cfpath.ToURI(f), string(data))

		sum := sha256.Sum256(data)
		// hash.Hash.Write never returns an error, which is why Fprintf to one
		// cannot fail.
		_, _ = fmt.Fprintf(fp, "%s\x00%x\n", f, sum)

		if opts.Progress != nil {
			opts.Progress(PhaseIndex, i+1, len(cfcs))
		}
	}

	return hex.EncodeToString(fp.Sum(nil))
}

// FileGraph is one file's contribution, ready to merge. Nodes marked provisional
// are placeholders for something another file owns — a call into a file this scan
// has not reached yet, or one outside the scanned set entirely.
//
// It is exported, with exported fields, because it is the unit a [Cache] stores:
// the whole point of caching is to skip re-parsing an unchanged file, and that
// means the thing parsing produced has to survive a round trip through JSON.
type FileGraph struct {
	Nodes       []Node `json:"nodes"`
	Provisional []Node `json:"provisional"`
	Edges       []Edge `json:"edges"`
	Stats       Stats  `json:"stats"`
}

func scanFile(opts *Options, root, file, fingerprint string) *FileGraph {
	res := &FileGraph{}

	data, err := opts.FS.ReadFile(file)
	if err != nil || cfpath.IsBinary(data) {
		res.Stats.Unreadable = 1

		return res
	}

	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])

	if opts.Cache != nil {
		if cached, ok := opts.Cache.Load(fingerprint, hash); ok {
			cached.Stats.Cached = 1

			return cached
		}
	}

	content := string(data)
	fileURI := cfpath.ToURI(file)
	baseDir := filepath.Dir(file)
	rel := relPath(root, file)
	isCFC := cfpath.IsCFCFile(file)
	cfg := opts.ConfigFor(file)

	funcLookup := func(component, funcName string) string {
		fd := cfg.Resolver.ResolveFunc(component, funcName, baseDir)
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

	pr := parser.ParseWithOptions(fileURI, content, &parser.ParseOptions{
		Resolvers:                cfg.Resolvers,
		ExpressionMappings:       cfg.ExpressionMappings,
		ServicePropertyResolvers: cfg.ServicePropertyResolvers,
		ExtractCalls:             true,
		ScanAllScopes:            true,
		ExtractLinks:             true,
		InterpolateAllText:       cfg.InterpolateAllText,
		FuncLookup:               funcLookup,
		BuiltinReturnLookup:      docs.LookupBuiltinReturnComponent,
	})
	pr.FuncLookup = funcLookup

	base := filepath.Base(file)
	isApplication := strings.EqualFold(base, "Application.cfc") || strings.EqualFold(base, "Application.cfm")

	// A .cfm is an entry point by definition: something outside the codebase asks
	// for it by URL. A .cfc is not — it is reached through one, with Application.cfc
	// the exception: the engine loads it per request without anything in the
	// codebase naming it, so treating it like any other component reported every
	// application's own front door as unreferenced code.
	utility := MatchesEntryGlob(opts.UtilityGlobs, rel)

	res.Nodes = append(res.Nodes, Node{
		ID:      FileID(rel),
		Kind:    KindFile,
		Name:    base,
		File:    rel,
		Entry:   !isCFC || isApplication || MatchesEntryGlob(opts.EntryGlobs, rel),
		Utility: utility,
	})

	// A file named by an entry glob has its public methods marked too: a runner
	// that loads a component by a constructed name calls into it by a constructed
	// method name just as often, so marking only the file would leave everything
	// inside it looking unreachable.
	res.addFunctions(pr, rel, isApplication, MatchesEntryGlob(opts.EntryGlobs, rel), utility)
	res.addExtends(cfg, pr, rel, baseDir, root)
	res.addRefs(cfg, pr, rel, baseDir, root)
	res.addIncludes(opts, cfg, pr, rel, baseDir, root)
	res.addCalls(opts, cfg, pr, rel, baseDir, root)
	res.addRoutes(cfg, rel, root, content)

	if opts.Cache != nil {
		// A cache write that fails is a slow next build, not a wrong one, so it is
		// not worth failing the scan over or serialising a mutex around.
		_ = opts.Cache.Save(fingerprint, hash, res)
	}

	return res
}

func (res *FileGraph) addFunctions(pr *parser.ParseResult, rel string, isApplication, entryFile, utility bool) {
	access := make(map[string]string, len(pr.Scopes))
	for _, sc := range pr.Scopes {
		access[strings.ToLower(sc.Name)] = sc.Access
	}

	for i := range pr.Funcs {
		f := &pr.Funcs[i]
		lower := strings.ToLower(f.Name)
		acc := access[lower]

		// Reachable from outside the codebase: exposed as a remote method, or an
		// Application.cfc method the engine itself invokes.
		entry := strings.EqualFold(acc, "remote") ||
			(isApplication && cfcLifecycle[lower]) ||
			(entryFile && !strings.EqualFold(acc, "private"))

		res.Nodes = append(res.Nodes, Node{
			ID:      FuncID(rel, f.Name),
			Kind:    KindFunction,
			Name:    f.Name,
			File:    rel,
			Line:    f.Line,
			Access:  acc,
			Entry:   entry,
			Utility: utility,
		})

		res.Edges = append(res.Edges, Edge{
			From:  FileID(rel),
			To:    FuncID(rel, f.Name),
			Kind:  EdgeContains,
			Count: 1,
		})
	}
}

func (res *FileGraph) addExtends(cfg FileConfig, pr *parser.ParseResult, rel, baseDir, root string) {
	if pr.Extends == "" {
		return
	}

	to, ok := res.targetFile(cfg, pr.Extends, baseDir, root)
	if !ok {
		return
	}

	res.Edges = append(res.Edges, Edge{From: FileID(rel), To: to, Kind: EdgeExtends, Count: 1})
}

func (res *FileGraph) addRefs(cfg FileConfig, pr *parser.ParseResult, rel, baseDir, root string) {
	for i := range pr.ComponentRefs {
		ref := &pr.ComponentRefs[i]
		if ref.Component == "" || ref.Component == "$any" {
			continue
		}

		to, ok := res.targetFile(cfg, ref.Component, baseDir, root)
		if !ok {
			continue
		}

		res.Edges = append(res.Edges, Edge{
			From:  res.callerID(pr, rel, ref.Line),
			To:    to,
			Kind:  EdgeInstantiates,
			Count: 1,
		})
	}
}

// addRoutes resolves the framework routes written in this file and joins the file
// to what a dispatcher will reach through them.
//
// The edge is from the *file*, not from whichever function the attribute sits in.
// A route in a page's markup is reached when the page is rendered, and attributing
// it to the enclosing function would claim the function invokes it, which is a
// different and usually false statement.
//
// Every target is emitted, not just the first. A view shared between products
// resolves into whichever product serves the page and that cannot be known
// statically, so picking one would be a guess presented as a fact; all of them
// marked Dynamic is the honest shape. On one workspace 497 of 523 resolvable
// routes had exactly one target anyway.
func (res *FileGraph) addRoutes(cfg FileConfig, rel, root, content string) {
	if cfg.Routes == nil || !cfg.Routes.Config.Enabled() {
		return
	}

	for _, ref := range route.Scan(content, &cfg.Routes.Config) {
		if !route.Plausible(ref.Value) {
			continue
		}

		res.Stats.Routes++

		targets := cfg.Routes.Resolve(ref.Value)
		if len(targets) == 0 {
			res.Stats.RoutesUnresolved++

			continue
		}

		for i := range targets {
			t := &targets[i]

			toRel := relPath(root, t.Path)

			to := FileID(toRel)
			if t.Kind == route.KindController && t.Method != "" {
				to = FuncID(toRel, t.Method)
			}

			res.Provisional = append(res.Provisional, Node{
				ID: to, Kind: nodeKindFor(t), Name: routeNodeName(t, toRel),
				File: toRel, Component: t.Component,
			})

			res.Edges = append(res.Edges, Edge{
				From: FileID(rel), To: to, Kind: EdgeRoute, Count: 1,
				Dynamic: len(targets) > 1,
			})
		}
	}
}

func nodeKindFor(t *route.Target) NodeKind {
	if t.Kind == route.KindController && t.Method != "" {
		return KindFunction
	}

	return KindFile
}

func routeNodeName(t *route.Target, rel string) string {
	if t.Kind == route.KindController && t.Method != "" {
		return t.Method
	}

	return filepath.Base(rel)
}

// addIncludes emits an edge only for a link that resolves to a real CFML file.
// Links carry hrefs and src attributes too, and turning every one of those into a
// node would bury the code map under the site's static assets and outbound URLs.
func (res *FileGraph) addIncludes(opts *Options, cfg FileConfig, pr *parser.ParseResult, rel, baseDir, root string) {
	for i := range pr.Links {
		link := &pr.Links[i]

		target := resolveInclude(opts, cfg, link.Path, baseDir)
		if target == "" {
			continue
		}

		toRel := relPath(root, target)

		res.Provisional = append(res.Provisional, Node{
			ID: FileID(toRel), Kind: KindFile, Name: filepath.Base(target), File: toRel,
		})
		res.Edges = append(res.Edges, Edge{
			From: FileID(rel), To: FileID(toRel), Kind: EdgeIncludes, Count: 1,
		})
	}
}

func (res *FileGraph) addCalls(opts *Options, cfg FileConfig, pr *parser.ParseResult, rel, baseDir, root string) {
	calls := pr.AllCalls()

	for i := range calls {
		call := &calls[i]

		res.Stats.CallSites++

		target, reason := cfg.Resolver.ResolveCallTarget(call, pr, baseDir)
		from := res.callerID(pr, rel, call.Line)

		// The builtin check runs *after* resolution, not before. A workspace
		// function may share a name with a built-in or a member method — init,
		// add, close, get, isValid and about sixty others do in one real codebase
		// — and testing the name first threw away every call to them before
		// anything looked for a definition. A call that resolved to a real
		// definition is a call to that definition, whatever else shares its name.
		if !target.Kind.Definite() && isBuiltin(call.FuncName) && call.Variable == "" {
			res.Stats.Builtin++

			if !opts.IncludeBuiltins {
				continue
			}
		}

		switch {
		case target.Kind.Definite() && target.URI != "":
			res.Stats.Resolved++
			res.addCallEdge(from, funcNodeFor(root, target), false)

		case reason == "":
			res.Stats.Dynamic++

			to, ok := res.targetFile(cfg, target.Component, baseDir, root)
			if !ok {
				if !opts.IncludeUnresolved {
					continue
				}

				to = ExternalID(externalLabel(target.Component, call))
				res.Provisional = append(res.Provisional, Node{
					ID: to, Kind: KindExternal, Name: externalLabel(target.Component, call),
				})
			}

			res.addCallEdge(from, to, true)

		default:
			res.Stats.Unresolved++

			if !opts.IncludeUnresolved {
				continue
			}

			to := ExternalID(externalLabel("", call))
			res.Provisional = append(res.Provisional, Node{
				ID: to, Kind: KindExternal, Name: externalLabel("", call),
			})
			res.addCallEdge(from, to, true)
		}
	}
}

func (res *FileGraph) addCallEdge(from, to string, dynamic bool) {
	if from == to {
		return
	}

	res.Edges = append(res.Edges, Edge{From: from, To: to, Kind: EdgeCalls, Count: 1, Dynamic: dynamic})
}

// callerID names the node a line belongs to: the enclosing function, or the file
// itself for top-level code. Attributing top-level code to the file rather than
// dropping it is what keeps .cfm pages in the graph at all — a page is almost
// entirely top-level code, and it is where every request starts.
func (res *FileGraph) callerID(pr *parser.ParseResult, rel string, line uint32) string {
	for _, sc := range pr.Scopes {
		if int(line) >= sc.Start && int(line) <= sc.End {
			return FuncID(rel, sc.Name)
		}
	}

	return FileID(rel)
}

// targetFile turns a component dot-path into a file node id, recording a
// provisional node for it. It reports false when the path names no file on disk.
func (res *FileGraph) targetFile(cfg FileConfig, component, baseDir, root string) (string, bool) {
	if component == "" || component == "$any" || strings.HasPrefix(component, "$builtin.") {
		return "", false
	}

	resolved := cfg.Resolver.ComponentPath(component, baseDir)
	if resolved == "" {
		return "", false
	}

	rel := relPath(root, resolved)

	res.Provisional = append(res.Provisional, Node{
		ID:        FileID(rel),
		Kind:      KindFile,
		Name:      filepath.Base(resolved),
		File:      rel,
		Component: component,
	})

	return FileID(rel), true
}

func funcNodeFor(root string, target resolve.CallTarget) string {
	return FuncID(relPath(root, cfpath.FromURI(string(target.URI))), target.FuncName)
}

func externalLabel(component string, call *parser.CallSite) string {
	if component != "" && component != "$any" {
		return component + "." + call.FuncName
	}

	if call.Variable != "" {
		return call.Variable + "." + call.FuncName
	}

	return call.FuncName
}

// resolveInclude turns a cfinclude-style path into an absolute file, or "" when it
// does not name a CFML file that exists.
func resolveInclude(opts *Options, cfg FileConfig, raw, baseDir string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.Contains(raw, "#") || strings.Contains(raw, "://") {
		return ""
	}

	if !cfpath.IsCFMLFile(raw) {
		return ""
	}

	candidates := []string{filepath.Join(baseDir, filepath.FromSlash(raw))}
	for _, wf := range cfg.Resolver.WorkspaceFolders {
		candidates = append(candidates, filepath.Join(wf, filepath.FromSlash(strings.TrimPrefix(raw, "/"))))
	}

	for _, c := range candidates {
		if info, err := opts.FS.Stat(c); err == nil && !info.IsDir() {
			abs, err := filepath.Abs(c)
			if err != nil {
				return c
			}

			return abs
		}
	}

	return ""
}

func isBuiltin(name string) bool {
	if _, ok := docs.LookupFunction(name); ok {
		return true
	}

	return parser.IsMemberMethod(name)
}

// relPath reports file relative to root, slash-separated so a node id is the same
// string on every platform. A file outside root keeps its ../ prefix rather than
// being dropped — it is still a real file and still part of the graph.
func relPath(root, file string) string {
	rel, err := filepath.Rel(root, file)
	if err != nil {
		return filepath.ToSlash(file)
	}

	return filepath.ToSlash(rel)
}

// MatchesEntryGlob reports whether rel is named by one of the patterns.
//
// Each pattern is tried against the whole path and against every leading portion
// of it, so "../prs" marks everything beneath it without the caller having to
// write "../prs/**" — path.Match has no "**", and requiring a trailing "/*" per
// directory level is a footgun rather than a feature.
func MatchesEntryGlob(globs []string, rel string) bool {
	if len(globs) == 0 {
		return false
	}

	for _, g := range globs {
		if ok, err := path.Match(g, rel); err == nil && ok {
			return true
		}

		// Directory prefix: "../prs" matches "../prs/a/b.cfc".
		if strings.HasPrefix(rel, strings.TrimSuffix(g, "/")+"/") {
			return true
		}

		for dir := path.Dir(rel); dir != "." && dir != "/"; dir = path.Dir(dir) {
			if ok, err := path.Match(g, dir); err == nil && ok {
				return true
			}
		}
	}

	return false
}

func countUtility(nodes []Node) int {
	n := 0

	for i := range nodes {
		if nodes[i].Utility {
			n++
		}
	}

	return n
}

func countKind(nodes []Node, kind NodeKind) int {
	n := 0

	for i := range nodes {
		if nodes[i].Kind == kind {
			n++
		}
	}

	return n
}

// collector merges per-file results into one map, deduplicating nodes by id and
// edges by (from, to, kind).
type collector struct {
	m           *Map
	nodes       map[string]Node
	provisional map[string]bool
	edges       map[edgeKey]*Edge
}

type edgeKey struct {
	from, to string
	kind     EdgeKind
}

func newCollector(m *Map) *collector {
	return &collector{
		m:           m,
		nodes:       make(map[string]Node),
		provisional: make(map[string]bool),
		edges:       make(map[edgeKey]*Edge),
	}
}

func (c *collector) merge(r *FileGraph) {
	for i := range r.Nodes {
		n := &r.Nodes[i]

		c.nodes[n.ID] = *n
		delete(c.provisional, n.ID)
	}

	// A provisional node never displaces a real one: the file that owns a node
	// knows its line, access and entry status, and a caller's placeholder knows
	// none of that. Whichever arrives first, the owner's wins.
	for i := range r.Provisional {
		n := &r.Provisional[i]

		if _, ok := c.nodes[n.ID]; ok && !c.provisional[n.ID] {
			continue
		}

		c.nodes[n.ID] = *n
		c.provisional[n.ID] = true
	}

	for _, e := range r.Edges {
		k := edgeKey{e.From, e.To, e.Kind}

		existing, ok := c.edges[k]
		if !ok {
			cp := e
			c.edges[k] = &cp

			continue
		}

		existing.Count += e.Count

		// One confident edge is enough to stop calling the relationship a guess.
		if !e.Dynamic {
			existing.Dynamic = false
		}
	}

	c.m.Stats.CallSites += r.Stats.CallSites
	c.m.Stats.Resolved += r.Stats.Resolved
	c.m.Stats.Dynamic += r.Stats.Dynamic
	c.m.Stats.Unresolved += r.Stats.Unresolved
	c.m.Stats.Builtin += r.Stats.Builtin
	c.m.Stats.Unreadable += r.Stats.Unreadable
	c.m.Stats.Cached += r.Stats.Cached
	c.m.Stats.Routes += r.Stats.Routes
	c.m.Stats.RoutesUnresolved += r.Stats.RoutesUnresolved
}

// finish sorts everything. A map that came out of a parallel scan is in whatever
// order the workers finished in, which differs between runs — and a map you cannot
// diff against yesterday's is most of the value gone.
func (c *collector) finish() {
	c.m.Nodes = slices.AppendSeq(make([]Node, 0, len(c.nodes)), maps.Values(c.nodes))

	sort.Slice(c.m.Nodes, func(i, j int) bool { return c.m.Nodes[i].ID < c.m.Nodes[j].ID })

	c.m.Edges = make([]Edge, 0, len(c.edges))

	for _, e := range c.edges {
		// An edge whose endpoint nothing declared points at a node that was never
		// created; drop it rather than emit a dangling reference a renderer has to
		// guess about.
		if _, ok := c.nodes[e.From]; !ok {
			continue
		}

		if _, ok := c.nodes[e.To]; !ok {
			continue
		}

		c.m.Edges = append(c.m.Edges, *e)
	}

	sort.Slice(c.m.Edges, func(i, j int) bool {
		a, b := c.m.Edges[i], c.m.Edges[j]
		if a.From != b.From {
			return a.From < b.From
		}

		if a.To != b.To {
			return a.To < b.To
		}

		return a.Kind < b.Kind
	})
}
