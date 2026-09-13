package daemon

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
)

func configWith(t *testing.T, body string) *Config {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, ".cfmleditor.json")

	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := FindConfig(dir)
	if err != nil {
		t.Fatal(err)
	}

	if cfg == nil {
		t.Fatal("FindConfig found nothing")
	}

	return cfg
}

// TestSettingsFromCarriesFeatures is the hop that reaches every daemon session.
// A switch read from config but never copied into Settings reaches none of
// them, and that failure is invisible — it looks exactly like the switch not
// being set. `completions` went missing here for precisely that reason.
func TestSettingsFromCarriesFeatures(t *testing.T) {
	set := SettingsFrom(configWith(t, `{
		"workspaceName": "w",
		"features": {"folding": false, "watchedFiles": false}
	}`))

	if set.Features.Folding {
		t.Error("folding=false did not reach Settings")
	}

	if set.Features.WatchedFiles {
		t.Error("watchedFiles=false did not reach Settings")
	}

	if !set.Features.DocumentHighlight || !set.Features.RangeFormatting {
		t.Errorf("SettingsFrom switched off features the config did not name: %+v", set.Features)
	}
}

// TestSettingsFromDefaultsFeaturesOn: a daemon config with no features block
// must produce the same all-on defaults a standalone session gets. The two
// disagreeing is the exact drift that left daemon sessions with completions
// switched off.
func TestSettingsFromDefaultsFeaturesOn(t *testing.T) {
	set := SettingsFrom(configWith(t, `{"workspaceName": "w"}`))

	if set.Features != config.ResolveFeatures(nil) {
		t.Errorf("daemon defaults %+v, config says %+v", set.Features, config.ResolveFeatures(nil))
	}

	rv := reflect.ValueOf(set.Features)
	for i := range rv.Type().NumField() {
		if rv.Field(i).Interface() != true {
			t.Errorf("daemon leaves %s off with no features block", rv.Type().Field(i).Name)
		}
	}
}
