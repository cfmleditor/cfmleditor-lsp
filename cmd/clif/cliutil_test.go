package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, dir string, resolvers string) {
	t.Helper()

	content := `{"componentResolvers": [` + resolvers + `]}`
	if err := os.WriteFile(filepath.Join(dir, ".cfmleditor.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadResolversFromConfig_FoundInGivenDir(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `{"match": "getService(\"$1\")", "resolve": "services.$1", "prefix": "getService"}`)

	resolvers := loadResolversFromConfig([]string{dir})
	if len(resolvers) != 1 || resolvers[0].Prefix != "getService" {
		t.Fatalf("expected 1 resolver from %s, got %+v", dir, resolvers)
	}
}

func TestLoadResolversFromConfig_FileArgUsesItsDir(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `{"match": "a", "resolve": "b", "prefix": "a"}`)

	filePath := filepath.Join(dir, "some.cfc")
	if err := os.WriteFile(filePath, []byte("component {}"), 0o644); err != nil {
		t.Fatal(err)
	}

	resolvers := loadResolversFromConfig([]string{filePath})
	if len(resolvers) != 1 {
		t.Fatalf("expected the config next to the file's dir to be found, got %+v", resolvers)
	}
}

func TestLoadResolversFromConfig_FallsBackToParentDir(t *testing.T) {
	parent := t.TempDir()
	writeConfig(t, parent, `{"match": "a", "resolve": "b", "prefix": "a"}`)

	child := filepath.Join(parent, "sub")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}

	resolvers := loadResolversFromConfig([]string{child})
	if len(resolvers) != 1 {
		t.Fatalf("expected config to be found in parent dir when absent from %s, got %+v", child, resolvers)
	}
}

func TestLoadResolversFromConfig_FiltersIncompleteResolvers(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir,
		`{"match": "getService(\"$1\")", "resolve": "services.$1", "prefix": "getService"},`+
			`{"match": "", "resolve": "services.Foo", "prefix": "foo"},`+
			`{"match": "getFoo()", "resolve": "", "prefix": "getFoo"}`)

	resolvers := loadResolversFromConfig([]string{dir})
	if len(resolvers) != 1 {
		t.Fatalf("expected only the fully-populated resolver to survive, got %+v", resolvers)
	}
}

func TestLoadResolversFromConfig_MalformedJSONSkipped(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".cfmleditor.json"), []byte("{not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := loadResolversFromConfig([]string{dir}); got != nil {
		t.Errorf("expected nil for malformed JSON with no other candidate paths, got %+v", got)
	}
}

func TestLoadResolversFromConfig_NoConfigAnywhere(t *testing.T) {
	dir := t.TempDir()

	if got := loadResolversFromConfig([]string{dir}); got != nil {
		t.Errorf("expected nil when no .cfmleditor.json exists, got %+v", got)
	}
}

func TestLoadResolversFromConfig_ReturnsOnFirstMatchingPath(t *testing.T) {
	first := t.TempDir()
	writeConfig(t, first, `{"match": "a", "resolve": "first", "prefix": "a"}`)

	second := t.TempDir()
	writeConfig(t, second, `{"match": "b", "resolve": "second", "prefix": "b"}`)

	resolvers := loadResolversFromConfig([]string{first, second})
	if len(resolvers) != 1 || resolvers[0].Resolve != "first" {
		t.Fatalf("expected only the first path's config to be used, got %+v", resolvers)
	}
}
