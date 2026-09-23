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
// first segment, then each workspace folder.
func (r *Resolver) IncludePath(raw, fromFile string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.Contains(raw, "#") || strings.Contains(raw, "://") {
		return ""
	}

	baseDir := filepath.Dir(fromFile)
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
	}

	for _, root := range r.WorkspaceFolders {
		candidates = append(candidates, filepath.Join(root, filepath.FromSlash(trimmed)))
	}

	for _, c := range candidates {
		if info, err := r.FS.Stat(c); err == nil && !info.IsDir() {
			return c
		}
	}

	return ""
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
			target := r.IncludePath(raw, e.file)
			if target == "" {
				continue
			}

			to := pathKey(target)
			g.paths[to] = target
			g.fwd[from] = append(g.fwd[from], to)
			g.rev[to] = append(g.rev[to], from)
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

	g := r.includes()
	if g == nil {
		return nil, ""
	}

	scope := g.includeScope(file)
	if len(scope) == 0 {
		return nil, ""
	}

	for _, p := range scope {
		if def := r.LookupFuncWithExtends(p, funcName); def != nil {
			return def, p
		}
	}

	return nil, ""
}
