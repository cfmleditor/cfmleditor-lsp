package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/cfmleditor/cfmleditor-lsp/internal/codemap/mcp"
	"github.com/cfmleditor/cfmleditor-lsp/internal/codemap/store"
	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
	"github.com/cfmleditor/cfmleditor-lsp/internal/conv"
	"github.com/cfmleditor/cfmleditor-lsp/internal/daemon"
	"github.com/cfmleditor/cfmleditor-lsp/internal/docs"
	"github.com/cfmleditor/cfmleditor-lsp/internal/frameworkapi"
	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"github.com/cfmleditor/cfmleditor-lsp/internal/resolve"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

const mcpUsage = `usage: cfmleditor-lsp mcp [--db <file>] [--root <dir>] [--allow-lint] [--map-only] [--no-explain]

Serve a CFML workspace over the Model Context Protocol on stdio, so an
assistant can ask about the code instead of grepping for it. It writes nothing.

Always offered, reading source on disk:
  find_unresolved_calls   what "unresolved" reports
  find_references         what "refs" reports
  explain_call            what "explain" reports
With --db, the code map "graph --db" built:
  search_symbols, get_symbol, get_callers, get_callees, find_path,
  list_islands, list_orphans, get_stats
With --allow-lint:
  lint                    CFLint's findings, as "cflint" reports them; starts a
                          Java process and downloads CFLint on first use

  --db <file>     the database written by "graph --db"
  --root <dir>    workspace root for explain_call (default: the map's own root,
                  or the working directory without a map)
  --allow-lint    offer the lint tool
  --map-only      offer only the map tools, and never read source (needs --db)
  --no-explain    do not offer explain_call

Paths given to a tool are relative to the server's working directory.

Register it with your MCP client, for example:
  {"command": "cfmleditor-lsp", "args": ["mcp", "--allow-lint"], "cwd": "/path/to/project"}
or with a map, built first by: cfmleditor-lsp graph --db .cfmleditor/codemap.db .
  {"command": "cfmleditor-lsp", "args": ["mcp", "--db", ".cfmleditor/codemap.db"]}
`

// fatalf prints to stderr and exits. It exists so a caller that has already
// released its resources can exit without gocritic flagging the os.Exit as
// skipping a defer it deliberately does not need.
func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(1)
}

type mcpFlags struct {
	db, root                      string
	allowLint, mapOnly, noExplain bool
}

func parseMCPFlags(args []string) mcpFlags {
	var f mcpFlags

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--db", "--root":
			if i+1 >= len(args) {
				fatalf("%s needs a value\n\n%s", args[i], mcpUsage)
			}

			if args[i] == "--db" {
				f.db = args[i+1]
			} else {
				f.root = args[i+1]
			}

			i++
		case "--allow-lint":
			f.allowLint = true
		case "--map-only":
			f.mapOnly = true
		case "--no-explain":
			f.noExplain = true
		default:
			fatalf("unknown option %q\n\n%s", args[i], mcpUsage)
		}
	}

	if f.mapOnly && f.db == "" {
		fatalf("--map-only needs --db: without a map there is nothing to offer\n\n%s", mcpUsage)
	}

	if f.mapOnly && f.allowLint {
		fatalf("--map-only and --allow-lint cannot be used together\n\n%s", mcpUsage)
	}

	return f
}

// openMap opens and checks the map at path, or exits saying how to build one.
func openMap(path string) *store.Store {
	if _, err := os.Stat(path); err != nil {
		fatalf("No map at %s. Build one first:\n  cfmleditor-lsp graph --db %s <dir>\n", path, path)
	}

	db, err := store.Open(path)
	if err != nil {
		fatalf("%v\n", err)
	}

	if _, err := db.Stats(); err != nil {
		// Closed explicitly and then exited via a helper, because os.Exit runs no
		// defers and a WAL-mode database left open strands its -wal and -shm files.
		_ = db.Close()

		fatalf("%s holds no map yet. Run: cfmleditor-lsp graph --db %s <dir>\n", path, path)
	}

	return db
}

// mcpServer builds the server the flags ask for. db may be nil.
func mcpServer(f *mcpFlags, db *store.Store) *mcp.Server {
	srv := &mcp.Server{Store: db, Name: "cfmleditor-lsp", Version: version}

	if f.mapOnly {
		return srv
	}

	srv.Unresolved = mcpUnresolved
	srv.FindRefs = mcpFindRefs

	if f.allowLint {
		srv.Lint = newMCPLinter()
	}

	if !f.noExplain {
		root := f.root
		if root == "" && db != nil {
			root, _ = db.Meta("root")
		}

		if root == "" {
			root = mustGetwd()
		}

		srv.Explain = newExplainer(root)
	}

	return srv
}

func cmdMCP(args []string) {
	f := parseMCPFlags(args)

	var db *store.Store
	if f.db != "" {
		db = openMap(f.db)
		defer func() { _ = db.Close() }()
	}

	srv := mcpServer(&f, db)

	// Diagnostics go to stderr. Stdout is the protocol channel, and one stray line
	// on it is a parse error at the other end rather than a log message.
	if f.db != "" {
		fmt.Fprintf(os.Stderr, "cfmleditor-lsp MCP server on stdio (db: %s)\n", f.db)
	} else {
		fmt.Fprintf(os.Stderr, "cfmleditor-lsp MCP server on stdio (no code map)\n")
	}

	if err := srv.Serve(os.Stdin, os.Stdout); err != nil {
		// Close before exiting rather than relying on the defer, which os.Exit
		// skips: this is a WAL-mode database and an unclosed one strands its -wal
		// and -shm files next to it for the next reader to recover.
		if db != nil {
			_ = db.Close()
		}

		fatalf("%v\n", err)
	}
}

// newExplainer builds the resolver lazily, on the first explain_call.
//
// Indexing the workspace costs seconds on a large one, and most sessions never
// call this tool. Paying it at startup would make every client's connection
// handshake time out on exactly the codebases where the map is most useful.
func newExplainer(root string) mcp.Explainer {
	var (
		once     sync.Once
		resolver *resolve.Resolver
		cfg      explainConfig
	)

	return func(file string, line int, match string) (string, error) {
		once.Do(func() { resolver, cfg = buildExplainResolver(root) })

		if resolver == nil {
			return "", fmt.Errorf("could not index the workspace at %s", root)
		}

		return explainAt(resolver, cfg, file, line, match)
	}
}

type explainConfig struct {
	resolvers                []parser.Resolver
	implicitExtends          func(string) string
	helperScope              func(string) bool
	stubs                    *frameworkapi.Set
	expressionMappings       map[string]string
	servicePropertyResolvers map[string]string
	interpolateAll           bool
}

func buildExplainResolver(root string) (*resolve.Resolver, explainConfig) {
	fsys := vfs.OS{}

	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, explainConfig{}
	}

	var (
		cfg              explainConfig
		mappings         map[string]string
		startupFiles     []string
		workspaceFolders []string
	)

	if c, _ := daemon.FindConfig(abs); c != nil {
		workspaceFolders = cliWorkspaceFolders(fsys, c, []string{abs})
		mappings = c.Mappings()
		startupFiles = c.StartupFiles()
		cfg.expressionMappings = c.ExpressionMappings()
		cfg.implicitExtends = config.ImplicitExtends(c.Frameworks())
		cfg.helperScope = config.HelperScope(c.Frameworks())
		cfg.stubs = frameworkapi.For(c.Frameworks())
		cfg.servicePropertyResolvers = c.ServicePropertyResolvers()
		cfg.interpolateAll = !c.ResolvedFeatures().OutputContextInterpolation

		for _, r := range c.ComponentResolvers() {
			cfg.resolvers = append(cfg.resolvers, r.Parser())
		}
	}

	scanRoots := workspaceFolders
	if len(scanRoots) == 0 {
		scanRoots = []string{abs}
	}

	if len(workspaceFolders) == 0 {
		workspaceFolders = cliWorkspaceFolders(fsys, nil, scanRoots)
	}

	resolver := &resolve.Resolver{
		FS:                 fsys,
		Index:              index.New(),
		Resolvers:          cfg.resolvers,
		Mappings:           mappings,
		StartupFiles:       startupFiles,
		ExpressionMappings: cfg.expressionMappings,
		WorkspaceFolders:   workspaceFolders,
		ImplicitExtends:    cfg.implicitExtends,
		HelperScope:        cfg.helperScope,
		Stubs:              cfg.stubs,
	}

	for _, f := range collectCFMLFiles(fsys, scanRoots) {
		if !cfpath.IsCFCFile(f) {
			continue
		}

		data, err := fsys.ReadFile(f)
		if err != nil || cfpath.IsBinary(data) {
			continue
		}

		resolver.Index.IndexFile(cfpath.ToURI(f), string(data))
	}

	return resolver, cfg
}

func explainAt(resolver *resolve.Resolver, cfg explainConfig, file string, line int, match string) (string, error) {
	abs, err := filepath.Abs(file)
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", file, err)
	}

	data, err := os.ReadFile(abs)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", file, err)
	}

	content := string(data)
	baseDir := filepath.Dir(abs)

	funcLookup := resolver.FuncLookup(baseDir)

	pr := parser.ParseWithOptions(cfpath.ToURI(abs), content, &parser.ParseOptions{
		Resolvers:                cfg.resolvers,
		ExpressionMappings:       cfg.expressionMappings,
		ServicePropertyResolvers: cfg.servicePropertyResolvers,
		InterpolateAllText:       cfg.interpolateAll,
		ExtractCalls:             true,
		ScanAllScopes:            true,
		FuncLookup:               funcLookup,
		BuiltinReturnLookup:      docs.LookupBuiltinReturnComponent,
	})
	pr.FuncLookup = funcLookup

	// The parser numbers lines from zero; users and editors number from one.
	target := conv.Uint32(line - 1)

	var b strings.Builder

	found := 0

	calls := pr.AllCalls()

	for i := range calls {
		call := &calls[i]

		if call.Line != target {
			continue
		}

		if match != "" && !strings.Contains(strings.ToLower(call.Text), strings.ToLower(match)) {
			continue
		}

		found++

		name := call.FuncName
		if call.Variable != "" {
			name = call.Variable + "." + call.FuncName
		}

		reason, steps := resolver.ExplainCall(call, pr, baseDir)

		fmt.Fprintf(&b, "%s:%d  %s\n", file, line, name)

		for _, step := range steps {
			fmt.Fprintf(&b, "    - %s\n", step)
		}

		if reason == "" {
			b.WriteString("    => resolved\n\n")
		} else {
			fmt.Fprintf(&b, "    => UNRESOLVED: %s\n\n", reason)
		}
	}

	if found == 0 {
		return fmt.Sprintf("No call site found at %s:%d%s.\n", file, line,
			map[bool]string{true: "", false: " matching " + match}[match == ""]), nil
	}

	return b.String(), nil
}
