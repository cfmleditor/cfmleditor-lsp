// Package deps builds transitive dependency graphs for CFML components.
package deps

import (
	"path/filepath"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/cfparser"
	"github.com/cfmleditor/cfmleditor-lsp/internal/graph"
	"go.lsp.dev/uri"
)

// Index provides the subset of index operations needed for dependency tracing.
type Index interface {
	RefsForFile(fileURI uri.URI) []*cfparser.ComponentRef
	FunctionsForFile(fileURI uri.URI) []*cfparser.FunctionDef
}

// Resolver resolves component dot-paths to file paths.
type Resolver interface {
	ComponentPath(component, baseDir string) string
}

// Options configures the dependency graph generation.
type Options struct {
	DocURI   string
	FuncName string // optional: scope to a specific function
	Index    Index
	Resolver Resolver
	MaxDepth int
}

// Result holds the dependency graph output.
type Result struct {
	Graph graph.Graph
}

// Build generates a transitive dependency graph.
func Build(opts Options) Result {
	filePath := strings.TrimPrefix(opts.DocURI, "file://")
	baseDir := filepath.Dir(filePath)
	maxDepth := opts.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 10
	}

	type node struct {
		uri      string
		funcName string
	}
	var edges []graph.Edge
	seen := make(map[string]bool)

	startKey := opts.DocURI + "::" + opts.FuncName
	seen[startKey] = true
	queue := []node{{uri: opts.DocURI, funcName: opts.FuncName}}

	for depth := 0; depth < maxDepth && len(queue) > 0; depth++ {
		var next []node
		for _, current := range queue {
			currentLabel := filepath.Base(strings.TrimPrefix(current.uri, "file://"))
			if current.funcName != "" {
				currentLabel += "::" + current.funcName
			}
			refs := refsForScope(opts.Index, uri.URI(current.uri), current.funcName)
			for _, ref := range refs {
				if ref.Component == "" {
					continue
				}
				edges = append(edges, graph.Edge{From: currentLabel, To: ref.Component})
				resolved := opts.Resolver.ComponentPath(ref.Component, baseDir)
				if resolved != "" {
					targetURI := "file://" + resolved
					targetKey := targetURI + "::"
					if !seen[targetKey] {
						seen[targetKey] = true
						next = append(next, node{uri: targetURI})
					}
				}
			}
		}
		queue = next
	}

	return Result{Graph: graph.Graph{Direction: "LR", Edges: edges}}
}

// refsForScope returns component refs, optionally filtered to a function scope.
func refsForScope(idx Index, fileURI uri.URI, funcName string) []*cfparser.ComponentRef {
	allRefs := idx.RefsForFile(fileURI)
	if funcName == "" {
		return allRefs
	}
	funcs := idx.FunctionsForFile(fileURI)
	var startLine, endLine uint32
	for _, f := range funcs {
		if strings.EqualFold(f.Name, funcName) {
			startLine = f.Line
			endLine = startLine + 500
			break
		}
	}
	if startLine == 0 && endLine == 0 {
		return allRefs
	}
	for _, f := range funcs {
		if f.Line > startLine && f.Line < endLine {
			endLine = f.Line
		}
	}
	var scoped []*cfparser.ComponentRef
	for _, ref := range allRefs {
		if ref.Line >= startLine && ref.Line < endLine {
			scoped = append(scoped, ref)
		}
	}
	return scoped
}
