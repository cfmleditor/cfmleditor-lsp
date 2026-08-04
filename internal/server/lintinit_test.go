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
