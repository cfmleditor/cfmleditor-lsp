package server

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	json "github.com/go-json-experiment/json"

	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"
)

// Every field of Settings must reach the Server. The failure this guards was
// not a wrong value but a missing assignment: daemon mode configured its
// stdio session and its socket sessions from two separate blocks of code, and
// two keys were only ever written in one of them.
//
// Comparing field-by-field via reflection rather than asserting a hand-written
// list means a field added to Settings and forgotten in Apply fails here,
// which is the mistake worth catching.
func TestSettingsApplyCoversEveryField(t *testing.T) {
	set := Settings{
		ConfigPath:               "/w/.cfmleditor.json",
		WorkspaceFolders:         []string{"/w"},
		IndexGlobs:               []string{"**/*.cfc"},
		Mappings:                 map[string]string{"models": "/w/models"},
		ExpressionMappings:       map[string]string{"#CORE#": "packages.core."},
		ServicePropertyResolvers: map[string]string{"package": "packages.${name}"},
		ComponentResolvers:       []config.Resolver{{Match: "getService(\"$1\")", Resolve: "services.$1", Prefix: "getService"}},
		PropertyResolvers:        []config.PropResolver{{Match: "$1", Resolve: "beans.$1", Attribute: "name"}},
		BeanPaths:                map[string]string{"svc": "/w/services"},
		Formatting:               config.ResolvedFormatting{Enabled: true, LineWidth: 123},
		Linting:                  true,
		References:               true,
		TagSnippets:              true,
		FunctionSnippets:         true,
		GlobalFunctionResolution: true,
	}

	srv := NewServer(nil, cflog.NewLogger(false))
	set.Apply(srv)

	setVal, srvVal := reflect.ValueOf(set), reflect.ValueOf(srv).Elem()

	for i := range setVal.NumField() {
		name := setVal.Type().Field(i).Name

		field := srvVal.FieldByName(name)
		if !field.IsValid() {
			t.Errorf("Settings.%s has no matching Server field", name)

			continue
		}

		if field.IsZero() {
			t.Errorf("Settings.%s was not applied to the Server", name)
		}
	}
}

// Apply must give the two session kinds identical configuration.
func TestSettingsApplyIsIdenticalAcrossSessions(t *testing.T) {
	set := Settings{
		Mappings:                 map[string]string{"models": "/w/models"},
		ExpressionMappings:       map[string]string{"#CORE#": "packages.core."},
		ServicePropertyResolvers: map[string]string{"package": "packages.${name}"},
		BeanPaths:                map[string]string{"svc": "/w/services"},
	}

	stdio := NewServer(nil, cflog.NewLogger(false))
	socket := NewServer(nil, cflog.NewLogger(false))

	set.Apply(stdio)
	set.Apply(socket)

	for _, tc := range []struct {
		name string
		a, b map[string]string
	}{
		{"Mappings", stdio.Mappings, socket.Mappings},
		{"ExpressionMappings", stdio.ExpressionMappings, socket.ExpressionMappings},
		{"ServicePropertyResolvers", stdio.ServicePropertyResolvers, socket.ServicePropertyResolvers},
		{"BeanPaths", stdio.BeanPaths, socket.BeanPaths},
	} {
		if !reflect.DeepEqual(tc.a, tc.b) {
			t.Errorf("%s differs between a stdio and a socket session: %v vs %v", tc.name, tc.a, tc.b)
		}
	}
}

// initializeIn runs initialize against dir, without writing any config of its
// own, so a caller can set up the config file and the Settings independently.
func initializeIn(t *testing.T, s *Server, dir string) {
	t.Helper()

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
}

// TestSessionConfiguredByTheDaemonDoesNotReloadConfig pins what the flag is for. A
// daemon session is handed its settings before initialize, and must not then
// load .cfmleditor.json a second time and apply it over the top.
//
// The config file here deliberately disagrees with the Settings, which is not
// a situation daemon mode produces — the daemon read that same file — but it
// is the only way to observe from the outside which of the two paths ran.
func TestSessionConfiguredByTheDaemonDoesNotReloadConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".cfmleditor.json"), []byte(`{"linting":{"enabled":false}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewServer(nil, cflog.NewLogger(false))
	Settings{ConfigPath: filepath.Join(dir, ".cfmleditor.json"), Linting: true}.Apply(s)
	initializeIn(t, s, dir)

	if !s.Linting {
		t.Error("the workspace config was loaded over a session the daemon had already configured")
	}
}

// TestConfigReloadDoesNotDependOnComponentResolvers is the defect the flag
// replaces. The condition used to be "this session has no component
// resolvers", so whether a daemon session re-read and re-applied the workspace
// config at startup turned on whether the config happened to declare one —
// an unrelated key deciding unrelated behaviour. Both shapes must now behave
// identically.
func TestConfigReloadDoesNotDependOnComponentResolvers(t *testing.T) {
	for _, tc := range []struct {
		name      string
		resolvers []config.Resolver
	}{
		{"settings without resolvers", nil},
		{"settings with a resolver", []config.Resolver{{Match: "getService(\"$1\")", Resolve: "svc.$1", Prefix: "getService"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, ".cfmleditor.json"), []byte(`{"linting":{"enabled":false}}`), 0o644); err != nil {
				t.Fatal(err)
			}

			s := NewServer(nil, cflog.NewLogger(false))
			Settings{
				ConfigPath:         filepath.Join(dir, ".cfmleditor.json"),
				Linting:            true,
				ComponentResolvers: tc.resolvers,
			}.Apply(s)
			initializeIn(t, s, dir)

			if !s.Linting {
				t.Error("config reloaded over an already-configured session")
			}

			if got := len(s.ComponentResolvers); got != len(tc.resolvers) {
				t.Errorf("component resolvers went from %d to %d — the config was applied a second time",
					len(tc.resolvers), got)
			}
		})
	}
}

// TestSessionWithNoConfigPathStillLoadsConfig is the other side, and it is not
// a rare one: daemon.FindConfig returns an empty path rather than nothing when
// its walk finds no file, so Settings.Apply runs for every session whether a
// config was found or not. A session told of no config file has to do its own
// discovery from the editor's workspace roots — which is also the only path
// that reads initializationOptions.
func TestSessionWithNoConfigPathStillLoadsConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".cfmleditor.json"), []byte(`{"linting":{"enabled":true}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewServer(nil, cflog.NewLogger(false))
	Settings{}.Apply(s)
	initializeIn(t, s, dir)

	if !s.Linting {
		t.Error("a session whose daemon found no config file did not discover one itself")
	}
}

// TestEditorSettingsReachASessionWithNoConfigPath is the same path seen through
// the feature that depends on it. initializationOptions are read only inside
// loadWorkspaceConfig, so a guard that skips it whenever Settings.Apply has run
// would silently drop every editor-provided setting — and Apply runs for every
// session, config file or not.
func TestEditorSettingsReachASessionWithNoConfigPath(t *testing.T) {
	dir := t.TempDir()

	s := NewServer(nil, cflog.NewLogger(false))
	Settings{}.Apply(s)

	raw, err := json.Marshal(map[string]any{
		"processId":             nil,
		"rootUri":               "file://" + dir,
		"capabilities":          map[string]any{},
		"initializationOptions": map[string]any{"linting": map[string]any{"enabled": true}},
		"workspaceFolders":      []map[string]any{{"uri": "file://" + dir, "name": "w"}},
	})
	if err != nil {
		t.Fatalf("marshalling params: %v", err)
	}

	if _, err := s.handleInitialize(context.Background(), raw); err != nil {
		t.Fatalf("handleInitialize: %v", err)
	}

	if !s.Linting {
		t.Error("initializationOptions were never applied")
	}
}
