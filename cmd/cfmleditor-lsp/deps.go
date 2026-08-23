package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/daemon"
	"github.com/cfmleditor/cfmleditor-lsp/internal/deps"
	"github.com/cfmleditor/cfmleditor-lsp/internal/graph"
	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"github.com/cfmleditor/cfmleditor-lsp/internal/resolve"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
	"go.lsp.dev/uri"
)

// DepEdge is one dependency edge, attributed to the file it was found in.
//
// From and To are the graph's node labels, so they carry the resolved component
// and the line the dependency was written on. Dashed marks a dependency whose
// component could not be resolved to a file — the previous output had no way to
// express that, and reported resolvable and unresolvable dependencies alike.
type DepEdge struct {
	FromFile string `json:"fromFile"`
	From     string `json:"from"`
	To       string `json:"to"`
	Dashed   bool   `json:"dashed,omitempty"`
}

// DepResult holds the full dependency graph.
type DepResult struct {
	Files int       `json:"files"`
	Edges []DepEdge `json:"edges"`
}

func cmdDeps(args []string) {
	format := "json"

	var filteredArgs []string

	for _, a := range args {
		if a == "--mermaid" {
			format = "mermaid"

			continue
		}

		filteredArgs = append(filteredArgs, a)
	}

	args = filteredArgs

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: cfmleditor-lsp deps [--mermaid] <dir-or-file> [...]\n")
		os.Exit(1)
	}

	fsys := vfs.OS{}

	files := collectCFMLFiles(fsys, args)
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "no CFML files found\n")
		os.Exit(1)
	}

	resolver, idx := depsResolver(fsys, args, files)

	var (
		edges []DepEdge
		seen  = make(map[string]bool)
	)

	for _, f := range files {
		for _, e := range depsForFile(f, resolver, idx) {
			key := f + "\x00" + e.From + "\x00" + e.To
			if seen[key] {
				continue
			}

			seen[key] = true

			edges = append(edges, DepEdge{FromFile: f, From: e.From, To: e.To, Dashed: e.Dashed})
		}
	}

	result := DepResult{Files: len(files), Edges: edges}

	if format == "mermaid" {
		g := graph.Graph{Direction: "LR"}
		for _, e := range edges {
			g.Edges = append(g.Edges, graph.Edge{From: e.From, To: e.To, Dashed: e.Dashed})
		}

		fmt.Println(g.Mermaid())

		return
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(result)
}

// depsResolver builds the resolver and index the traversal needs. Resolution is
// what lets a dependency be followed to another file at all; the previous
// implementation did none, which is why it could only ever report one level.
func depsResolver(fsys vfs.FS, args, files []string) (*resolve.Resolver, *index.Index) {
	var (
		workspaceFolders   []string
		mappings           map[string]string
		expressionMappings map[string]string
		cfResolvers        []parser.Resolver
	)

	searchDir := "."
	if info, err := os.Stat(args[0]); err == nil && info.IsDir() {
		searchDir = args[0]
	} else if len(args) > 0 {
		searchDir = filepath.Dir(args[0])
	}

	if cfg, _ := daemon.FindConfig(searchDir); cfg != nil {
		workspaceFolders = cfg.WorkspaceFolders()
		mappings = cfg.Mappings()
		expressionMappings = cfg.ExpressionMappings()

		for _, r := range cfg.ComponentResolvers() {
			cfResolvers = append(cfResolvers, parser.Resolver{
				Match: r.Match, Resolve: r.Resolve, Prefix: r.Prefix,
				NoFollow: r.NoFollow, Anchored: r.Anchored,
			})
		}
	} else {
		for _, a := range args {
			if info, err := os.Stat(a); err == nil && info.IsDir() {
				abs, _ := filepath.Abs(a)
				workspaceFolders = append(workspaceFolders, abs)
			}
		}
	}

	idx := index.New()

	for _, f := range files {
		if !cfpath.IsCFCFile(f) {
			continue
		}

		content, err := os.ReadFile(f)
		if err != nil {
			continue
		}

		abs, _ := filepath.Abs(f)
		pr := parser.ParseWithOptions(uri.File(abs), string(content), parser.ParseOptions{
			Resolvers: cfResolvers, ExpressionMappings: expressionMappings,
		})
		idx.IndexFileFromResult(pr.URI, pr.Funcs, pr.ComponentRefs)
	}

	return &resolve.Resolver{
		FS:                 fsys,
		Index:              idx,
		Resolvers:          cfResolvers,
		Mappings:           mappings,
		ExpressionMappings: expressionMappings,
		WorkspaceFolders:   workspaceFolders,
	}, idx
}

// depsForFile builds the dependency graph rooted at one file, the same way
// cfmleditor.exportDeps does for a whole document — so the CLI and the editor
// command answer with the same graph rather than each having their own idea of
// what a dependency is.
func depsForFile(path string, resolver *resolve.Resolver, idx *index.Index) []graph.Edge {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	abs, _ := filepath.Abs(path)
	fileURI := uri.File(abs)

	pr := parser.ParseWithOptions(fileURI, string(content), parser.ParseOptions{
		Resolvers: resolver.Resolvers, ExpressionMappings: resolver.ExpressionMappings,
		ExtractCalls: true,
	})

	var calls []parser.CallSite
	for _, sc := range pr.Scopes {
		calls = append(calls, pr.FuncCalls(sc.Start, sc.End)...)
	}

	result := deps.Build(deps.Options{
		DocURI:    string(fileURI),
		Calls:     calls,
		Refs:      pr.ComponentRefs,
		Index:     idx,
		Resolver:  resolver,
		LoadCalls: depsCallLoader(resolver),
		MaxDepth:  10,
	})

	return result.Graph.Edges
}

// depsCallLoader parses a target file on demand so the traversal can continue
// past the first hop, memoised for the life of one command.
func depsCallLoader(resolver *resolve.Resolver) func(uri.URI, string) ([]parser.CallSite, []parser.ComponentRef) {
	parsed := make(map[uri.URI]*parser.ParseResult)

	return func(fileURI uri.URI, funcName string) ([]parser.CallSite, []parser.ComponentRef) {
		pr, ok := parsed[fileURI]
		if !ok {
			data, err := os.ReadFile(strings.TrimPrefix(string(fileURI), "file://"))
			if err != nil {
				parsed[fileURI] = nil

				return nil, nil
			}

			pr = parser.ParseWithOptions(fileURI, string(data), parser.ParseOptions{
				Resolvers: resolver.Resolvers, ExpressionMappings: resolver.ExpressionMappings,
				ExtractCalls: true,
			})
			parsed[fileURI] = pr
		}

		if pr == nil {
			return nil, nil
		}

		for _, sc := range pr.Scopes {
			for _, f := range pr.Funcs {
				if !strings.EqualFold(f.Name, funcName) || int(f.Line) != sc.Start {
					continue
				}

				// Function locals first, then file-level: a `var x = new Foo()`
				// shadows a `variables.x`, and the consumer takes the first match.
				refs := append([]parser.ComponentRef{}, pr.FuncComponentRefs(sc.Start, sc.End)...)
				refs = append(refs, pr.ComponentRefs...)

				return pr.FuncCalls(sc.Start, sc.End), refs
			}
		}

		return nil, nil
	}
}
