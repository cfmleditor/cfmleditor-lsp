package server

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	json "github.com/go-json-experiment/json"

	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// boolOf reads a protocol provider field that may be a bool or a bool-valued
// union, which is how the generated capabilities struct spells them.
func providerOn(t *testing.T, v any) bool {
	t.Helper()

	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Bool {
		return rv.Bool()
	}

	if rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return false
		}

		return providerOn(t, rv.Elem().Interface())
	}

	t.Fatalf("provider field of unexpected kind %s", rv.Kind())

	return false
}

// TestFeaturesDefaultOnInCapabilities is the baseline the switches are measured
// against: a server with no config advertises all three providers.
func TestFeaturesDefaultOnInCapabilities(t *testing.T) {
	caps := initializeWithConfig(t, `{"formatting": {"enabled": true}}`).capabilities()

	if !providerOn(t, caps.DocumentHighlightProvider) {
		t.Error("documentHighlight is not advertised by default")
	}

	if !providerOn(t, caps.FoldingRangeProvider) {
		t.Error("foldingRange is not advertised by default")
	}

	if !providerOn(t, caps.DocumentRangeFormattingProvider) {
		t.Error("rangeFormatting is not advertised by default")
	}
}

// TestDisablingAFeatureUnadvertisesIt. Un-advertising rather than declining is
// the point: a client told there is no provider falls back to its own
// behaviour, where one told there is a provider and then handed nothing shows
// the user an empty result.
func TestDisablingAFeatureUnadvertisesIt(t *testing.T) {
	caps := initializeWithConfig(t, `{
		"formatting": {"enabled": true},
		"features": {"documentHighlight": false, "folding": false, "rangeFormatting": false}
	}`).capabilities()

	if providerOn(t, caps.DocumentHighlightProvider) {
		t.Error("documentHighlight is still advertised after being switched off")
	}

	if providerOn(t, caps.FoldingRangeProvider) {
		t.Error("foldingRange is still advertised after being switched off")
	}

	if providerOn(t, caps.DocumentRangeFormattingProvider) {
		t.Error("rangeFormatting is still advertised after being switched off")
	}
}

// TestDisablingOneFeatureLeavesTheRestAdvertised reaches the server through a
// real initialize, which is the hop the config-package test cannot cover.
func TestDisablingOneFeatureLeavesTheRestAdvertised(t *testing.T) {
	caps := initializeWithConfig(t, `{
		"formatting": {"enabled": true},
		"features": {"folding": false}
	}`).capabilities()

	if providerOn(t, caps.FoldingRangeProvider) {
		t.Error("foldingRange is still advertised after being switched off")
	}

	if !providerOn(t, caps.DocumentHighlightProvider) || !providerOn(t, caps.DocumentRangeFormattingProvider) {
		t.Error("switching off folding also un-advertised its siblings")
	}
}

// TestDisabledHandlersDeclineTheRequest: the capability is the real gate, but a
// client that sends the request anyway must get nothing rather than an answer
// the capability said would not come.
func TestDisabledHandlersDeclineTheRequest(t *testing.T) {
	s := initializeWithConfig(t, `{
		"formatting": {"enabled": true},
		"features": {"documentHighlight": false, "folding": false, "rangeFormatting": false}
	}`)

	docURI := uriOfTempDoc(t, s, "component {\n\tfunction f() {\n\t\tx = 1;\n\t}\n}\n")

	hl, err := s.handleDocumentHighlight(context.Background(), mustJSON(t, protocol.DocumentHighlightParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
			Position:     protocol.Position{Line: 2, Character: 2},
		},
	}))
	if err != nil || hl != nil {
		t.Errorf("documentHighlight answered while disabled: %v, %v", hl, err)
	}

	fr, err := s.handleFoldingRange(context.Background(), mustJSON(t, protocol.FoldingRangeParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
	}))
	if err != nil || fr != nil {
		t.Errorf("foldingRange answered while disabled: %v, %v", fr, err)
	}

	rf, err := s.handleRangeFormatting(context.Background(), mustJSON(t, protocol.DocumentRangeFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		Range:        protocol.Range{End: protocol.Position{Line: 4}},
	}))
	if err != nil || rf != nil {
		t.Errorf("rangeFormatting answered while disabled: %v, %v", rf, err)
	}
}

// TestRangeFormattingHasItsOwnSwitch is the reason it is not simply governed by
// formatting.enabled: turning it off must leave format-on-save working.
func TestRangeFormattingHasItsOwnSwitch(t *testing.T) {
	caps := initializeWithConfig(t, `{
		"formatting": {"enabled": true},
		"features": {"rangeFormatting": false}
	}`).capabilities()

	if providerOn(t, caps.DocumentRangeFormattingProvider) {
		t.Error("rangeFormatting is still advertised after being switched off")
	}

	if !providerOn(t, caps.DocumentFormattingProvider) {
		t.Error("switching off rangeFormatting also switched off whole-document formatting")
	}
}

// TestWatchedFilesSwitchReachesTheServer: the config hop, on its own.
func TestWatchedFilesSwitchReachesTheServer(t *testing.T) {
	if initializeWithConfig(t, `{"features": {"watchedFiles": false}}`).Features.WatchedFiles {
		t.Error("watchedFiles=false did not reach the server")
	}
}

// TestWatchedFilesSwitchStopsReindexing. There is no capability to
// un-advertise here — file watching is a registration the server sends — so the
// switch is checked on the work itself.
//
// The fixture is a real file with a real function name, and the control below
// is what makes the assertion mean anything: the first version of this test
// fired an event at a path that did not exist and asserted an unrelated name
// was unindexed, which was true whether or not the switch was honoured.
//
// Built with newTestServer rather than through initialize, because
// handleInitialize spawns the workspace indexer and setting WorkspaceFolders
// afterwards races its read of them — which `go test -race` duly reported.
func TestWatchedFilesSwitchStopsReindexing(t *testing.T) {
	setup := func(t *testing.T, on bool) (*Server, string) {
		t.Helper()

		dir := t.TempDir()

		path := filepath.Join(dir, "Watched.cfc")
		if err := os.WriteFile(path, []byte("component {\n\tfunction watchedFn() {}\n}\n"), 0o600); err != nil {
			t.Fatal(err)
		}

		s := newTestServer()
		s.WorkspaceFolders = []string{dir}
		s.Features.WatchedFiles = on

		return s, path
	}

	t.Run("off", func(t *testing.T) {
		s, path := setup(t, false)

		s.applyWatchedFileChanges([]protocol.FileEvent{{URI: uri.File(path), Type: protocol.FileChangeTypeCreated}})

		if got := len(s.index.Lookup("watchedFn")); got != 0 {
			t.Errorf("a watched-file change was applied while watching is disabled (%d entries)", got)
		}
	})

	t.Run("on (control)", func(t *testing.T) {
		s, path := setup(t, true)

		s.applyWatchedFileChanges([]protocol.FileEvent{{URI: uri.File(path), Type: protocol.FileChangeTypeCreated}})

		if got := len(s.index.Lookup("watchedFn")); got != 1 {
			t.Errorf("the change was not applied with watching on (%d entries) — the off case above proves nothing", got)
		}
	})
}

// TestNewServerDefaultsFeaturesOn covers the path that skips config entirely.
// The zero value of ResolvedFeatures is every switch off, so a session that
// never reaches applyConfig would silently lose all four.
func TestNewServerDefaultsFeaturesOn(t *testing.T) {
	s := newTestServer()

	rv := reflect.ValueOf(s.Features)
	for i := range rv.Type().NumField() {
		if rv.Field(i).Interface() != true {
			t.Errorf("a fresh Server has %s off, want on", rv.Type().Field(i).Name)
		}
	}

	// And it must match what the config package says an absent block means.
	if s.Features != config.ResolveFeatures(nil) {
		t.Errorf("NewServer defaults %+v, config.ResolveFeatures(nil) says %+v", s.Features, config.ResolveFeatures(nil))
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()

	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}

	return b
}

func uriOfTempDoc(t *testing.T, s *Server, content string) uri.URI {
	t.Helper()

	docURI := uri.URI("file:///tmp/feature-test/T.cfc")
	s.setDocument(docURI, content)

	return docURI
}
