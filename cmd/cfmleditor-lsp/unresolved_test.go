package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
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
