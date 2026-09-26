package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/daemon"
	"github.com/cfmleditor/cfmleditor-lsp/internal/docs"
	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"github.com/cfmleditor/cfmleditor-lsp/internal/resolve"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
	"go.lsp.dev/uri"
)

// cmdExplain prints, for one or more call sites on a given line, every resolution
// step CanResolveCall walked through to reach its verdict — which ComponentRef or
// componentResolver set the receiver's type, which FuncLookup hop fired for a chained
// call, and why the final method-exists check passed or failed. This turns a manual
// trace through script_parser.go/tag_parser.go/result.go/resolve.go into one command.
func cmdExplain(args []string) {
	var configRoot string

	var filteredArgs []string

	for i := 0; i < len(args); i++ {
		if args[i] == "--root" && i+1 < len(args) {
			configRoot = args[i+1]
			i++

			continue
		}

		filteredArgs = append(filteredArgs, args[i])
	}

	args = filteredArgs

	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: cfmleditor-lsp explain [--root <dir>] <file> <line> [call-substring]\n")
		fmt.Fprintf(os.Stderr, "  e.g. cfmleditor-lsp explain directcontent.cfc 104\n")
		fmt.Fprintf(os.Stderr, "       cfmleditor-lsp explain directcontent.cfc 104 createTemplate\n")
		fmt.Fprintf(os.Stderr, "       cfmleditor-lsp explain --root ../tassweb/webroot directcontent.cfc 104\n")
		os.Exit(1)
	}

	file, err := filepath.Abs(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	line, err := strconv.Atoi(args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid line number %q\n", args[1])
		os.Exit(1)
	}

	var filter string
	if len(args) > 2 {
		filter = args[2]
	}

	fsys := vfs.OS{}

	searchDir := filepath.Dir(file)

	if configRoot != "" {
		abs, err := filepath.Abs(configRoot)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		searchDir = abs
	}

	var (
		cfResolvers              []parser.Resolver
		mappings                 map[string]string
		expressionMappings       map[string]string
		servicePropertyResolvers map[string]string
		interpolateAll           bool
		workspaceFolders         []string
	)

	cfg, _ := daemon.FindConfig(searchDir)
	if cfg != nil {
		workspaceFolders = cfg.WorkspaceFolders()
		mappings = cfg.Mappings()
		expressionMappings = cfg.ExpressionMappings()
		servicePropertyResolvers = cfg.ServicePropertyResolvers()
		interpolateAll = !cfg.ResolvedFeatures().OutputContextInterpolation

		for _, r := range cfg.ComponentResolvers() {
			cfResolvers = append(cfResolvers, parser.Resolver{Match: r.Match, Resolve: r.Resolve, Prefix: r.Prefix, NoFollow: r.NoFollow, Anchored: r.Anchored, DynamicIfMissing: r.DynamicIfMissing})
		}

		fmt.Fprintf(os.Stderr, "Using config: %s\n", cfg.Path)
	} else {
		workspaceFolders = []string{searchDir}
	}

	files := collectCFMLFiles(fsys, workspaceFolders)

	resolver := &resolve.Resolver{
		FS:                 fsys,
		Index:              index.New(),
		Resolvers:          cfResolvers,
		Mappings:           mappings,
		ExpressionMappings: expressionMappings,
		WorkspaceFolders:   workspaceFolders,
	}

	fmt.Fprintf(os.Stderr, "Indexing %d files...\n", len(files))

	for _, f := range files {
		if !cfpath.IsCFCFile(f) {
			continue
		}

		data, err := fsys.ReadFile(f)
		if err != nil || cfpath.IsBinary(data) {
			continue
		}

		resolver.Index.IndexFile(uri.URI("file://"+f), string(data))
	}

	data, err := fsys.ReadFile(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading %s: %v\n", file, err)
		os.Exit(1)
	}

	content := string(data)
	fileURI := uri.URI("file://" + file)
	baseDir := filepath.Dir(file)

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

	pr := parser.ParseWithOptions(fileURI, content, parser.ParseOptions{
		Resolvers:                cfResolvers,
		ExpressionMappings:       expressionMappings,
		ServicePropertyResolvers: servicePropertyResolvers,
		InterpolateAllText:       interpolateAll,
		ExtractCalls:             true,
		ScanAllScopes:            true,
		FuncLookup:               funcLookup,
		BuiltinReturnLookup:      docs.LookupBuiltinReturnComponent,
	})
	pr.FuncLookup = funcLookup

	// The selection and the report are shared with cfmleditor.explainCall, so
	// the server's answer reads exactly as this one does.
	matches := resolve.CallsOnLine(pr, line-1, filter)

	if len(matches) == 0 {
		fmt.Fprintf(os.Stderr, "no call sites found on %s:%d\n", file, line)
		os.Exit(1)
	}

	resolver.WriteExplanation(os.Stdout, file, line-1, matches, pr, baseDir)
}
