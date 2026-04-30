package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".cfmleditor.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFindConfigInDir(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `{"name":"myproject"}`)

	cfg, err := FindConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		t.Fatal("expected config, got nil")
	}
	if cfg.Name != "myproject" {
		t.Fatalf("got name %q, want %q", cfg.Name, "myproject")
	}
}

func TestFindConfigInParent(t *testing.T) {
	parent := t.TempDir()
	child := filepath.Join(parent, "sub")
	os.MkdirAll(child, 0o755)
	writeConfig(t, parent, `{"name":"parentproj"}`)

	cfg, err := FindConfig(child)
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil || cfg.Name != "parentproj" {
		t.Fatalf("expected parentproj, got %+v", cfg)
	}
}

func TestFindConfigMissingName(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `{"sharedIndexPaths":["lib"]}`)

	cfg, err := FindConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg != nil {
		t.Fatalf("expected nil config when name is empty, got %+v", cfg)
	}
}

func TestFindConfigNoFile(t *testing.T) {
	dir := t.TempDir()
	cfg, err := FindConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg != nil {
		t.Fatalf("expected nil, got %+v", cfg)
	}
}

func TestSocketPathDerivedFromName(t *testing.T) {
	a := &Config{Name: "alpha"}
	b := &Config{Name: "beta"}
	c := &Config{Name: "alpha"}

	if a.SocketPath() == b.SocketPath() {
		t.Fatal("different names should produce different socket paths")
	}
	if a.SocketPath() != c.SocketPath() {
		t.Fatal("same name should produce same socket path")
	}
}

func TestSharedIndexPathsRelative(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `{"name":"proj","sharedIndexPaths":["lib","../shared"]}`)

	cfg := &Config{Path: filepath.Join(dir, ".cfmleditor.json"), Name: "proj"}
	paths := cfg.SharedIndexPaths()

	if len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(paths))
	}
	if paths[0] != filepath.Join(dir, "lib") {
		t.Fatalf("got %q, want %q", paths[0], filepath.Join(dir, "lib"))
	}
	expected := filepath.Join(filepath.Dir(dir), "shared")
	if paths[1] != expected {
		t.Fatalf("got %q, want %q", paths[1], expected)
	}
}
