package codemap_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/codemap"
	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	"github.com/cfmleditor/cfmleditor-lsp/internal/resolve"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

// The same workspace gives the same map. ContentBox keeps a RailoDBInfo.cfc in
// several patch directories, and a patch that has none calls `new
// RailoDBInfo()`: every copy is equally near, and the edge went to whichever
// the index — filled by a parallel scan — listed first, so it moved from run to
// run and the island numbering moved with it. Ten copies and ten builds make
// a lucky pass impossible.
func TestTheSameWorkspaceGivesTheSameMap(t *testing.T) {
	root := t.TempDir()

	write := func(rel, src string) string {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(p, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}

		return p
	}

	var files []string

	for i := range 10 {
		files = append(files, write(fmt.Sprintf("patches/p%d/Widget.cfc", i), "component {\n\tfunction run() {}\n}\n"))
	}

	files = append(files, write("patches/zz/Caller.cfc", "component {\n\tfunction go() {\n\t\treturn new Widget().run();\n\t}\n}\n"))

	for range 10 {
		fsys := vfs.OS{}
		m := codemap.Build(&codemap.Options{
			Root: root, Files: files, FS: fsys, Workers: 4,
			Resolver: &resolve.Resolver{FS: fsys, Index: index.New(), WorkspaceFolders: []string{root}},
		})

		var to []string

		for _, e := range m.Edges {
			if e.From == "patches/zz/Caller.cfc::go" && e.Kind == codemap.EdgeCalls {
				to = append(to, e.To)
			}
		}

		if len(to) != 1 || to[0] != "patches/p0/Widget.cfc::run" {
			t.Fatalf("go's call edges: got %v, want [patches/p0/Widget.cfc::run]", to)
		}
	}
}
