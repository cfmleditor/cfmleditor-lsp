package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"
)

// initializeWithConfig drives handleInitialize the way an editor does in
// standalone mode: a workspace root whose .cfmleditor.json is the only source
// of configuration.
func initializeWithConfig(t *testing.T, configJSON string) *Server {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".cfmleditor.json"), []byte(configJSON), 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	s := NewServer(nil, cflog.NewLogger(false))

	raw, err := json.Marshal(map[string]any{
		"processId":        nil,
		"rootUri":          "file://" + dir,
		"capabilities":     map[string]any{},
		"workspaceFolders": []map[string]any{{"uri": "file://" + dir, "name": "w"}},
	})
	if err != nil {
		t.Fatalf("marshalling params: %v", err)
	}

	if _, err := s.handleInitialize(context.Background(), raw); err != nil {
		t.Fatalf("handleInitialize: %v", err)
	}

	return s
}

// TestInitializeAppliesConfigBeforeSpawningWorkers pins the ordering inside
// handleInitialize: the workspace config must be applied before indexWorkspace
// and initLinter are spawned.
//
// initLinter returns early when it reads s.Linting as false, and nothing ever
// re-runs it, so losing that race left s.linter nil and diagnostics silently
// dead for the whole session. Spawning the goroutines only after the config
// writes also gives them a happens-before edge to those writes — without it
// this test reports a race under -race.
func TestInitializeAppliesConfigBeforeSpawningWorkers(t *testing.T) {
	s := initializeWithConfig(t, `{"linting":{"enabled":true}}`)

	if !s.Linting {
		t.Fatal("Linting not applied from workspace config by the time handleInitialize returned — " +
			"initLinter would have read it as false and left the linter nil")
	}
}

// TestInitializeAppliesFullConfigBeforeIndexing covers the same ordering for
// the settings indexWorkspace consumes: it used to start before mappings,
// resolvers, and bean paths were applied, so the initial index could run
// against empty configuration.
func TestInitializeAppliesFullConfigBeforeIndexing(t *testing.T) {
	s := initializeWithConfig(t, `{
		"mappings": {"models": "./src/models"},
		"beanPaths": {"svc": "./src/services"},
		"componentResolvers": [{"match": "getService(\"$1\")", "resolve": "services.$1", "prefix": "getService"}]
	}`)

	if len(s.Mappings) == 0 {
		t.Error("mappings not applied before indexWorkspace was spawned")
	}

	if len(s.BeanPaths) == 0 {
		t.Error("beanPaths not applied before indexWorkspace was spawned")
	}

	if len(s.ComponentResolvers) == 0 {
		t.Error("componentResolvers not applied before indexWorkspace was spawned")
	}
}

// initializeWithRoot drives handleInitialize against an existing directory,
// for cases where the config does not live in the root itself.
func initializeWithRoot(t *testing.T, root string) *Server {
	t.Helper()

	s := NewServer(nil, cflog.NewLogger(false))

	raw, err := json.Marshal(map[string]any{
		"processId":        nil,
		"rootUri":          "file://" + root,
		"capabilities":     map[string]any{},
		"workspaceFolders": []map[string]any{{"uri": "file://" + root, "name": "w"}},
	})
	if err != nil {
		t.Fatalf("marshalling params: %v", err)
	}

	if _, err := s.handleInitialize(context.Background(), raw); err != nil {
		t.Fatalf("handleInitialize: %v", err)
	}

	return s
}

// TestConfigFoundAboveWorkspaceRoot covers a workspace opened on a
// subdirectory of the project, with .cfmleditor.json at the project root.
// Standalone mode used to check only the root directory itself, so this config
// was invisible even though daemon mode loads it.
func TestConfigFoundAboveWorkspaceRoot(t *testing.T) {
	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, ".cfmleditor.json"),
		[]byte(`{"linting":{"enabled":true},"mappings":{"models":"./src/models"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	nested := filepath.Join(project, "src", "components")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	s := initializeWithRoot(t, nested)

	if !s.Linting {
		t.Error("config above the workspace root was not found")
	}

	if len(s.Mappings) == 0 {
		t.Error("mappings from the config above the workspace root were not applied")
	}
}

// TestNearestConfigWins ensures the upward walk stops at the closest config
// rather than continuing to an ancestor.
func TestNearestConfigWins(t *testing.T) {
	outer := t.TempDir()
	if err := os.WriteFile(filepath.Join(outer, ".cfmleditor.json"),
		[]byte(`{"mappings":{"outer":"./outer"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	inner := filepath.Join(outer, "inner")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(inner, ".cfmleditor.json"),
		[]byte(`{"mappings":{"inner":"./inner"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	s := initializeWithRoot(t, inner)

	if _, ok := s.Mappings["inner"]; !ok {
		t.Errorf("expected the nearest config to win, got mappings %v", s.Mappings)
	}
}

// TestUnparseableConfigDoesNotMaskAncestor checks that a malformed config
// does not abort the walk and hide a valid one further up.
func TestUnparseableConfigDoesNotMaskAncestor(t *testing.T) {
	outer := t.TempDir()
	if err := os.WriteFile(filepath.Join(outer, ".cfmleditor.json"),
		[]byte(`{"linting":{"enabled":true}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	inner := filepath.Join(outer, "inner")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(inner, ".cfmleditor.json"),
		[]byte(`{ this is not json`), 0o644); err != nil {
		t.Fatal(err)
	}

	s := initializeWithRoot(t, inner)

	if !s.Linting {
		t.Error("a malformed nearer config masked the valid one above it")
	}
}

// initializeWith drives handleInitialize with both a workspace root and
// client-supplied initializationOptions.
func initializeWith(t *testing.T, root, initOptions string) *Server {
	t.Helper()

	s := NewServer(nil, cflog.NewLogger(false))

	params := map[string]any{
		"processId":        nil,
		"rootUri":          "file://" + root,
		"capabilities":     map[string]any{},
		"workspaceFolders": []map[string]any{{"uri": "file://" + root, "name": "w"}},
	}

	if initOptions != "" {
		var opts any
		if err := json.Unmarshal([]byte(initOptions), &opts); err != nil {
			t.Fatalf("test initOptions is not valid JSON: %v", err)
		}

		params["initializationOptions"] = opts
	}

	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshalling params: %v", err)
	}

	if _, err := s.handleInitialize(context.Background(), raw); err != nil {
		t.Fatalf("handleInitialize: %v", err)
	}

	return s
}

// TestEditorSettingsUsedWithoutConfigFile is the case that motivated this:
// an editor with no .cfmleditor.json anywhere could not enable linting at all.
func TestEditorSettingsUsedWithoutConfigFile(t *testing.T) {
	s := initializeWith(t, t.TempDir(), `{"linting":{"enabled":true}}`)

	if !s.Linting {
		t.Error("linting from initializationOptions was ignored")
	}
}

// TestConfigFileWinsOverEditorSettings pins the agreed precedence: a checked-in
// .cfmleditor.json stays the source of truth for every key it states.
func TestConfigFileWinsOverEditorSettings(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".cfmleditor.json"),
		[]byte(`{"linting":{"enabled":false},"javaStubsPath":"from.file"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	s := initializeWith(t, dir, `{"linting":{"enabled":true},"javaStubsPath":"from.editor"}`)

	if s.Linting {
		t.Error("editor settings overrode the config file's linting; the file must win")
	}

	// javaStubsPath becomes a synthesized resolver, so check it landed from the file.
	for _, r := range s.ComponentResolvers {
		if r.Resolve == "from.editor.$1" {
			t.Error("editor javaStubsPath overrode the file's; the file must win")
		}
	}
}

// TestEditorSettingsFillGapsInConfigFile is the other half of the precedence
// rule: keys the file does not mention still come from the editor.
func TestEditorSettingsFillGapsInConfigFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".cfmleditor.json"),
		[]byte(`{"mappings":{"models":"./src/models"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	s := initializeWith(t, dir, `{"linting":{"enabled":true}}`)

	if !s.Linting {
		t.Error("linting was set only by the editor and the file is silent on it, so it should apply")
	}

	if len(s.Mappings) == 0 {
		t.Error("the config file's mappings were lost during the merge")
	}
}

// TestMalformedEditorSettingsAreIgnored ensures bad initializationOptions
// degrade to "no editor config" rather than breaking initialize outright.
func TestMalformedEditorSettingsAreIgnored(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".cfmleditor.json"),
		[]byte(`{"linting":{"enabled":true}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewServer(nil, cflog.NewLogger(false))

	// A JSON string where an object is expected.
	raw := []byte(`{"processId":null,"rootUri":"file://` + dir + `","capabilities":{},` +
		`"workspaceFolders":[{"uri":"file://` + dir + `","name":"w"}],` +
		`"initializationOptions":"not-an-object"}`)

	if _, err := s.handleInitialize(context.Background(), raw); err != nil {
		t.Fatalf("initialize should survive malformed initializationOptions: %v", err)
	}

	if !s.Linting {
		t.Error("the config file should still have been applied despite bad editor settings")
	}
}

// TestRunDiagnosticsWithoutLinterIsNoop guards the nil-linter path that stays
// reachable whenever linting is disabled or cflint could not be located.
func TestRunDiagnosticsWithoutLinterIsNoop(t *testing.T) {
	s := initializeWithConfig(t, `{}`)

	if s.Linting {
		t.Fatal("linting should be off for an empty config")
	}

	// Must return promptly rather than panicking on the nil linter or conn.
	s.runDiagnostics(context.Background(), "file:///nonexistent.cfc")
}
