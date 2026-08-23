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

	// LoadCalls returns the calls made inside funcName in the given file, and
	// the refs that resolve those calls' receivers. It is what lets
	// buildFromCalls walk past the first hop: the Index stores definitions and
	// refs, never call sites, so there is nothing there to continue from.
	//
	// Refs come back alongside the calls rather than being read from the Index
	// because a receiver may be a function-local `var x = new Foo()`, which is
	// not a file-level ComponentRef and so never reaches the index at all.
	//
	// Leaving it nil keeps the previous behaviour: one hop, then stop.
	LoadCalls func(fileURI uri.URI, funcName string) ([]parser.CallSite, []parser.ComponentRef)

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
		refs    []parser.ComponentRef
		baseDir string
	}

	var edges []graph.Edge

	// The refs for the file the calls came from, used to resolve a scope-
	// qualified receiver. Options.Refs cannot be relied on here: its only
	// caller fills it *instead of* Calls, never alongside, so it is empty in
	// exactly the case this function runs. Reading them from the index keeps
	// the resolution working regardless of how the caller populated Options.
	startRefs := opts.Refs
	if len(startRefs) == 0 && opts.Index != nil {
		startRefs = derefRefs(opts.Index.RefsForFile(uri.URI(opts.DocURI)))
	}

	queue := []node{{label: startLabel, calls: opts.Calls, refs: startRefs, baseDir: baseDir}}

	for depth := 0; depth < maxDepth && len(queue) > 0; depth++ {
		var next []node

		for _, current := range queue {
			for _, call := range current.calls {
				if call.Component == "" && call.Variable == "" {
					continue
				}

				resolved := ""
				toLabel := call.FuncName

				component := call.Component
				if component == "" && call.Variable != "" {
					component = componentForReceiver(call.Variable, current.refs)
				}

				if component != "" {
					resolved = opts.Resolver.ComponentPath(component, current.baseDir)
					toLabel = component + "." + call.FuncName

					if resolved != "" {
						toLabel = filepath.Base(resolved) + "::" + call.FuncName
					}
				} else if call.Variable != "" {
					toLabel = call.Variable + "." + call.FuncName
				}

				// The node the edge points at and the node any onward edge
				// starts from have to be the same string, or the rendered graph
				// is a set of disconnected pairs instead of a path — the target
				// carried a " (line N)" suffix that the queued child's label
				// did not.
				target := fmt.Sprintf("%s (line %d)", toLabel, call.Line)

				edges = append(edges, graph.Edge{
					From:   current.label,
					To:     target,
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

				targetCalls, targetRefs := getFuncCalls(opts, targetURI, call.FuncName)
				if len(targetCalls) == 0 {
					continue
				}

				if len(targetRefs) == 0 && opts.Index != nil {
					targetRefs = derefRefs(opts.Index.RefsForFile(targetURI))
				}

				next = append(next, node{
					label:   target,
					calls:   targetCalls,
					refs:    targetRefs,
					baseDir: filepath.Dir(resolved),
				})
			}
		}

		queue = next
	}

	return edges
}

// componentForReceiver resolves a call's receiver to a component using the refs
// of the file the call lives in.
//
// The two are recorded under different keys on purpose: a receiver written
// `VARIABLES.svc` is stored on the call as "VARIABLES.svc", while the ref that
// identifies it is stored as "svc". Refs are keyed by bare name and it is the
// caller's job to strip the scope, which is what resolve.CanResolveCall does
// through the same helper. Reading call.Component directly — as this traversal
// did — sees an empty string for every scope-qualified receiver, which is how
// most CFML is written, so every edge came out dashed and the walk stopped at
// depth 0.
func componentForReceiver(variable string, refs []parser.ComponentRef) string {
	lookup := parser.StripReceiverScope(variable)
	if lookup == "" {
		return ""
	}

	for i := range refs {
		if strings.EqualFold(refs[i].Variable, lookup) {
			return refs[i].Component
		}
	}

	return ""
}

func derefRefs(ptrs []*parser.ComponentRef) []parser.ComponentRef {
	out := make([]parser.ComponentRef, 0, len(ptrs))
	for _, p := range ptrs {
		out = append(out, *p)
	}

	return out
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

				// Same node-identity requirement as in buildFromCalls.
				target := fmt.Sprintf("%s (line %d)", toLabel, ref.Line)

				edges = append(edges, graph.Edge{
					From: current.label,
					To:   target,
				})

				if resolved == "" {
					continue
				}

				targetURI := "file://" + resolved
				if seen[targetURI] {
					continue
				}

				seen[targetURI] = true

				next = append(next, node{
					label:   target,
					refs:    derefRefs(opts.Index.RefsForFile(uri.URI(targetURI))),
					baseDir: filepath.Dir(resolved),
				})
			}
		}

		queue = next
	}

	return edges
}

// getFuncCalls returns the calls funcName makes inside the target file, and the
// refs that resolve their receivers, by asking the caller. Without a LoadCalls
// hook it returns nothing and the traversal stops after one hop — which is what
// it always did, because the Index it used to consult holds definitions and
// refs but no call sites.
//
// File-level tracing through buildFromRefs never had the limitation: refs *are*
// in the index, so it walks as far as MaxDepth on its own.
func getFuncCalls(opts Options, fileURI uri.URI, funcName string) ([]parser.CallSite, []parser.ComponentRef) {
	if opts.LoadCalls == nil {
		return nil, nil
	}

	return opts.LoadCalls(fileURI, funcName)
}
