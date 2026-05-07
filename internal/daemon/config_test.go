package daemon

import (
	"os"
	"path/filepath"
	"sort"
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
	writeConfig(t, dir, `{"workspaceName":"myproject"}`)

	cfg, err := FindConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil || cfg.Name != "myproject" {
		t.Fatalf("expected myproject, got %+v", cfg)
	}
}

func TestFindConfigInParent(t *testing.T) {
	parent := t.TempDir()
	child := filepath.Join(parent, "sub")
	os.MkdirAll(child, 0o755)
	writeConfig(t, parent, `{"workspaceName":"parentproj"}`)

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
	writeConfig(t, dir, `{"workspacePaths":["lib"]}`)

	cfg, _ := FindConfig(dir)
	if cfg != nil {
		t.Fatalf("expected nil when name missing, got %+v", cfg)
	}
}

func TestFindConfigNoFile(t *testing.T) {
	cfg, _ := FindConfig(t.TempDir())
	if cfg != nil {
		t.Fatalf("expected nil, got %+v", cfg)
	}
}

func TestSocketPathDerivedFromName(t *testing.T) {
	a := &Config{Name: "alpha"}
	b := &Config{Name: "beta"}
	if a.SocketPath() == b.SocketPath() {
		t.Fatal("different names should produce different socket paths")
	}
	if a.SocketPath() != (&Config{Name: "alpha"}).SocketPath() {
		t.Fatal("same name should produce same socket path")
	}
}

func TestWorkspaceFolders(t *testing.T) {
	root := t.TempDir()
	tassweb := filepath.Join(root, "tassweb")
	os.MkdirAll(tassweb, 0o755)

	dir := filepath.Join(root, "project")
	writeConfig(t, dir, `{"workspaceName":"proj","workspacePaths":["../tassweb","."]}`)
	cfg := &Config{Path: filepath.Join(dir, ".cfmleditor.json"), Name: "proj"}
	folders := cfg.WorkspaceFolders()

	if len(folders) != 2 {
		t.Fatalf("expected 2 folders, got %d: %v", len(folders), folders)
	}
	if folders[0] != tassweb {
		t.Fatalf("got %q, want %q", folders[0], tassweb)
	}
	if folders[1] != dir {
		t.Fatalf("got %q, want %q", folders[1], dir)
	}
}

func TestIndexGlobsResolvesBaseName(t *testing.T) {
	root := t.TempDir()
	tassweb := filepath.Join(root, "tassweb")
	os.MkdirAll(tassweb, 0o755)

	dir := filepath.Join(root, "project")
	writeConfig(t, dir, `{
		"workspaceName":"proj",
		"workspacePaths":["../tassweb"],
		"workspaceIndexGlobs":["tassweb/**/*.cfc"]
	}`)
	cfg := &Config{Path: filepath.Join(dir, ".cfmleditor.json"), Name: "proj"}
	globs := cfg.IndexGlobs()

	if len(globs) != 1 {
		t.Fatalf("expected 1 glob, got %d: %v", len(globs), globs)
	}
	expected := tassweb + "/**/*.cfc"
	if globs[0] != expected {
		t.Fatalf("got %q, want %q", globs[0], expected)
	}
}

func TestIndexGlobsNilWhenNotDefined(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `{"workspaceName":"proj","workspacePaths":["."]}`)
	cfg := &Config{Path: filepath.Join(dir, ".cfmleditor.json"), Name: "proj"}

	if globs := cfg.IndexGlobs(); globs != nil {
		t.Fatalf("expected nil, got %v", globs)
	}
}

func TestExpandGlobDoubleStar(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "models")
	os.MkdirAll(sub, 0o755)
	os.WriteFile(filepath.Join(root, "Top.cfc"), []byte(""), 0o644)
	os.WriteFile(filepath.Join(sub, "Deep.cfc"), []byte(""), 0o644)
	os.WriteFile(filepath.Join(sub, "skip.txt"), []byte(""), 0o644)

	matches := expandGlob(root + "/**/*.cfc")
	sort.Strings(matches)

	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d: %v", len(matches), matches)
	}
}

func TestExpandGlobParentRefDoubleStar(t *testing.T) {
	root := t.TempDir()
	tassweb := filepath.Join(root, "tassweb", "sub")
	os.MkdirAll(tassweb, 0o755)
	os.WriteFile(filepath.Join(root, "tassweb", "Root.cfc"), []byte(""), 0o644)
	os.WriteFile(filepath.Join(tassweb, "Nested.cfc"), []byte(""), 0o644)

	pattern := filepath.Join(root, "tassweb") + "/**/*.cfc"
	matches := expandGlob(pattern)

	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d: %v", len(matches), matches)
	}
}
