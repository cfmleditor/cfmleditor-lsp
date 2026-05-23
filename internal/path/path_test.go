package path

import (
	"os"
	"path/filepath"
	"testing"
)

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
