package deps

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/uri"
)

// dirResolver resolves a component relative to the directory it is asked about,
// and only when the file exists. The resolver in deps_test.go ignores baseDir
// entirely, which is why it could not see this.
type dirResolver struct{}

func (dirResolver) ComponentPath(component, baseDir string) string {
	p := filepath.Join(baseDir, strings.ReplaceAll(component, ".", string(filepath.Separator))+".cfc")
	if _, err := os.Stat(p); err != nil {
		return ""
	}

	return p
}

// The traversal used to carry the starting file's directory to every depth, so
// a dependency one hop out in another directory was resolved as though it sat
// beside the file the graph started from — it came back unresolved and was
// drawn as a dashed edge.
func TestTransitiveDepsResolveAgainstTheirOwnDirectory(t *testing.T) {
	root := t.TempDir()

	pkg := filepath.Join(root, "pkg")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}

	write := func(path, body string) {
		t.Helper()

		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Start calls pkg.Mid; Mid calls Deep, which sits beside Mid, not beside
	// Start — so resolving Deep needs Mid's directory, not Start's.
	write(filepath.Join(root, "Start.cfc"),
		"component {\n    variables.mid = new pkg.Mid();\n\n    public void function Go() {\n        VARIABLES.mid.Run();\n    }\n}\n")
	write(filepath.Join(pkg, "Mid.cfc"),
		"component {\n    variables.deep = new Deep();\n\n    public void function Run() {\n        VARIABLES.deep.Work();\n    }\n}\n")
	write(filepath.Join(pkg, "Deep.cfc"),
		"component {\n    public void function Work() {\n        return;\n    }\n}\n")

	idx := index.New()

	for _, p := range []string{
		filepath.Join(root, "Start.cfc"),
		filepath.Join(pkg, "Mid.cfc"),
		filepath.Join(pkg, "Deep.cfc"),
	} {
		content, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}

		pr := parser.ParseWithOptions(uri.File(p), string(content), parser.ParseOptions{ExtractCalls: true})
		idx.IndexFileFromResult(pr.URI, pr.Funcs, pr.ComponentRefs)
	}

	startContent, err := os.ReadFile(filepath.Join(root, "Start.cfc"))
	if err != nil {
		t.Fatal(err)
	}

	startPR := parser.ParseWithOptions(uri.File(filepath.Join(root, "Start.cfc")),
		string(startContent), parser.ParseOptions{ExtractCalls: true})

	// The refs path: file-level `new` refs carry a resolvable Component, which
	// is what the traversal follows between files.
	result := Build(Options{
		DocURI:   "file://" + filepath.Join(root, "Start.cfc"),
		Refs:     startPR.ComponentRefs,
		Index:    idx,
		Resolver: dirResolver{},
		MaxDepth: 5,
	})

	// buildFromRefs labels a resolved edge with the file's basename and an
	// unresolved one with the bare component name, so the label is the signal.
	var sawResolved, sawUnresolved bool

	for _, e := range result.Graph.Edges {
		switch {
		case strings.HasPrefix(e.To, "Deep.cfc"):
			sawResolved = true
		case strings.HasPrefix(e.To, "Deep "):
			sawUnresolved = true
		}
	}

	if sawUnresolved {
		t.Errorf("Deep resolved against the wrong directory; edges: %+v", result.Graph.Edges)
	}

	if !sawResolved {
		t.Errorf("no resolved edge reached Deep; edges: %+v", result.Graph.Edges)
	}
}
