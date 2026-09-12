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
	_ = os.MkdirAll(child, 0o755)

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
	if cfg == nil {
		t.Fatal("expected config with directory name fallback, got nil")
	}

	expected := dir
	if cfg.Name != expected {
		t.Fatalf("expected name %q, got %q", expected, cfg.Name)
	}
}

// TestFindConfigNoFile pins the answer every caller already tests for. It used
// to be a Config with an empty Path and the base name of dir as its Name, which
// made "found nothing" look exactly like "found something" — and since the
// socket path is a hash of Name, it keyed the daemon on the base name of the
// working directory.
func TestFindConfigNoFile(t *testing.T) {
	cfg, err := FindConfig(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	if cfg != nil {
		t.Fatalf("expected nil with no config file, got %+v", cfg)
	}
}

// TestSocketPathDoesNotCollideOnFolderName is the harm that fallback did. Two
// unrelated projects in folders with the same name shared one daemon, and so
// one index — one project's symbols answering the other's go-to-definition.
// With no config there is now no daemon at all; with one, the socket is keyed
// on a name the project chose or on the config's own absolute directory.
func TestSocketPathDoesNotCollideOnFolderName(t *testing.T) {
	a := filepath.Join(t.TempDir(), "app")
	b := filepath.Join(t.TempDir(), "app")

	writeConfig(t, a, `{}`)
	writeConfig(t, b, `{}`)

	cfgA, err := FindConfig(a)
	if err != nil || cfgA == nil {
		t.Fatalf("FindConfig(a) = %v, %v", cfgA, err)
	}

	cfgB, err := FindConfig(b)
	if err != nil || cfgB == nil {
		t.Fatalf("FindConfig(b) = %v, %v", cfgB, err)
	}

	if cfgA.SocketPath() == cfgB.SocketPath() {
		t.Errorf("two projects in folders both called %q share a socket: %s",
			filepath.Base(a), cfgA.SocketPath())
	}
}

// TestSocketPathIsStableForOneProject is the other half: the same project must
// keep reaching the same daemon, whichever directory under it the server starts
// in, or every editor window gets an index of its own.
func TestSocketPathIsStableForOneProject(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, `{"workspaceName":"myproject"}`)

	nested := filepath.Join(root, "src", "models")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	top, err := FindConfig(root)
	if err != nil || top == nil {
		t.Fatalf("FindConfig(root) = %v, %v", top, err)
	}

	deep, err := FindConfig(nested)
	if err != nil || deep == nil {
		t.Fatalf("FindConfig(nested) = %v, %v", deep, err)
	}

	if top.SocketPath() != deep.SocketPath() {
		t.Errorf("the same project resolved to two sockets: %s and %s", top.SocketPath(), deep.SocketPath())
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
	sharedLib := filepath.Join(root, "shared-lib")
	_ = os.MkdirAll(sharedLib, 0o755)

	dir := filepath.Join(root, "project")
	writeConfig(t, dir, `{"workspaceName":"proj","workspacePaths":["../shared-lib","."]}`)
	cfg := &Config{Path: filepath.Join(dir, ".cfmleditor.json"), Name: "proj"}
	folders := cfg.WorkspaceFolders()

	if len(folders) != 2 {
		t.Fatalf("expected 2 folders, got %d: %v", len(folders), folders)
	}

	if folders[0] != sharedLib {
		t.Fatalf("got %q, want %q", folders[0], sharedLib)
	}

	if folders[1] != dir {
		t.Fatalf("got %q, want %q", folders[1], dir)
	}
}

func TestIndexGlobsResolvesBaseName(t *testing.T) {
	root := t.TempDir()
	sharedLib := filepath.Join(root, "shared-lib")
	_ = os.MkdirAll(sharedLib, 0o755)

	dir := filepath.Join(root, "project")
	writeConfig(t, dir, `{
		"workspaceName":"proj",
		"workspacePaths":["../shared-lib"],
		"workspaceIndexGlobs":["shared-lib/**/*.cfc"]
	}`)

	cfg := &Config{Path: filepath.Join(dir, ".cfmleditor.json"), Name: "proj"}
	globs := cfg.IndexGlobs()

	if len(globs) != 1 {
		t.Fatalf("expected 1 glob, got %d: %v", len(globs), globs)
	}

	expected := sharedLib + "/**/*.cfc"
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
	_ = os.MkdirAll(sub, 0o755)
	_ = os.WriteFile(filepath.Join(root, "Top.cfc"), []byte(""), 0o644)
	_ = os.WriteFile(filepath.Join(sub, "Deep.cfc"), []byte(""), 0o644)
	_ = os.WriteFile(filepath.Join(sub, "skip.txt"), []byte(""), 0o644)

	matches := expandGlob(root + "/**/*.cfc")
	sort.Strings(matches)

	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d: %v", len(matches), matches)
	}
}

func TestExpandGlobParentRefDoubleStar(t *testing.T) {
	root := t.TempDir()
	subDir := filepath.Join(root, "shared-lib", "sub")
	_ = os.MkdirAll(subDir, 0o755)
	_ = os.WriteFile(filepath.Join(root, "shared-lib", "Root.cfc"), []byte(""), 0o644)
	_ = os.WriteFile(filepath.Join(subDir, "Nested.cfc"), []byte(""), 0o644)

	pattern := filepath.Join(root, "shared-lib") + "/**/*.cfc"
	matches := expandGlob(pattern)

	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d: %v", len(matches), matches)
	}
}
