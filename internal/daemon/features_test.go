package daemon

import (
	"os"
	"path/filepath"
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
// must produce the same defaults a standalone session gets. The two
// disagreeing is the exact drift that left daemon sessions with completions
// switched off.
//
// The comparison is against config.ResolveFeatures(nil) rather than against a
// list of expected values, so it keeps testing agreement rather than restating
// the defaults — folding's is off, and a copy here would have had to be found
// and changed when it moved.
func TestSettingsFromDefaultsFeaturesOn(t *testing.T) {
	set := SettingsFrom(configWith(t, `{"workspaceName": "w"}`))

	if set.Features != config.ResolveFeatures(nil) {
		t.Errorf("daemon defaults %+v, config says %+v", set.Features, config.ResolveFeatures(nil))
	}

	// The trap this guards: the zero value is every switch off, and a daemon
	// path that never applied the defaults would land on it.
	if set.Features == (config.ResolvedFeatures{}) {
		t.Fatal("daemon took the zero value of ResolvedFeatures — every switch off")
	}
}
