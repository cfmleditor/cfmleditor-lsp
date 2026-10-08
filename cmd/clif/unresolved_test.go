package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cfmleditor/clif/internal/daemon"
	"github.com/cfmleditor/clif/internal/vfs"
)

func TestCollectCFMLFiles(t *testing.T) {
	dir := t.TempDir()

	mustWrite := func(rel string) {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(full, []byte("component {}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	mustWrite("a.cfc")
	mustWrite("b.cfm")
	mustWrite("readme.md")             // not a CFML file — excluded
	mustWrite("node_modules/skip.cfc") // excluded dir
	mustWrite(".git/skip.cfc")         // excluded dir
	mustWrite("vendor/skip.cfc")       // excluded dir
	mustWrite("nested/deep/keep.cfml") // nested, real CFML — included

	files := collectCFMLFiles(vfs.OS{}, []string{dir})

	found := make(map[string]bool, len(files))
	for _, f := range files {
		found[filepath.Base(f)] = true
	}

	for _, want := range []string{"a.cfc", "b.cfm", "keep.cfml"} {
		if !found[want] {
			t.Errorf("expected %s to be collected, got %v", want, files)
		}
	}

	for _, unwanted := range []string{"readme.md", "skip.cfc"} {
		if found[unwanted] {
			t.Errorf("expected %s to be excluded, got %v", unwanted, files)
		}
	}

	if len(files) != 3 {
		t.Errorf("expected exactly 3 files (a.cfc, b.cfm, keep.cfml), got %d: %v", len(files), files)
	}
}

// TestPresetHint: the scan ends by suggesting the presets box.json implies,
// naming the config to add them to and keeping the ones it already has; with
// every implied preset named, or no box.json, it says nothing.
func TestPresetHint(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()

		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if got := presetHint(nil, dir); got != "" {
		t.Errorf("no box.json: %q", got)
	}

	write("box.json", `{"dependencies":{"coldbox":"^7"},"devDependencies":{"testbox":"*"}}`)

	if got := presetHint(nil, dir); !strings.Contains(got, `"frameworks": ["coldbox", "testbox"]`) || !strings.Contains(got, "a .clif.json") {
		t.Errorf("no config: %q", got)
	}

	write(".cfmleditor.json", `{"frameworks": ["coldbox"]}`)

	cfg, err := daemon.FindConfig(dir)
	if err != nil || cfg == nil {
		t.Fatalf("FindConfig: %v", err)
	}

	if got := presetHint(cfg, dir); !strings.Contains(got, "names testbox,") || !strings.Contains(got, `["coldbox", "testbox"]`) || !strings.Contains(got, cfg.Path) {
		t.Errorf("with config: %q", got)
	}

	write(".cfmleditor.json", `{"frameworks": ["coldbox", "testbox"]}`)

	cfg, _ = daemon.FindConfig(dir)
	if got := presetHint(cfg, dir); got != "" {
		t.Errorf("all named: %q", got)
	}
}
