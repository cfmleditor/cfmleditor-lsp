package deps

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/uri"
)

// Function-level tracing stopped at the first hop because getFuncCalls was a
// stub: the index stores definitions and refs, never call sites, so there was
// nowhere to read the next hop's calls from. The caller now supplies them.
func TestFunctionDepsTraceBeyondTheFirstHop(t *testing.T) {
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

	result := Build(Options{
		DocURI:    "file://" + controllerPath,
		FuncName:  "BuildReport",
		Calls:     calls,
		Index:     idx,
		Resolver:  &testResolver{dir: dir},
		LoadCalls: fileLoader(t),
		MaxDepth:  10,
	})

	var reachedPersist bool

	for _, e := range result.Graph.Edges {
		if strings.HasPrefix(e.To, "persist.cfc::") {
			reachedPersist = true
		}

		if e.Dashed {
			t.Errorf("unresolved edge %q; all edges: %+v", e.To, result.Graph.Edges)
		}
	}

	if !reachedPersist {
		t.Errorf("traversal stopped before persist.cfc; edges: %+v", result.Graph.Edges)
	}
}

// fileLoader reads and parses a target file the way the server's exportDeps
// command does, returning the calls a function makes and the refs that resolve
// their receivers.
func fileLoader(t *testing.T) func(uri.URI, string) ([]parser.CallSite, []parser.ComponentRef) {
	t.Helper()

	return func(fileURI uri.URI, funcName string) ([]parser.CallSite, []parser.ComponentRef) {
		path := strings.TrimPrefix(string(fileURI), "file://")

		pr := parseFile(t, path)
		for _, sc := range pr.Scopes {
			for _, f := range pr.Funcs {
				if strings.EqualFold(f.Name, funcName) && int(f.Line) == sc.Start {
					refs := append([]parser.ComponentRef{}, pr.ComponentRefs...)
					refs = append(refs, pr.FuncComponentRefs(sc.Start, sc.End)...)

					return pr.FuncCalls(sc.Start, sc.End), refs
				}
			}
		}

		return nil, nil
	}
}

// Now that the traversal actually recurses, a cycle has to terminate. Two
// components calling each other is ordinary CFML, not a pathological case.
func TestFunctionDepsTerminateOnACycle(t *testing.T) {
	dir := t.TempDir()

	write := func(name, body string) {
		t.Helper()

		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("a.cfc", "component {\n    variables.b = new b();\n\n    public void function Ping() {\n        VARIABLES.b.Pong();\n    }\n}\n")
	write("b.cfc", "component {\n    variables.a = new a();\n\n    public void function Pong() {\n        VARIABLES.a.Ping();\n    }\n}\n")

	aPath := filepath.Join(dir, "a.cfc")
	pr := parseFile(t, aPath)

	var calls []parser.CallSite

	for _, sc := range pr.Scopes {
		for _, f := range pr.Funcs {
			if strings.EqualFold(f.Name, "Ping") && int(f.Line) == sc.Start {
				calls = pr.FuncCalls(sc.Start, sc.End)
			}
		}
	}

	if len(calls) == 0 {
		t.Fatal("no calls extracted for Ping")
	}

	done := make(chan []string, 1)

	go func() {
		result := Build(Options{
			DocURI:    "file://" + aPath,
			FuncName:  "Ping",
			Calls:     calls,
			Index:     index.New(),
			Resolver:  &testResolver{dir: dir},
			LoadCalls: fileLoader(t),
			MaxDepth:  50,
		})

		var labels []string
		for _, e := range result.Graph.Edges {
			labels = append(labels, e.To)
		}

		done <- labels
	}()

	select {
	case labels := <-done:
		// a.Ping -> b.Pong -> a.Ping, then the cycle is cut.
		if len(labels) > 4 {
			t.Errorf("cycle produced %d edges, expected the walk to be cut: %v", len(labels), labels)
		}

		if len(labels) == 0 {
			t.Error("cycle produced no edges at all")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Build did not terminate on a cycle")
	}
}
