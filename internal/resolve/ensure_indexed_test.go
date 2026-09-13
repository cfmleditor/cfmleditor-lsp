package resolve

import (
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

// countingFS counts ReadFile calls per path so a test can tell "read once and
// cached" from "read again every time".
type countingFS struct {
	vfs.FS

	mu    sync.Mutex
	reads map[string]int
}

func newCountingFS() *countingFS {
	return &countingFS{FS: vfs.OS{}, reads: make(map[string]int)}
}

func (c *countingFS) ReadFile(path string) ([]byte, error) {
	c.mu.Lock()
	c.reads[path]++
	c.mu.Unlock()

	return c.FS.ReadFile(path)
}

func (c *countingFS) Stat(path string) (fs.FileInfo, error) { return c.FS.Stat(path) }

func (c *countingFS) count(path string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.reads[path]
}

// TestEnsureIndexedReadsAFunctionlessCFCOnce pins the difference between "this
// file has no functions" and "this file has not been indexed".
//
// EnsureIndexed used to re-read from disk whenever FunctionsForFile came back
// empty, which is the correct test for an unseen file and the wrong one for a
// property-only bean or DTO: those index to zero functions legitimately, so
// every lookup against one re-read and re-parsed it, forever.
func TestEnsureIndexedReadsAFunctionlessCFCOnce(t *testing.T) {
	dir := t.TempDir()

	// A component with no functions at all. `property name="id";` will not do:
	// the parser synthesises getId/setId for it, so it is not functionless.
	beanPath := filepath.Join(dir, "Bean.cfc")
	if err := os.WriteFile(beanPath, []byte("component {\n\tthis.id = 0;\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	withFuncsPath := filepath.Join(dir, "Svc.cfc")
	if err := os.WriteFile(withFuncsPath, []byte("component {\n\tfunction go() {}\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfs := newCountingFS()
	r := &Resolver{FS: cfs, Index: index.New()}

	for range 3 {
		if defs := r.EnsureIndexed(beanPath); len(defs) != 0 {
			t.Fatalf("Bean.cfc: got %d functions, want 0", len(defs))
		}
	}

	if got := cfs.count(beanPath); got != 1 {
		t.Errorf("Bean.cfc read %d times, want 1 — a functionless CFC is being re-read on every lookup", got)
	}

	// The control: a CFC that does declare functions was already cached, and
	// must stay that way.
	for range 3 {
		if defs := r.EnsureIndexed(withFuncsPath); len(defs) != 1 {
			t.Fatalf("Svc.cfc: got %d functions, want 1", len(defs))
		}
	}

	if got := cfs.count(withFuncsPath); got != 1 {
		t.Errorf("Svc.cfc read %d times, want 1", got)
	}
}
