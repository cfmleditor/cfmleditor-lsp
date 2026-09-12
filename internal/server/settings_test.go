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

// TestNearestConfigToTheEditorWins pins which .cfmleditor.json governs a
// session. The daemon walks up from the process's working directory and the
// session walks up from the workspace roots the editor reported; those start in
// different places, so a project's own config can sit under the folder the
// editor opened while the daemon passed a different one on its way up. The
// nearer one is this session's.
func TestNearestConfigToTheEditorWins(t *testing.T) {
	outer := t.TempDir()
	inner := filepath.Join(outer, "project")

	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}

	outerPath := filepath.Join(outer, ".cfmleditor.json")
	if err := os.WriteFile(outerPath, []byte(`{"mappings": {"models": "./outer-models"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(inner, ".cfmleditor.json"), []byte(`{"mappings": {"models": "./inner-models"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewServer(nil, cflog.NewLogger(false))
	// The daemon found the outer config; the editor opened the inner folder.
	Settings{ConfigPath: outerPath, Mappings: map[string]string{"models": filepath.Join(outer, "outer-models")}}.Apply(s)
	initializeIn(t, s, inner)

	if got := s.Mappings["models"]; got != filepath.Join(inner, "inner-models") {
		t.Errorf("mappings came from %q, want the project's own config", got)
	}
}

// TestConfigReloadDoesNotDependOnComponentResolvers is the defect the
// ConfigPath work replaced. The condition used to be "this session has no
// component resolvers", so whether a session re-read and re-applied the
// workspace config at startup turned on whether the config happened to declare
// one — an unrelated key deciding unrelated behaviour. Both shapes must behave
// identically, and neither may end up with the file's resolvers listed twice.
func TestConfigReloadDoesNotDependOnComponentResolvers(t *testing.T) {
	const resolverJSON = `{"componentResolvers": [{"match": "getService(\"$1\")", "resolve": "svc.$1", "prefix": "getService"}]}`

	for _, tc := range []struct {
		name      string
		resolvers []config.Resolver
	}{
		{"settings without resolvers", nil},
		{"settings with the file's resolver already applied", []config.Resolver{
			{Match: `getService("$1")`, Resolve: "svc.$1", Prefix: "getService"},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			cfgPath := filepath.Join(dir, ".cfmleditor.json")

			if err := os.WriteFile(cfgPath, []byte(resolverJSON), 0o644); err != nil {
				t.Fatal(err)
			}

			s := NewServer(nil, cflog.NewLogger(false))
			Settings{ConfigPath: cfgPath, ComponentResolvers: tc.resolvers}.Apply(s)
			initializeIn(t, s, dir)

			if got := len(s.ComponentResolvers); got != 1 {
				t.Errorf("component resolvers = %d, want 1: %+v", got, s.ComponentResolvers)
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

// initializeServerWith drives handleInitialize on an existing Server, so a
// caller can apply Settings to it first the way a daemon session does.
// lintinit_test.go's initializeWith builds its own Server, which cannot.
func initializeServerWith(t *testing.T, s *Server, dir, initOptions string) {
	t.Helper()

	params := map[string]any{
		"processId":        nil,
		"rootUri":          "file://" + dir,
		"capabilities":     map[string]any{},
		"workspaceFolders": []map[string]any{{"uri": "file://" + dir, "name": "w"}},
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
}

// TestEditorSettingsMergeWithTheDaemonsConfig is the IntelliLucee case. That
// plugin has no .cfmleditor.json to write to: it sends every formatter setting
// from the IDE's settings UI as initializationOptions, and starts the server
// with no working directory of its own. Whether the daemon's walk finds a
// config file therefore depends on where the IDE process happens to have been
// started — and must not decide whether the IDE's settings are honoured.
func TestEditorSettingsMergeWithTheDaemonsConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".cfmleditor.json")

	if err := os.WriteFile(cfgPath, []byte(`{
		"mappings": {"models": "./models"},
		"formatting": {"enabled": true, "lineWidth": 100}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewServer(nil, cflog.NewLogger(false))
	Settings{
		ConfigPath: cfgPath,
		Mappings:   map[string]string{"models": filepath.Join(dir, "models")},
		Formatting: config.ResolvedFormatting{Enabled: true, LineWidth: 100},
	}.Apply(s)

	initializeServerWith(t, s, dir, `{"formatting": {"enabled": true, "lineWidth": 100, "attrBreakThreshold": 7}}`)

	if got := s.Formatting.AttrBreakThreshold; got != 7 {
		t.Errorf("attrBreakThreshold from the editor = %d, want 7 — initializationOptions were dropped", got)
	}

	if got := s.Formatting.LineWidth; got != 100 {
		t.Errorf("lineWidth = %d, want 100 from the config file", got)
	}

	if got := s.Mappings["models"]; got != filepath.Join(dir, "models") {
		t.Errorf("the config file's mappings were lost in the merge: %q", got)
	}
}

// TestConfigFileStillWinsOverEditorSettings keeps the documented precedence
// through the merge: the file wins on every key it sets.
func TestConfigFileStillWinsOverEditorSettings(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".cfmleditor.json")

	if err := os.WriteFile(cfgPath, []byte(`{"formatting": {"enabled": true, "lineWidth": 100}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewServer(nil, cflog.NewLogger(false))
	Settings{ConfigPath: cfgPath, Formatting: config.ResolvedFormatting{Enabled: true, LineWidth: 100}}.Apply(s)

	initializeServerWith(t, s, dir, `{"formatting": {"enabled": true, "lineWidth": 40}}`)

	if got := s.Formatting.LineWidth; got != 100 {
		t.Errorf("lineWidth = %d, want the config file's 100", got)
	}
}

// TestOverlayDoesNotDuplicateResolvers is why the overlay clears before it
// applies. applyConfig appends resolvers, so re-applying the same file on top
// of what the daemon already put there would list every one of them twice.
func TestOverlayDoesNotDuplicateResolvers(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".cfmleditor.json")

	if err := os.WriteFile(cfgPath, []byte(`{
		"componentResolvers": [{"match": "getService(\"$1\")", "resolve": "svc.$1", "prefix": "getService"}]
	}`), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewServer(nil, cflog.NewLogger(false))
	Settings{
		ConfigPath:         cfgPath,
		ComponentResolvers: []config.Resolver{{Match: `getService("$1")`, Resolve: "svc.$1", Prefix: "getService"}},
	}.Apply(s)

	initializeServerWith(t, s, dir, `{"formatting": {"enabled": true}}`)

	if got := len(s.ComponentResolvers); got != 1 {
		t.Errorf("component resolvers = %d, want 1: %+v", got, s.ComponentResolvers)
	}
}

// TestConfigAppliedOnceWithNoEditorSettings covers the plainest daemon case:
// nothing to merge, and the governing file applied exactly once.
func TestConfigAppliedOnceWithNoEditorSettings(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".cfmleditor.json")

	if err := os.WriteFile(cfgPath, []byte(`{"linting": {"enabled": true}, "mappings": {"models": "./models"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewServer(nil, cflog.NewLogger(false))
	Settings{ConfigPath: cfgPath, Linting: true, Mappings: map[string]string{"models": filepath.Join(dir, "models")}}.Apply(s)
	initializeIn(t, s, dir)

	if !s.Linting {
		t.Error("the config file's linting was lost")
	}

	if got := s.Mappings["models"]; got != filepath.Join(dir, "models") {
		t.Errorf("mappings = %q", got)
	}
}

// TestEditorSettingsSurviveAnUnreadableDaemonConfig is the failure this path
// cannot afford. For a client with no config file to write, initializationOptions
// are the entire configuration; discarding them because a file the daemon
// mentioned has since gone would leave the session unconfigured and say nothing.
func TestEditorSettingsSurviveAnUnreadableDaemonConfig(t *testing.T) {
	dir := t.TempDir()

	s := NewServer(nil, cflog.NewLogger(false))
	Settings{ConfigPath: filepath.Join(dir, "gone", ".cfmleditor.json")}.Apply(s)

	initializeServerWith(t, s, dir, `{"linting": {"enabled": true}}`)

	if !s.Linting {
		t.Error("initializationOptions were dropped because the daemon's config file could not be read")
	}
}
