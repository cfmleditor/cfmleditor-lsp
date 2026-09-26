package server

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
)

// A config that does not mention a block must not reset it.
//
// This is how a feature switch could read off in the file and on in the server.
// The daemon applies the config it was started from; then initialize loads the
// config at the workspace root and applied *its* resolved values over the top —
// and an absent block resolves to that block's defaults, not to "leave this
// alone". A workspace config with no `features` block switched every feature
// back on.
func TestAConfigWithoutABlockLeavesItAlone(t *testing.T) {
	dir := t.TempDir()

	// A config naming only mappings: every optional block is absent.
	write(t, filepath.Join(dir, ".cfmleditor.json"), `{"mappings":{"app":"./app"}}`)

	off, on := false, true

	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}
	// governingConfig walks up from the *editor's* roots, not WorkspaceFolders.
	// Without this the config is never found, configureSession returns early,
	// and every assertion below passes because nothing happened at all.
	srv.addWorkspaceRoot(dir)

	// What the daemon would have applied before initialize runs.
	srv.Features = config.ResolveFeatures(&config.Features{Routes: &off, Folding: &on})
	srv.Linting = true
	srv.LintMinSeverity = "ERROR"
	srv.References = true
	srv.TagSnippets = false
	srv.GlobalFunctionResolution = false

	if path, cfg := srv.governingConfig(); cfg == nil || filepath.Dir(path) != dir {
		t.Fatalf("the test config was not the governing one (got %q); every assertion below would pass vacuously", path)
	}

	srv.configureSession(nil)

	if srv.Features.Routes {
		t.Error("features.routes came back on; a config with no features block reset it")
	}

	if !srv.Features.Folding {
		t.Error("features.folding was lost; a config with no features block reset it")
	}

	if !srv.Linting {
		t.Error("linting was lost")
	}

	if srv.LintMinSeverity != "ERROR" {
		t.Errorf("linting.minSeverity = %q, want ERROR", srv.LintMinSeverity)
	}

	if !srv.References {
		t.Error("references was lost")
	}

	if srv.TagSnippets {
		t.Error("completions.tagSnippets came back on")
	}

	if srv.GlobalFunctionResolution {
		t.Error("completions.globalFunctionResolution came back on")
	}
}

// A config that *does* mention a block still wins, or the fix above would make
// configuration unchangeable rather than merely sticky.
func TestAConfigWithABlockStillWins(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, ".cfmleditor.json"), `{"features":{"routes":false},"linting":{"enabled":false}}`)

	on := true

	srv := newTestServer()
	srv.WorkspaceFolders = []string{dir}
	srv.addWorkspaceRoot(dir)
	srv.Features = config.ResolveFeatures(&config.Features{Routes: &on})
	srv.Linting = true

	srv.configureSession(nil)

	if srv.Features.Routes {
		t.Error("features.routes stayed on; the config that names it must win")
	}

	if srv.Linting {
		t.Error("linting stayed on; the config that names it must win")
	}
}

// optionalBlockState and config.JSON's optional blocks are the same list written
// twice. A block added to the config and not here is one that silently goes back
// to its defaults whenever a config omits it — exactly the bug above, returning.
func TestEveryOptionalConfigBlockIsPreserved(t *testing.T) {
	var blocks []string

	ct := reflect.TypeFor[config.JSON]()
	for f := range ct.Fields() {
		if f.Type.Kind() == reflect.Pointer && f.Type.Elem().Kind() == reflect.Struct {
			blocks = append(blocks, f.Name)
		}
	}

	src, err := os.ReadFile("standalone_config.go")
	if err != nil {
		t.Fatalf("reading standalone_config.go: %v", err)
	}

	for _, b := range blocks {
		if !strings.Contains(string(src), "merged."+b+" == nil") {
			t.Errorf("config.JSON has an optional %s block, but restoreUnmentionedBlocks does not "+
				"preserve it; a config omitting %s will silently reset it to defaults", b, b)
		}
	}

	t.Logf("optional blocks preserved: %v", blocks)
}

func write(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}
