package deps

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
)

// A call written `VARIABLES.svc.GetData()` is recorded with Variable
// "VARIABLES.svc", while the ref that identifies it is recorded under "svc" —
// refs are keyed by bare name and callers strip the scope, which is what
// resolve.CanResolveCall does via parser.StripReceiverScope. The deps traversal
// read call.Component directly instead, so it stayed empty: every edge came out
// dashed and the walk stopped at depth 0, never reaching the components behind
// the first hop.
func TestFunctionDepsFollowScopePrefixedReceivers(t *testing.T) {
	dir := testdataDir()
	idx := buildIndex(t, dir)
	controllerPath, _ := filepath.Abs(filepath.Join(dir, "controller.cfc"))
	pr := parseFile(t, filepath.Join(dir, "controller.cfc"))

	var calls []parser.CallSite

	for _, sc := range pr.Scopes {
		for _, f := range pr.Funcs {
			if strings.EqualFold(f.Name, "BuildReport") && int(f.Line) == sc.Start {
				calls = pr.FuncCalls(sc.Start, sc.End)
			}
		}
	}

	if len(calls) == 0 {
		t.Fatal("FuncCalls returned no calls for BuildReport")
	}

	// Refs is deliberately left nil, matching the only real caller: the
	// exportDeps command fills Refs *instead of* Calls, never alongside, so a
	// fix that depended on Options.Refs being populated here would pass its
	// test and do nothing in production.
	result := Build(Options{
		DocURI:   "file://" + controllerPath,
		FuncName: "BuildReport",
		Calls:    calls,
		Index:    idx,
		Resolver: &testResolver{dir: dir},
		MaxDepth: 10,
	})

	var dashed []string

	for _, e := range result.Graph.Edges {
		if e.Dashed {
			dashed = append(dashed, e.To)
		}
	}

	if len(dashed) > 0 {
		t.Errorf("edges left unresolved: %v\nall edges: %+v", dashed, result.Graph.Edges)
	}

	// Both edges must name the component they resolved to, not the raw receiver.
	for _, want := range []string{"service.cfc::GetData", "service.cfc::GetSummary"} {
		var found bool

		for _, e := range result.Graph.Edges {
			if strings.HasPrefix(e.To, want) {
				found = true
			}
		}

		if !found {
			t.Errorf("no edge labelled %q; edges: %+v", want, result.Graph.Edges)
		}
	}
}
