package deps

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	"go.lsp.dev/uri"
)

func testdataDir() string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(f), "..", "..", "testdata", "deps")
}

type testResolver struct{ dir string }

func (r *testResolver) ComponentPath(component, _ string) string {
	name := component
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		name = name[idx+1:]
	}
	abs, _ := filepath.Abs(filepath.Join(r.dir, name+".cfc"))
	return abs
}

func parseFile(t *testing.T, path string) *parser.ParseResult {
	t.Helper()
	absPath, _ := filepath.Abs(path)
	content, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return parser.ParseWithOptions(uri.URI("file://"+absPath), string(content), parser.ParseOptions{})
}

func buildIndex(t *testing.T, dir string) *index.Index {
	t.Helper()
	idx := index.New()
	for _, name := range []string{"controller.cfc", "service.cfc", "persist.cfc"} {
		pr := parseFile(t, filepath.Join(dir, name))
		idx.IndexFileFromResult(pr.URI, pr.Funcs, pr.Refs)
	}
	return idx
}

// TestBuild_FunctionDeps verifies that BuildReport's graph labels correctly
// and includes transitive deps through service to persist.
func TestBuild_FunctionDeps(t *testing.T) {
	dir := testdataDir()
	idx := buildIndex(t, dir)
	controllerPath, _ := filepath.Abs(filepath.Join(dir, "controller.cfc"))
	pr := parseFile(t, filepath.Join(dir, "controller.cfc"))

	// Get function-level calls using FuncCalls
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
		DocURI:   "file://" + controllerPath,
		FuncName: "BuildReport",
		Calls:    calls,
		Index:    idx,
		Resolver: &testResolver{dir: dir},
		MaxDepth: 10,
	})

	edges := result.Graph.Edges
	foundGetData := false
	foundGetSummary := false
	for _, e := range edges {
		if strings.Contains(e.To, "GetData") {
			foundGetData = true
		}
		if strings.Contains(e.To, "GetSummary") {
			foundGetSummary = true
		}
		if strings.Contains(e.To, "PurgeCache") {
			t.Errorf("BuildReport should not depend on PurgeCache: %s -> %s", e.From, e.To)
		}
	}
	if !foundGetData {
		t.Errorf("expected dependency on service.GetData")
	}
	if !foundGetSummary {
		t.Errorf("expected dependency on service.GetSummary")
	}
	if t.Failed() {
		for _, e := range edges {
			t.Logf("  %s -> %s", e.From, e.To)
		}
	}
}

// TestBuild_FileDeps verifies full file deps include service and persist transitively.
func TestBuild_FileDeps(t *testing.T) {
	dir := testdataDir()
	idx := buildIndex(t, dir)
	controllerPath, _ := filepath.Abs(filepath.Join(dir, "controller.cfc"))
	pr := parseFile(t, filepath.Join(dir, "controller.cfc"))

	// All file-level refs
	refs := make([]parser.ComponentRef, len(pr.Refs))
	copy(refs, pr.Refs)

	result := Build(Options{
		DocURI:   "file://" + controllerPath,
		Refs:     refs,
		Index:    idx,
		Resolver: &testResolver{dir: dir},
		MaxDepth: 10,
	})

	edges := result.Graph.Edges
	foundService := false
	foundPersist := false
	for _, e := range edges {
		if strings.Contains(e.To, "service") {
			foundService = true
		}
		if strings.Contains(e.To, "persist") {
			foundPersist = true
		}
	}
	if !foundService {
		t.Errorf("expected direct dependency on service")
	}
	if !foundPersist {
		t.Errorf("expected transitive dependency on persist")
	}
	if t.Failed() {
		for _, e := range edges {
			t.Logf("  %s -> %s", e.From, e.To)
		}
	}
}

// TestBuild_CleanupDeps verifies Cleanup's graph includes service and persist.
func TestBuild_CleanupDeps(t *testing.T) {
	dir := testdataDir()
	idx := buildIndex(t, dir)
	controllerPath, _ := filepath.Abs(filepath.Join(dir, "controller.cfc"))
	pr := parseFile(t, filepath.Join(dir, "controller.cfc"))

	refs := make([]parser.ComponentRef, len(pr.Refs))
	copy(refs, pr.Refs)

	result := Build(Options{
		DocURI:   "file://" + controllerPath,
		FuncName: "Cleanup",
		Refs:     refs,
		Index:    idx,
		Resolver: &testResolver{dir: dir},
		MaxDepth: 10,
	})

	edges := result.Graph.Edges
	foundService := false
	for _, e := range edges {
		if strings.Contains(e.To, "service") {
			foundService = true
		}
	}
	if !foundService {
		t.Errorf("expected dependency on service")
		for _, e := range edges {
			t.Logf("  %s -> %s", e.From, e.To)
		}
	}
}
