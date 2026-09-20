package resolve

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

// countingDirFS records what reaches the filesystem, which is the thing the
// directory cache is about — a resolution that is right and does five times the
// syscalls is still wrong for the workspace scan that makes 11,769 of them.
type countingDirFS struct {
	vfs.FS
	mu       sync.Mutex
	readDirs int
	entries  int
}

func (c *countingDirFS) ReadDir(path string) ([]fs.DirEntry, error) {
	e, err := c.FS.ReadDir(path)

	c.mu.Lock()
	c.readDirs++
	c.entries += len(e)
	c.mu.Unlock()

	return e, err
}

func (c *countingDirFS) counts() (int, int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.readDirs, c.entries
}

// dirCacheTree builds a workspace with a deep shared prefix and many leaves
// under it — the shape that makes the same parent directories get listed over
// and over, and the shape a real component tree has.
func dirCacheTree(t *testing.T, dirs, perDir int) (root string, paths []string) {
	t.Helper()

	root = t.TempDir()

	for d := range dirs {
		dir := filepath.Join(root, "app", "packages", "core", fmt.Sprintf("mod%d", d))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}

		for f := range perDir {
			name := fmt.Sprintf("Service%d.cfc", f)
			if err := os.WriteFile(filepath.Join(dir, name), []byte("component {}"), 0o644); err != nil {
				t.Fatal(err)
			}

			paths = append(paths, fmt.Sprintf("app.packages.core.mod%d.Service%d", d, f))
		}
	}

	return root, paths
}

func dirCacheResolver(root string) *Resolver {
	return &Resolver{
		FS:               vfs.OS{},
		WorkspaceFolders: []string{root},
		Index:            index.New(),
	}
}

// TestDirCacheListsEachDirectoryOnce is the cost test. Every one of these
// resolutions is a cache miss in resolveCache — distinct components — so what
// is left is the listing behind them, and the shared prefix is listed once per
// component without this.
func TestDirCacheListsEachDirectoryOnce(t *testing.T) {
	root, paths := dirCacheTree(t, 20, 20)

	counter := &countingDirFS{FS: vfs.OS{}}

	orig := cfpath.DefaultFS
	cfpath.DefaultFS = counter

	defer func() { cfpath.DefaultFS = orig }()

	r := dirCacheResolver(root)

	for _, p := range paths {
		if got := r.ComponentPath(p, root); got == "" {
			t.Fatalf("%s did not resolve", p)
		}
	}

	readDirs, entries := counter.counts()

	// The tree has 20 leaf directories plus app, packages, core and the root:
	// 24 distinct directories, and a listing apiece is the floor.
	t.Logf("%d components: %d ReadDir calls, %d entries listed", len(paths), readDirs, entries)

	if readDirs > 40 {
		t.Errorf("%d components took %d ReadDir calls over a tree of 24 directories — "+
			"the shared prefix is being listed per component", len(paths), readDirs)
	}
}

// The cache must not change an answer, only what it costs to get one. This
// resolves everything twice through one resolver and once through a resolver
// that has never seen the tree, and requires all three to agree — including on
// the paths that do not resolve, since a cached "not there" is the answer most
// likely to go wrong.
func TestDirCacheDoesNotChangeAnyAnswer(t *testing.T) {
	root, paths := dirCacheTree(t, 6, 6)

	paths = append(paths,
		"app.packages.core.mod0.Missing",
		"app.packages.core.nosuchmod.Service0",
		"app.nosuch.core.mod0.Service0",
		"nosuchroot.a.b",
		"APP.PACKAGES.CORE.MOD0.SERVICE0", // the case-insensitive walk
		"app.packages.core.mod0.service0",
	)

	warm := dirCacheResolver(root)

	for _, p := range paths {
		fresh := dirCacheResolver(root)

		want := fresh.ComponentPath(p, root)

		if got := warm.ComponentPath(p, root); got != want {
			t.Errorf("%s: warm resolver returned %q, a fresh one %q", p, got, want)
		}

		// Again on the warm one, now through resolveCache as well.
		if got := warm.ComponentPath(p, root); got != want {
			t.Errorf("%s: second warm call returned %q, want %q", p, got, want)
		}
	}
}

// A directory that could not be read is not cached: the failure is usually
// "not there yet", and a resolver kept past the file appearing would otherwise
// go on denying it. The server drops the resolver on a save or a watched-file
// change, so this only covers the window inside one resolver's life — but that
// window is where a newly written component gets its first lookup.
func TestDirCacheDoesNotRememberAMissingDirectory(t *testing.T) {
	root := t.TempDir()

	r := dirCacheResolver(root)

	if got := r.ComponentPath("late.Service", root); got != "" {
		t.Fatalf("resolved before the directory existed: %q", got)
	}

	dir := filepath.Join(root, "late")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "Service.cfc"), []byte("component {}"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A different component name, so resolveCache's own entry for the first
	// lookup is not what answers.
	if err := os.WriteFile(filepath.Join(dir, "Other.cfc"), []byte("component {}"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := r.ComponentPath("late.Other", root); got == "" {
		t.Error("a directory that did not exist at first lookup stayed unresolvable after it appeared")
	}
}
