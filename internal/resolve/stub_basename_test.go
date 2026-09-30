package resolve

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/frameworkapi"
	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

// Loading a qualified stub must not register its filename as a workspace
// component. The two orders reproduce the batch scan's FileSystem fluctuation
// deterministically, without relying on goroutine scheduling.
func TestBareComponentLookupDoesNotDependOnLoadedStubs(t *testing.T) {
	for _, tc := range []struct {
		name      string
		preset    bool
		workspace bool
		loadFirst bool
	}{
		{name: "no preset, bare first"},
		{name: "no preset, stub first", loadFirst: true},
		{name: "preset, bare first", preset: true},
		{name: "preset, stub first", preset: true, loadFirst: true},
		{name: "workspace, bare first", workspace: true},
		{name: "workspace, stub first", workspace: true, loadFirst: true},
		{name: "workspace and preset, bare first", workspace: true, preset: true},
		{name: "workspace and preset, stub first", workspace: true, preset: true, loadFirst: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			r := &Resolver{FS: vfs.OS{}, Index: index.New()}
			stub := frameworkapi.Namespaced("commandbox.system.util.FileSystem")
			if stub == "" {
				t.Fatal("FileSystem stub is missing")
			}

			var want string
			if tc.preset {
				r.Stubs = frameworkapi.For([]string{"commandbox"})
				want = stub
			}

			if tc.workspace {
				// Farther away than the virtual stub: excluding stubs must also
				// happen before selecting the nearest workspace candidate.
				want = filepath.Join(dir, "a", "b", "c", "d", "e", "f", "g", "h", "FileSystem.cfc")
				if err := os.MkdirAll(filepath.Dir(want), 0o750); err != nil {
					t.Fatal(err)
				}

				const src = "component { function workspaceOnly() {} }"
				if err := os.WriteFile(want, []byte(src), 0o600); err != nil {
					t.Fatal(err)
				}

				r.Index.IndexFile(cfpath.ToURI(want), src)
			}

			if tc.loadFirst {
				if defs := r.EnsureIndexed(stub); len(defs) == 0 {
					t.Fatal("FileSystem stub was not indexed")
				}
			}

			if got := r.ComponentPath("FileSystem", dir); got != want {
				t.Fatalf("bare FileSystem: got %q, want %q", got, want)
			}

			if got := r.ComponentPath("commandbox.system.util.FileSystem", dir); got != stub {
				t.Fatalf("qualified FileSystem: got %q, want %q", got, stub)
			}

			if defs := r.EnsureIndexed(stub); len(defs) == 0 {
				t.Fatal("FileSystem stub was not indexed")
			}

			// A fresh path cache, with the same now-populated index, checks that
			// caching a miss did not merely hide the changed lookup answer.
			fresh := &Resolver{FS: r.FS, Index: r.Index, Stubs: r.Stubs}
			if got := fresh.ComponentPath("FileSystem", dir); got != want {
				t.Fatalf("bare FileSystem after loading: got %q, want %q", got, want)
			}
		})
	}
}
