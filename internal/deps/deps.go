// Package deps builds transitive dependency graphs for CFML components.
package deps

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/graph"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/uri"
)

// Index provides the subset of index operations needed for dependency tracing.
type Index interface {
	RefsForFile(fileURI uri.URI) []*parser.ComponentRef
	FunctionsForFile(fileURI uri.URI) []*parser.FunctionDef
}

// Resolver resolves component dot-paths to file paths.
type Resolver interface {
	ComponentPath(component, baseDir string) string
}

// Options configures the dependency graph generation.
type Options struct {
	DocURI   string
	FuncName string                // optional: scope to a specific function
	Calls    []parser.CallSite     // function-level calls (from FuncCalls)
	Refs     []parser.ComponentRef // file-level refs (fallback when Calls is empty)
	Index    Index
	Resolver Resolver
	MaxDepth int
}

// Result holds the dependency graph output.
type Result struct {
	Graph graph.Graph
}

// Build generates a transitive dependency graph.
// If Calls is provided, traces function-level dependencies.
// Otherwise falls back to component-level refs.
func Build(opts Options) Result {
	filePath := strings.TrimPrefix(opts.DocURI, "file://")
	baseDir := filepath.Dir(filePath)

	maxDepth := opts.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 10
	}

	startLabel := filepath.Base(filePath)
	if opts.FuncName != "" {
		startLabel += "::" + opts.FuncName
	}

	var edges []graph.Edge

	seen := make(map[string]bool)
	seen[opts.DocURI+"::"+opts.FuncName] = true

	if len(opts.Calls) > 0 {
		edges = buildFromCalls(opts, startLabel, baseDir, maxDepth, seen)
	} else {
		edges = buildFromRefs(opts, startLabel, baseDir, maxDepth, seen)
	}

	return Result{Graph: graph.Graph{Direction: "LR", Edges: edges}}
}

// buildFromCalls traces function-level dependencies using CallSite data.
func buildFromCalls(opts Options, startLabel, baseDir string, maxDepth int, seen map[string]bool) []graph.Edge {
	// baseDir travels with each node. It is what ComponentPath resolves a
	// component against — relative paths and the governing Application.cfc both
	// depend on it — and carrying the *start* file's directory through the whole
	// traversal meant every transitive hop was resolved as though it lived
	// beside the file the graph started from. Dependencies one hop out in
	// another directory came back unresolved and were drawn as dashed edges.
	type node struct {
		label   string
		calls   []parser.CallSite
		baseDir string
	}

	var edges []graph.Edge

	queue := []node{{label: startLabel, calls: opts.Calls, baseDir: baseDir}}

	for depth := 0; depth < maxDepth && len(queue) > 0; depth++ {
		var next []node

		for _, current := range queue {
			for _, call := range current.calls {
				if call.Component == "" && call.Variable == "" {
					continue
				}

				resolved := ""
				toLabel := call.FuncName

				if call.Component != "" {
					resolved = opts.Resolver.ComponentPath(call.Component, current.baseDir)
					toLabel = call.Component + "." + call.FuncName

					if resolved != "" {
						toLabel = filepath.Base(resolved) + "::" + call.FuncName
					}
				} else if call.Variable != "" {
					toLabel = call.Variable + "." + call.FuncName
				}

				edges = append(edges, graph.Edge{
					From:   current.label,
					To:     fmt.Sprintf("%s (line %d)", toLabel, call.Line),
					Dashed: resolved == "",
				})

				if resolved == "" {
					continue
				}

				targetKey := resolved + "::" + call.FuncName
				if seen[targetKey] {
					continue
				}

				seen[targetKey] = true

				// Get calls for the target function from the index
				targetURI := uri.URI("file://" + resolved)

				targetCalls := getFuncCalls(opts.Index, targetURI, call.FuncName)
				if len(targetCalls) > 0 {
					next = append(next, node{label: toLabel, calls: targetCalls, baseDir: filepath.Dir(resolved)})
				}
			}
		}

		queue = next
	}

	return edges
}

// buildFromRefs traces component-level dependencies using ComponentRef data.
func buildFromRefs(opts Options, startLabel, baseDir string, maxDepth int, seen map[string]bool) []graph.Edge {
	// baseDir travels with each node, for the reason given in buildFromCalls.
	type node struct {
		label   string
		refs    []parser.ComponentRef
		baseDir string
	}

	var edges []graph.Edge

	queue := []node{{label: startLabel, refs: opts.Refs, baseDir: baseDir}}

	for depth := 0; depth < maxDepth && len(queue) > 0; depth++ {
		var next []node

		for _, current := range queue {
			for _, ref := range current.refs {
				if ref.Component == "" {
					continue
				}

				resolved := opts.Resolver.ComponentPath(ref.Component, current.baseDir)
				toLabel := ref.Component

				if resolved != "" {
					toLabel = filepath.Base(resolved)
				}

				edges = append(edges, graph.Edge{
					From: current.label,
					To:   fmt.Sprintf("%s (line %d)", toLabel, ref.Line),
				})

				if resolved == "" {
					continue
				}

				targetURI := "file://" + resolved
				if seen[targetURI] {
					continue
				}

				seen[targetURI] = true
				ptrs := opts.Index.RefsForFile(uri.URI(targetURI))

				var nextRefs []parser.ComponentRef

				for _, p := range ptrs {
					nextRefs = append(nextRefs, *p)
				}

				next = append(next, node{
					label:   filepath.Base(resolved),
					refs:    nextRefs,
					baseDir: filepath.Dir(resolved),
				})
			}
		}

		queue = next
	}

	return edges
}

// getFuncCalls retrieves CallSite data for a function. This requires the file
// to be parsed — we check if the index has the function, then return empty
// (the caller should provide parsed data for deeper tracing).
func getFuncCalls(_ Index, _ uri.URI, _ string) []parser.CallSite {
	// TODO: For deeper recursive tracing, the handler should provide a callback
	// or pre-parse target files. For now, we stop at one level of call resolution.
	return nil
}
