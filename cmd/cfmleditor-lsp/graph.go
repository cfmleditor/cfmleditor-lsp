package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cfmleditor/cfmleditor-lsp/internal/codemap"
	"github.com/cfmleditor/cfmleditor-lsp/internal/codemap/store"
	"github.com/cfmleditor/cfmleditor-lsp/internal/daemon"
	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"github.com/cfmleditor/cfmleditor-lsp/internal/resolve"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

const graphUsage = `usage: cfmleditor-lsp graph [options] <dir> [...]

Build a whole-project map: every function and file, and the calls,
instantiations, inheritance and includes between them.

  --db <file>      save the map to a SQLite database, and reuse its per-file
                   cache so an unchanged file is not parsed again
  --no-cache       with --db, rebuild every file rather than reusing the cache
  --format <f>     text (default), json, jsonl, dot, mermaid, html
  --level <l>      function (default), call, file, package
                     function: functions and files, with every relationship
                     call:     only functions and the calls between them
                     file:     one node per file
                     package:  one node per directory
  --out <file>     write here instead of stdout
  --from <id>      keep only what these node ids reach (repeatable)
  --live           keep only what an entry point reaches
  --detached       keep only what no entry point reaches
  --under <path>   scope to this path prefix, keeping the nodes just outside
                   it that an edge crosses into, so callers from elsewhere
                   still appear (marked as boundary nodes)
  --under-strict   with --under, cut hard at the prefix instead
  --unresolved     include calls that resolved to nothing
  --builtins       include calls to built-in CFML functions
  --entry <glob>   also treat files matching this as entry points, for code a
                   runner invokes by a constructed name (repeatable), e.g.
                   --entry '../prs' --entry 'tasks/*'
  --one-config     read every file under the scan root's own .cfmleditor.json,
                   instead of each file's nearest one
  --workers <n>    scan parallelism (default: GOMAXPROCS)
  --limit <n>      list length in the text report (default 20)
  --quiet          no progress on stderr

Every declared function is in the map whether or not anything calls it.
Unreachable code is not dropped: it becomes its own island with its own
root, which --detached lists and the html view draws as a separate tree.
`

type graphFlags struct {
	db          string
	noCache     bool
	format      string
	level       string
	out         string
	under       string
	underStrict bool
	entryGlobs  []string
	from        []string
	live        bool
	detached    bool
	unresolved  bool
	builtins    bool
	oneConfig   bool
	workers     int
	limit       int
	quiet       bool
	paths       []string
}

func cmdGraph(args []string) {
	flags, err := parseGraphFlags(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n\n%s", err, graphUsage)
		os.Exit(1)
	}

	m, err := buildGraph(flags)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	m = applyGraphViews(m, flags)

	if err := writeGraph(m, flags); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func parseGraphFlags(args []string) (graphFlags, error) {
	f := graphFlags{format: "text", level: "function", limit: 20}

	for i := 0; i < len(args); i++ {
		next := func(name string) (string, error) {
			if i+1 >= len(args) {
				return "", fmt.Errorf("%s needs a value", name)
			}

			i++

			return args[i], nil
		}

		var err error

		switch a := args[i]; a {
		case "--db":
			f.db, err = next(a)
		case "--no-cache":
			f.noCache = true
		case "--format":
			f.format, err = next(a)
		case "--level":
			f.level, err = next(a)
		case "--out":
			f.out, err = next(a)
		case "--under":
			f.under, err = next(a)
		case "--under-strict":
			f.underStrict = true
		case "--entry":
			var v string

			v, err = next(a)
			f.entryGlobs = append(f.entryGlobs, v)
		case "--from":
			var v string

			v, err = next(a)
			f.from = append(f.from, v)
		case "--workers":
			var v string

			v, err = next(a)
			if err == nil {
				_, err = fmt.Sscanf(v, "%d", &f.workers)
			}
		case "--limit":
			var v string

			v, err = next(a)
			if err == nil {
				_, err = fmt.Sscanf(v, "%d", &f.limit)
			}
		case "--live":
			f.live = true
		case "--detached":
			f.detached = true
		case "--unresolved":
			f.unresolved = true
		case "--builtins":
			f.builtins = true
		case "--one-config":
			f.oneConfig = true
		case "--quiet":
			f.quiet = true
		case "-h", "--help":
			fmt.Fprint(os.Stderr, graphUsage)
			os.Exit(0)
		default:
			if strings.HasPrefix(a, "-") {
				err = fmt.Errorf("unknown option %q", a)
			} else {
				f.paths = append(f.paths, a)
			}
		}

		if err != nil {
			return f, err
		}
	}

	if len(f.paths) == 0 {
		return f, fmt.Errorf("no directory given")
	}

	if f.live && f.detached {
		return f, fmt.Errorf("--live and --detached are opposites; pass at most one")
	}

	return f, nil
}

func buildGraph(f graphFlags) (*codemap.Map, error) {
	fsys := vfs.OS{}

	root, err := filepath.Abs(f.paths[0])
	if err != nil {
		return nil, fmt.Errorf("resolving %s: %w", f.paths[0], err)
	}

	if info, err := os.Stat(root); err == nil && !info.IsDir() {
		root = filepath.Dir(root)
	}

	scanRoots, fallback, shared := routeWorkspace(fsys, root, f)

	files := collectCFMLFiles(fsys, scanRoots)
	if len(files) == 0 {
		return nil, fmt.Errorf("no CFML files found under %s", strings.Join(scanRoots, ", "))
	}

	configs := newConfigSet(fsys, shared, fallback)
	if !f.oneConfig {
		configs.preload(scanRoots)

		if found := configs.Configs(); len(found) > 1 && !f.quiet {
			fmt.Fprintf(os.Stderr, "Resolving each file under its own config (%d found):\n", len(found))

			for _, c := range found {
				fmt.Fprintf(os.Stderr, "  %s\n", c)
			}
		}
	}

	var (
		db    *store.Store
		cache *store.Cache
	)

	if f.db != "" {
		db, err = store.Open(f.db)
		if err != nil {
			return nil, err
		}

		defer func() { _ = db.Close() }()

		if !f.noCache {
			cache = db.NewCache()
		}
	}

	opts := codemap.Options{
		Root:                     root,
		Files:                    files,
		FS:                       fsys,
		Resolver:                 fallback.Resolver,
		Resolvers:                fallback.Resolvers,
		ExpressionMappings:       fallback.ExpressionMappings,
		ServicePropertyResolvers: fallback.ServicePropertyResolvers,
		Workers:                  f.workers,
		ConfigExtra:              configs.Fingerprint(),
		EntryGlobs:               f.entryGlobs,
		IncludeUnresolved:        f.unresolved,
		IncludeBuiltins:          f.builtins,
	}

	if cache != nil {
		opts.Cache = cache
	}

	// Without this every file is read under whichever config sits above the scan
	// root, which on a workspace of several applications means most of the code is
	// resolved with somebody else's resolvers and produces no edges at all.
	if !f.oneConfig {
		opts.ConfigFor = configs.For
	}

	if !f.quiet {
		opts.Progress = graphProgress()
	}

	m := codemap.Build(opts)

	if !f.quiet {
		fmt.Fprintf(os.Stderr, "\n")
	}

	if cache != nil {
		if err := cache.Flush(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not write the file cache: %v\n", err)
		}

		if hits, misses := cache.Hits(); !f.quiet {
			fmt.Fprintf(os.Stderr, "Cache: %d hits, %d misses\n", hits, misses)
		}
	}

	// The map is saved before any view narrows it: a database holding a filtered
	// map would answer every later question about a subset of the codebase while
	// looking like it held all of it.
	if db != nil {
		if err := db.Save(m); err != nil {
			return nil, err
		}

		if err := db.PruneCache(m.Fingerprint); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not prune old cache generations: %v\n", err)
		}

		if !f.quiet {
			fmt.Fprintf(os.Stderr, "Saved %d nodes and %d edges to %s\n", len(m.Nodes), len(m.Edges), f.db)
		}
	}

	return m, nil
}

// graphProgress throttles to one line a second. A build over thousands of files
// calls this thousands of times, and a stderr write per file costs more than the
// parse it is reporting on.
func graphProgress() func(codemap.Phase, int, int) {
	last := time.Now()

	return func(phase codemap.Phase, done, total int) {
		if done < total && time.Since(last) < time.Second {
			return
		}

		last = time.Now()

		fmt.Fprintf(os.Stderr, "\r%s %d/%d   ", phase, done, total)
	}
}

func applyGraphViews(m *codemap.Map, f graphFlags) *codemap.Map {
	if f.under != "" {
		if f.underStrict {
			m = m.Filter(f.under)
		} else {
			m = m.FilterUnder(f.under)
		}
	}

	switch {
	case len(f.from) > 0:
		m = m.Reachable(f.from)
	case f.live:
		m = m.Reachable(nil)
	case f.detached:
		m = m.Detached()
	}

	switch f.level {
	case "call":
		m = m.CallGraph()
	case "file":
		m = m.Collapse(codemap.LevelFile)
	case "package":
		m = m.Collapse(codemap.LevelPackage)
	}

	return m
}

func writeGraph(m *codemap.Map, f graphFlags) error {
	out := os.Stdout

	if f.out != "" {
		file, err := os.Create(f.out)
		if err != nil {
			return fmt.Errorf("creating %s: %w", f.out, err)
		}

		defer func() { _ = file.Close() }()

		out = file
	}

	w := bufio.NewWriter(out)

	var err error

	switch f.format {
	case "json":
		err = m.WriteJSON(w)
	case "jsonl":
		err = m.WriteJSONL(w)
	case "dot":
		err = m.WriteDOT(w)
	case "mermaid":
		err = m.WriteMermaid(w, 2000)
	case "html":
		err = m.WriteHTML(w, "Code map — "+filepath.Base(m.Root))
	case "text":
		err = m.WriteText(w, f.limit)
	default:
		return fmt.Errorf("unknown format %q (want text, json, jsonl, dot, mermaid or html)", f.format)
	}

	if err != nil {
		return err
	}

	// Flushed explicitly, not deferred: a failure here is a truncated file, and a
	// deferred flush would discard that error and report success.
	if err := w.Flush(); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	if f.out != "" && !f.quiet {
		fmt.Fprintf(os.Stderr, "Wrote %s (%d nodes, %d edges)\n", f.out, len(m.Nodes), len(m.Edges))
	}

	return nil
}

// routeWorkspace loads the config governing root, collects the files to scan, and
// builds the fallback resolution environment.
//
// Shared by `graph` and `routes` so the two cannot disagree about which config
// governs a file — a diagnostic that scanned a different workspace from the build
// it is meant to explain would be worse than none.
func routeWorkspace(fsys vfs.FS, root string, f graphFlags) (scanRoots []string, fallback codemap.FileConfig, shared *index.Index) {
	cfg, _ := daemon.FindConfig(root)

	var (
		resolvers                []parser.Resolver
		mappings                 map[string]string
		expressionMappings       map[string]string
		servicePropertyResolvers map[string]string
		workspaceFolders         []string
	)

	if cfg != nil {
		workspaceFolders = cfg.WorkspaceFolders()
		mappings = cfg.Mappings()
		expressionMappings = cfg.ExpressionMappings()
		servicePropertyResolvers = cfg.ServicePropertyResolvers()

		for _, r := range cfg.ComponentResolvers() {
			resolvers = append(resolvers, parser.Resolver{
				Match: r.Match, Resolve: r.Resolve, Prefix: r.Prefix,
				NoFollow: r.NoFollow, Anchored: r.Anchored,
			})
		}

		if !f.quiet {
			fmt.Fprintf(os.Stderr, "Using config: %s\n", cfg.Path)
		}
	}

	scanRoots = workspaceFolders
	if len(scanRoots) == 0 {
		scanRoots = f.paths
	}

	// One index for the whole scan, shared by every config: function signatures are
	// a property of the workspace, not of whose resolvers you read them under.
	shared = index.New()

	fallback = codemap.FileConfig{
		Resolver: &resolve.Resolver{
			FS:                 fsys,
			Index:              shared,
			Resolvers:          resolvers,
			Mappings:           mappings,
			ExpressionMappings: expressionMappings,
			WorkspaceFolders:   workspaceFolders,
		},
		Resolvers:                resolvers,
		ExpressionMappings:       expressionMappings,
		ServicePropertyResolvers: servicePropertyResolvers,
	}

	return scanRoots, fallback, shared
}
