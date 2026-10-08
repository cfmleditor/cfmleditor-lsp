package resolve

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/cfmleditor/clif/internal/index"
	"github.com/cfmleditor/clif/internal/parser"
	"github.com/cfmleditor/clif/internal/vfs"
	"go.lsp.dev/uri"
)

// A bare component name that no path answers is looked up by file name across
// the index, and several files can match. The nearest wins, and among equally
// near ones the lowest path — as Index.LookupPreferred decides for a function.
// ContentBox keeps a RailoDBInfo.cfc in each of several patch directories, all
// equally near a sibling patch that has none, and taking whichever the index
// listed first changed the answer, and every code-map edge through it, from
// run to run.
//
// Each run builds a fresh resolver, whose path cache would otherwise answer
// the second from the first, and ten candidates make a lucky pass one in ten
// per run, so twenty runs cannot pass by chance.
func TestBareComponentNamePicksTheNearestThenTheLowestPath(t *testing.T) {
	build := func(nearer bool) *index.Index {
		idx := index.New()

		for i := 9; i >= 0; i-- {
			u := uri.URI(fmt.Sprintf("file:///proj/patches/d%d/Widget.cfc", i))
			idx.IndexFileFromResult(u, []parser.FunctionDef{{Name: "run", URI: u, Line: 1}}, nil)
		}

		if nearer {
			u := uri.URI("file:///proj/patches/zz/sub/Widget.cfc")
			idx.IndexFileFromResult(u, []parser.FunctionDef{{Name: "run", URI: u, Line: 1}}, nil)
		}

		return idx
	}

	for _, c := range []struct {
		name   string
		nearer bool
		want   string
	}{
		{"equally near: the lowest path", false, "/proj/patches/d0/Widget.cfc"},
		{"one nearer: the nearest", true, "/proj/patches/zz/sub/Widget.cfc"},
	} {
		for range 20 {
			r := &Resolver{FS: vfs.OS{}, Index: build(c.nearer)}

			if got := r.ComponentPath("Widget", "/proj/patches/zz"); got != c.want {
				t.Fatalf("%s: got %q, want %q", c.name, got, c.want)
			}
		}
	}
}

// ComponentPath applies expressionMappings too, and in the same order as the
// parser: with both files on disk, map order decided which one a path named.
func TestComponentPathAppliesExpressionMappingsLongestFirst(t *testing.T) {
	dir := t.TempDir()

	for _, p := range []string{"old/Widget.cfc", "core/legacy/Widget.cfc"} {
		full := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(full, []byte("component {}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	want := filepath.Join(dir, "old", "Widget.cfc")

	for range 30 {
		r := &Resolver{
			FS:       vfs.OS{},
			Index:    index.New(),
			Mappings: map[string]string{"pkg": dir},
			ExpressionMappings: map[string]string{
				"#core#":        "pkg.core.",
				"#core#legacy.": "pkg.old.",
			},
		}

		if got := r.ComponentPath("#core#legacy.Widget", dir); got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	}
}
