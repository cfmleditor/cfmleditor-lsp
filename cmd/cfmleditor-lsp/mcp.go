package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/cfmleditor/cfmleditor-lsp/internal/codemap/mcp"
	"github.com/cfmleditor/cfmleditor-lsp/internal/codemap/store"
	"github.com/cfmleditor/cfmleditor-lsp/internal/daemon"
	"github.com/cfmleditor/cfmleditor-lsp/internal/docs"
	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"github.com/cfmleditor/cfmleditor-lsp/internal/resolve"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

const mcpUsage = `usage: cfmleditor-lsp mcp --db <file> [--root <dir>] [--no-explain]

Serve a code map over the Model Context Protocol on stdio, so an assistant can
query the codebase's structure instead of grepping for it. Read-only: every
tool is a query, and none of them writes a file or runs a command.

  --db <file>     the database written by "graph --db" (required)
  --root <dir>    workspace root for explain_call (default: the map's own root)
  --no-explain    do not offer explain_call, and never parse source

Build the map first:
  cfmleditor-lsp graph --db .cfmleditor/codemap.db .

Then register this command with your MCP client, for example:
  {"command": "cfmleditor-lsp", "args": ["mcp", "--db", ".cfmleditor/codemap.db"]}
`

// fatalf prints to stderr and exits. It exists so a caller that has already
// released its resources can exit without gocritic flagging the os.Exit as
// skipping a defer it deliberately does not need.
func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(1)
}

func cmdMCP(args []string) {
	var (
		dbPath    string
		root      string
		noExplain bool
	)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--db", "--root":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "%s needs a value\n\n%s", args[i], mcpUsage)
				os.Exit(1)
			}

			if args[i] == "--db" {
				dbPath = args[i+1]
			} else {
				root = args[i+1]
			}

			i++
		case "--no-explain":
			noExplain = true
		case "-h", "--help":
			fmt.Fprint(os.Stderr, mcpUsage)

			return
		default:
			fmt.Fprintf(os.Stderr, "unknown option %q\n\n%s", args[i], mcpUsage)
			os.Exit(1)
		}
	}

	if dbPath == "" {
		fmt.Fprint(os.Stderr, mcpUsage)
		os.Exit(1)
	}

	if _, err := os.Stat(dbPath); err != nil {
		fmt.Fprintf(os.Stderr, "No map at %s. Build one first:\n  cfmleditor-lsp graph --db %s <dir>\n", dbPath, dbPath)
		os.Exit(1)
	}

	db, err := store.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	defer func() { _ = db.Close() }()

	if _, err := db.Stats(); err != nil {
		// Closed explicitly and then exited via a helper, because os.Exit runs no
		// defers and a WAL-mode database left open strands its -wal and -shm files.
		_ = db.Close()

		fatalf("%s holds no map yet. Run: cfmleditor-lsp graph --db %s <dir>\n", dbPath, dbPath)
	}

	srv := &mcp.Server{Store: db, Name: "cfmleditor-lsp codemap", Version: version}

	if !noExplain {
		if root == "" {
			root, _ = db.Meta("root")
		}

		if root != "" {
			srv.Explain = newExplainer(root)
		}
	}

	// Diagnostics go to stderr. Stdout is the protocol channel, and one stray line
	// on it is a parse error at the other end rather than a log message.
	fmt.Fprintf(os.Stderr, "cfmleditor-lsp codemap MCP server on stdio (db: %s)\n", dbPath)

	if err := srv.Serve(os.Stdin, os.Stdout); err != nil {
		// Close before exiting rather than relying on the defer, which os.Exit
		// skips: this is a WAL-mode database and an unclosed one strands its -wal
		// and -shm files next to it for the next reader to recover.
		_ = db.Close()

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
		workspaceFolders []string
	)

	if c, _ := daemon.FindConfig(abs); c != nil {
		workspaceFolders = c.WorkspaceFolders()
		mappings = c.Mappings()
		cfg.expressionMappings = c.ExpressionMappings()
		cfg.servicePropertyResolvers = c.ServicePropertyResolvers()
		cfg.interpolateAll = !c.ResolvedFeatures().OutputContextInterpolation

		for _, r := range c.ComponentResolvers() {
			cfg.resolvers = append(cfg.resolvers, parser.Resolver{
				Match: r.Match, Resolve: r.Resolve, Prefix: r.Prefix,
				NoFollow: r.NoFollow, Anchored: r.Anchored, DynamicIfMissing: r.DynamicIfMissing,
			})
		}
	}

	scanRoots := workspaceFolders
	if len(scanRoots) == 0 {
		scanRoots = []string{abs}
	}

	resolver := &resolve.Resolver{
		FS:                 fsys,
		Index:              index.New(),
		Resolvers:          cfg.resolvers,
		Mappings:           mappings,
		ExpressionMappings: cfg.expressionMappings,
		WorkspaceFolders:   workspaceFolders,
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

	funcLookup := func(component, funcName string) string {
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
	target := uint32(line - 1)

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
