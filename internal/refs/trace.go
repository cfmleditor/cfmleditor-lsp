package refs

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/graph"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

// TraceResult holds the output of a recursive reference trace.
type TraceResult struct {
	Entries []Entry
	Summary string
	Graph   graph.Graph
}

// Trace finds all callers of funcName, then recursively traces callers of
// wrapper functions up to maxDepth levels.
func Trace(fsys vfs.FS, roots []string, opts Options) []Entry {
	entries := Find(fsys, roots, opts)

	// Recursively trace callers that are inside a named function
	// Each recursive level verifies calls resolve to the file containing the wrapper
	recurseOpts := opts
	type funcRef struct {
		name string
		file string
	}
	tracedFuncs := make(map[string]bool)
	tracedFuncs[opts.SourceFile+"\t"+strings.ToLower(opts.FuncName)] = true
	for depth := 0; depth < 10; depth++ {
		var newFuncs []funcRef
		for _, e := range entries {
			key := e.File + "\t" + strings.ToLower(e.Function)
			if e.Function != "" && !tracedFuncs[key] {
				tracedFuncs[key] = true
				newFuncs = append(newFuncs, funcRef{name: e.Function, file: e.File})
			}
		}
		if len(newFuncs) == 0 {
			break
		}
		for _, fr := range newFuncs {
			recurseOpts.FuncName = fr.name
			recurseOpts.SourceFile = fr.file
			extra := Find(fsys, roots, recurseOpts)
			entries = append(entries, extra...)
		}
	}
	return entries
}

// FormatResult builds a summary and graph from trace entries.
func FormatResult(entries []Entry, funcName, sourceURI string, roots []string) TraceResult {
	// Summary
	var lines []string
	sourceRel := relativePath(strings.TrimPrefix(sourceURI, "file://"), roots)
	lines = append(lines, fmt.Sprintf("Calls to '%s' (%s): %d match(es)", funcName, sourceRel, len(entries)))
	for _, e := range entries {
		rel := relativePath(e.File, roots)
		marker := ""
		if !e.Resolved {
			marker = " [unresolved]"
		}
		lines = append(lines, fmt.Sprintf("  %s:%d%s  %s", rel, e.Line+1, marker, e.Call))
	}

	// Build graph edges
	sourceLabel := sourceRel + "::" + funcName
	sourceBase := strings.ToLower(strings.TrimSuffix(filepath.Base(sourceRel), filepath.Ext(sourceRel)))

	type nodeInfo struct {
		label, fileBase string
	}
	var nodes []nodeInfo
	seen := make(map[string]bool)
	for _, e := range entries {
		rel := relativePath(e.File, roots)
		label := fmt.Sprintf("%s:%d", rel, e.Line+1)
		if e.Function != "" {
			label += "::" + e.Function
		}
		if seen[label] {
			continue
		}
		seen[label] = true
		fileBase := strings.ToLower(strings.TrimSuffix(filepath.Base(e.File), filepath.Ext(e.File)))
		nodes = append(nodes, nodeInfo{label: label, fileBase: fileBase})
	}

	var edges []graph.Edge
	for i, e := range entries {
		if i >= len(nodes) {
			break
		}
		n := nodes[i]
		from := sourceLabel
		dashed := !e.Resolved
		if e.Component != "" {
			compBase := strings.ToLower(e.Component)
			if dotIdx := strings.LastIndexByte(compBase, '.'); dotIdx >= 0 {
				compBase = compBase[dotIdx+1:]
			}
			compBase = strings.TrimSuffix(compBase, ".cfc")
			if !strings.EqualFold(compBase, sourceBase) {
				for j, other := range nodes {
					if j == i {
						continue
					}
					if strings.EqualFold(compBase, other.fileBase) {
						from = other.label
						dashed = false
						break
					}
				}
			}
		}
		edges = append(edges, graph.Edge{From: from, To: n.label, Dashed: dashed})
	}

	return TraceResult{
		Entries: entries,
		Summary: strings.Join(lines, "\n"),
		Graph:   graph.Graph{Direction: "TD", Edges: edges},
	}
}

func relativePath(path string, roots []string) string {
	rel := path
	for _, root := range roots {
		if r, err := filepath.Rel(root, path); err == nil && len(r) < len(rel) {
			rel = r
		}
	}
	return rel
}
