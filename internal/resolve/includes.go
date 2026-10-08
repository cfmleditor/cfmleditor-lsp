package resolve

import (
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"go.lsp.dev/uri"
)

// includeGraph is every static cfinclude in the index, resolved to files, in
// both directions. Keys and values are pathKey spellings; paths holds each
// key's real spelling, since the filesystem and the index need that one.
type includeGraph struct {
	gen   uint64
	fwd   map[string][]string // includer -> files it includes
	rev   map[string][]string // included file -> files that include it
	paths map[string]string

	// scopes caches includeScope per starting file for this generation. The
	// graph is shared by every caller of the resolver, so it has a lock.
	mu     sync.Mutex
	scopes map[string][]string
}

// pathKey is the spelling a file is compared under: cleaned and lowercased,
// because CFML paths are case-insensitive wherever a TASS-style workspace runs.
func pathKey(p string) string {
	return strings.ToLower(filepath.Clean(p))
}

// IncludePath resolves a cfinclude template path written in fromFile to a file
// that exists, or "" when it names none. The order is go-to-definition's: the
// including file's directory, the Application.cfc root, a mapping named by the
// first segment, then each workspace folder, then a workspace folder named
// by the first segment.
func (r *Resolver) IncludePath(raw, fromFile string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.Contains(raw, "#") || strings.Contains(raw, "://") {
		return ""
	}

	baseDir := filepath.Dir(fromFile)
	key := raw + "\t" + baseDir

	r.mu.RLock()
	p, ok := r.includeCache[key]
	r.mu.RUnlock()

	if ok {
		return p
	}

	p = r.includePathUncached(raw, baseDir)

	r.mu.Lock()
	if r.includeCache == nil {
		r.includeCache = map[string]string{}
	}

	r.includeCache[key] = p
	r.mu.Unlock()

	return p
}

// includePathUncached stats each candidate in turn. It depends only on the
// path and the including file's directory, and a scan asks it for the same
// include once per lookup through it, which made it a quarter of the CPU of
// an unresolved scan; IncludePath memoises it beside resolveCache, and
// InvalidatePaths drops both.
func (r *Resolver) includePathUncached(raw, baseDir string) string {
	rel := filepath.FromSlash(raw)

	candidates := []string{filepath.Join(baseDir, rel)}

	if appDir := r.FindApplicationRoot(baseDir); appDir != "" {
		candidates = append(candidates, filepath.Join(appDir, rel))
	}

	trimmed := strings.TrimPrefix(raw, "/")
	if seg, rest, ok := strings.Cut(trimmed, "/"); ok && seg != "" {
		for key, dir := range r.EffectiveMappings(baseDir) {
			if strings.EqualFold(strings.Trim(key, "/"), seg) {
				candidates = append(candidates, filepath.Join(dir, filepath.FromSlash(rest)))
			}
		}

		// A first segment naming the package a box.json above says this is,
		// as slugRoot reads a dot-path: coldbox-platform includes
		// "/coldbox/system/exceptions/BugReport-Public.cfm" from its own
		// system/Bootstrap.cfc, a mapping only an installed copy has.
		if root, _ := r.slugRoot(seg+".x", baseDir); root != "" {
			candidates = append(candidates, filepath.Join(root, filepath.FromSlash(rest)))
		}
	}

	for _, root := range r.WorkspaceFolders {
		candidates = append(candidates, filepath.Join(root, filepath.FromSlash(trimmed)))
	}

	// Last, a first segment naming a workspace folder: see inFolderNamed.
	candidates = append(candidates, cfpath.InFolderNamed(r.WorkspaceFolders, trimmed)...)

	for _, c := range candidates {
		if info, err := r.fs().Stat(c); err == nil && !info.IsDir() {
			return c
		}
	}

	return ""
}

// includeTargets is the files raw names from fromFile: IncludePath's one, or
// for a directory listing's glob (parser.directoryIncludes, `sub/*.cfm`) every
// template in that directory beside fromFile, in name order.
func (r *Resolver) includeTargets(raw, fromFile string) []string {
	dir, pattern, glob := strings.Cut(raw, "/*")
	if !glob {
		if target := r.IncludePath(raw, fromFile); target != "" {
			return []string{target}
		}

		return nil
	}

	if pattern != ".cfm" || strings.ContainsAny(dir, "*#") {
		return nil
	}

	listed := filepath.Join(filepath.Dir(fromFile), filepath.FromSlash(dir))

	entries, err := r.fs().ReadDir(listed)
	if err != nil {
		return nil
	}

	var out []string

	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".cfm") {
			out = append(out, filepath.Join(listed, entry.Name()))
		}
	}

	slices.Sort(out)

	return out
}

// includes returns the include graph for the index as it now stands, building
// it again only when the index's include generation has moved.
func (r *Resolver) includes() *includeGraph {
	if r.Index == nil {
		return nil
	}

	gen := r.Index.IncludeGeneration()

	r.mu.RLock()
	g := r.incGraph
	r.mu.RUnlock()

	if g != nil && g.gen == gen {
		return g
	}

	type entry struct {
		file  string
		paths []string
	}

	var entries []entry

	gen = r.Index.ForEachInclude(func(fileURI uri.URI, paths []string) {
		entries = append(entries, entry{file: cfpath.FromURI(string(fileURI)), paths: paths})
	})

	g = &includeGraph{
		gen:    gen,
		fwd:    make(map[string][]string, len(entries)),
		rev:    make(map[string][]string),
		paths:  make(map[string]string),
		scopes: make(map[string][]string),
	}

	for _, e := range entries {
		from := pathKey(e.file)
		g.paths[from] = e.file

		for _, raw := range e.paths {
			for _, target := range r.includeTargets(raw, e.file) {
				to := pathKey(target)
				g.paths[to] = target
				g.fwd[from] = append(g.fwd[from], to)
				g.rev[to] = append(g.rev[to], from)
			}
		}
	}

	r.mu.Lock()
	r.incGraph = g
	r.mu.Unlock()

	return g
}

// includeScope lists the files whose functions a bare call written in file can
// reach through cfinclude, file itself excluded: every file that includes it,
// transitively, and everything each of those — and file — includes,
// transitively. An included file runs in its includer's variables scope, so a
// function any of them declares is callable unqualified from all of them; a
// template included into api.cfc beside forty others can call what api.cfc
// declares and what any sibling declares.
//
// A file several components include resolves against all of them. That is
// lenient where only one includer is in play at runtime, and it is the only
// static answer: which includer ran is not in the source.
func (g *includeGraph) includeScope(file string) []string {
	start := pathKey(file)

	g.mu.Lock()
	s, ok := g.scopes[start]
	g.mu.Unlock()

	if ok {
		return s
	}

	roots := map[string]bool{start: true}
	queue := []string{start}

	for len(queue) > 0 {
		k := queue[0]
		queue = queue[1:]

		for _, up := range g.rev[k] {
			if !roots[up] {
				roots[up] = true
				queue = append(queue, up)
			}
		}
	}

	seen := make(map[string]bool, len(roots))

	var walk func(k string)

	walk = func(k string) {
		if seen[k] {
			return
		}

		seen[k] = true

		for _, down := range g.fwd[k] {
			walk(down)
		}
	}

	for k := range roots {
		walk(k)
	}

	delete(seen, start)

	scope := make([]string, 0, len(seen))
	for k := range seen {
		scope = append(scope, g.paths[k])
	}

	slices.Sort(scope)

	g.mu.Lock()
	g.scopes[start] = scope
	g.mu.Unlock()

	return scope
}

// findThroughIncludes is canResolveCall's include step, for a bare or this.
// call that neither the calling file nor its extends chain answered. It looks
// for funcName among the files includeScope names for the calling file, and in
// the extends chain of any component among them, and returns the definition
// and the file that led to it.
func (r *Resolver) findThroughIncludes(pr *parser.ParseResult, funcName string) (*parser.FunctionDef, string) {
	if pr == nil || pr.URI == "" {
		return nil, ""
	}

	file := cfpath.FromURI(string(pr.URI))

	if g := r.includes(); g != nil {
		for _, p := range g.includeScope(file) {
			if def := r.scopeFunc(p, funcName); def != nil {
				return def, p
			}
		}
	}

	// A framework's helper templates are included by the framework rather
	// than by the file, which is the only difference that matters here.
	return r.findThroughHelpers(file, funcName)
}

// scopeFunc is funcName as p declares it, for p in an include scope. A
// template has no extends chain and what it includes is in the scope itself,
// so only its own functions are read; a component is looked up whole. A
// dispatcher that includes every page beside it puts a hundred templates in
// each page's scope, and the whole lookup for each was ten times the cost of
// the scan's include step.
func (r *Resolver) scopeFunc(p, funcName string) *parser.FunctionDef {
	if !strings.EqualFold(filepath.Ext(p), ".cfm") && !strings.EqualFold(filepath.Ext(p), ".cfml") {
		return r.LookupFuncWithExtends(p, funcName)
	}

	for _, def := range r.EnsureIndexed(p) {
		if strings.EqualFold(def.Name, funcName) {
			return def
		}
	}

	return nil
}
