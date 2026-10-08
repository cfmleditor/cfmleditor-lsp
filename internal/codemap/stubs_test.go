package codemap_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cfmleditor/clif/internal/codemap"
	"github.com/cfmleditor/clif/internal/config"
	"github.com/cfmleditor/clif/internal/frameworkapi"
	"github.com/cfmleditor/clif/internal/index"
	"github.com/cfmleditor/clif/internal/parser"
	"github.com/cfmleditor/clif/internal/resolve"
	"github.com/cfmleditor/clif/internal/vfs"
)

// TestFrameworkStubsAreNotPartOfTheMap: a call into a framework's bundled
// API resolves, but the stub is not code in the workspace, so the map has no
// file or function for it — as it had none when the framework was simply
// missing. With unresolved calls shown, the call is an external node, as it
// was then; resolved into the stub, its edge pointed at a node no scan makes
// and was dropped.
func TestFrameworkStubsAreNotPartOfTheMap(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "handlers", "Main.cfc")

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	src := `component extends="coldbox.system.EventHandler" {
	function index( event ) {
		var d = new coldbox.system.async.time.Duration();
		event.getValue( "x" );
		getInstance( "UserService" );
		helper();
	}
	function helper() {}
}`
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}

	frameworks := []string{"coldbox"}

	var resolvers []parser.Resolver
	for _, r := range config.FrameworkResolvers(frameworks) {
		resolvers = append(resolvers, r.Parser())
	}

	for _, unresolved := range []bool{false, true} {
		fsys := vfs.OS{}
		m := codemap.Build(&codemap.Options{
			Root: root, Files: []string{path}, FS: fsys, Workers: 1, Resolvers: resolvers,
			IncludeUnresolved: unresolved,
			Resolver: &resolve.Resolver{
				FS: fsys, Index: index.New(), WorkspaceFolders: []string{root}, Resolvers: resolvers,
				ImplicitExtends: config.ImplicitExtends(frameworks), Stubs: frameworkapi.For(frameworks),
			},
		})

		calls, external := 0, 0

		for i := range m.Nodes {
			n := &m.Nodes[i]
			if strings.Contains(n.File, "__cfmleditor_frameworks__") || strings.Contains(n.ID, "__cfmleditor_frameworks__") {
				t.Errorf("unresolved=%v: the map holds a stub: %s", unresolved, n.ID)
			}

			if n.Kind == codemap.KindExternal && strings.HasPrefix(n.Name, "coldbox.system.") {
				external++
			}
		}

		for i := range m.Edges {
			if m.Edges[i].Kind == codemap.EdgeCalls {
				calls++
			}
		}

		if calls == 0 {
			t.Errorf("unresolved=%v: no call edges at all; the fixture is not exercising the map", unresolved)
		}

		if want := map[bool]int{false: 0, true: 2}[unresolved]; external != want {
			t.Errorf("unresolved=%v: %d external ColdBox nodes, want %d (getValue and getInstance)", unresolved, external, want)
		}
	}
}
