package path

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

// fakeDirEntry is a minimal fs.DirEntry for caseSensitiveFS.
type fakeDirEntry struct{ name string }

func (e fakeDirEntry) Name() string               { return e.name }
func (e fakeDirEntry) IsDir() bool                { return false }
func (e fakeDirEntry) Type() fs.FileMode          { return 0 }
func (e fakeDirEntry) Info() (fs.FileInfo, error) { return nil, nil }

// caseSensitiveFS is a minimal in-memory vfs.FS that treats file/directory names as
// case-sensitive, regardless of the host OS. Real disks vary (APFS/NTFS are
// case-insensitive by default, ext4 is not), so this makes the case-insensitivity
// behavior of ResolvePath deterministic and platform-independent to test.
type caseSensitiveFS struct {
	// dirs maps an exact-case directory path to the exact-case entry names within it.
	dirs map[string][]string
}

func (f *caseSensitiveFS) ReadFile(string) ([]byte, error) { return nil, os.ErrNotExist }

func (f *caseSensitiveFS) Stat(path string) (fs.FileInfo, error) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	if slices.Contains(f.dirs[dir], base) {
		return nil, nil
	}

	return nil, os.ErrNotExist
}

func (f *caseSensitiveFS) ReadDir(path string) ([]fs.DirEntry, error) {
	entries, ok := f.dirs[path]
	if !ok {
		return nil, os.ErrNotExist
	}

	out := make([]fs.DirEntry, len(entries))
	for i, e := range entries {
		out[i] = fakeDirEntry{name: e}
	}

	return out, nil
}

func (f *caseSensitiveFS) Walk(string, filepath.WalkFunc) error { return nil }

var _ vfs.FS = (*caseSensitiveFS)(nil)

func TestResolvePath_Found(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "models")
	_ = os.MkdirAll(sub, 0o755)
	_ = os.WriteFile(filepath.Join(sub, "User.cfc"), []byte("component {}"), 0o644)

	got := ResolvePath("models.User", dir, nil)
	want := filepath.Join(sub, "User.cfc")

	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolvePath_NotFound(t *testing.T) {
	got := ResolvePath("no.Such", t.TempDir(), nil)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestResolvePath_Mapping(t *testing.T) {
	dir := t.TempDir()
	libDir := filepath.Join(dir, "lib")
	_ = os.MkdirAll(libDir, 0o755)
	_ = os.WriteFile(filepath.Join(libDir, "Helper.cfc"), []byte("component {}"), 0o644)

	mappings := map[string]string{"mylib": libDir}
	got := ResolvePath("mylib.Helper", t.TempDir(), mappings)
	want := filepath.Join(libDir, "Helper.cfc")

	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolvePath_MappingNestedPath(t *testing.T) {
	dir := t.TempDir()
	libDir := filepath.Join(dir, "lib", "sub")
	_ = os.MkdirAll(libDir, 0o755)
	_ = os.WriteFile(filepath.Join(libDir, "Thing.cfc"), []byte("component {}"), 0o644)

	mappings := map[string]string{"mylib": filepath.Join(dir, "lib")}
	got := ResolvePath("mylib.sub.Thing", t.TempDir(), mappings)
	want := filepath.Join(libDir, "Thing.cfc")

	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestResolvePath_CaseInsensitiveNestedSegments verifies that ResolvePath tolerates a case
// mismatch in every segment of a multi-level dot-path, not just the final filename — on a
// case-sensitive filesystem, a mismatch in an intermediate directory segment previously broke
// resolution entirely (the single whole-path Stat failed before any correction could run).
func TestResolvePath_CaseInsensitiveNestedSegments(t *testing.T) {
	orig := DefaultFS
	defer func() { DefaultFS = orig }()

	DefaultFS = &caseSensitiveFS{
		dirs: map[string][]string{
			"/root":            {"Models"},
			"/root/Models":     {"Sub"},
			"/root/Models/Sub": {"Thing.cfc"},
		},
	}

	got := ResolvePath("models.sub.thing", "/root", nil)
	want := "/root/Models/Sub/Thing.cfc"

	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestResolvePath_CaseInsensitiveMappingKey verifies that a mappings key is matched
// case-insensitively, matching CFML's traditional case-insensitive component-path semantics.
func TestResolvePath_CaseInsensitiveMappingKey(t *testing.T) {
	orig := DefaultFS
	defer func() { DefaultFS = orig }()

	DefaultFS = &caseSensitiveFS{
		dirs: map[string][]string{
			"/lib": {"Helper.cfc"},
		},
	}

	mappings := map[string]string{"MyLib": "/lib"}
	got := ResolvePath("mylib.Helper", "/unused", mappings)
	want := "/lib/Helper.cfc"

	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
