// Package deps builds transitive dependency graphs for CFML components.
package deps

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"github.com/cfmleditor/cfmleditor-lsp/internal/graph"
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
	FuncName string                  // optional: scope to a specific function
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
	type node struct {
		label string
		calls []parser.CallSite
	}
	var edges []graph.Edge
	queue := []node{{label: startLabel, calls: opts.Calls}}

	for depth := 0; depth < maxDepth && len(queue) > 0; depth++ {
		var next []node
		for _, current := range queue {
			for _, call := range current.calls {
				if call.Component == "" {
					continue
				}
				resolved := opts.Resolver.ComponentPath(call.Component, baseDir)
				toLabel := call.Component + "." + call.FuncName
				if resolved != "" {
					toLabel = filepath.Base(resolved) + "::" + call.FuncName
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
					next = append(next, node{label: toLabel, calls: targetCalls})
				}
			}
		}
		queue = next
	}
	return edges
}

// buildFromRefs traces component-level dependencies using ComponentRef data.
func buildFromRefs(opts Options, startLabel, baseDir string, maxDepth int, seen map[string]bool) []graph.Edge {
	type node struct {
		label string
		refs  []parser.ComponentRef
	}
	var edges []graph.Edge
	queue := []node{{label: startLabel, refs: opts.Refs}}

	for depth := 0; depth < maxDepth && len(queue) > 0; depth++ {
		var next []node
		for _, current := range queue {
			for _, ref := range current.refs {
				if ref.Component == "" {
					continue
				}
				resolved := opts.Resolver.ComponentPath(ref.Component, baseDir)
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
					label: filepath.Base(resolved),
					refs:  nextRefs,
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
