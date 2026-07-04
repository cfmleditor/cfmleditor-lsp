package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

func TestIsBuiltin(t *testing.T) {
	if !isBuiltin("trim") {
		t.Error("expected trim (a documented built-in function) to be recognized")
	}

	if !isBuiltin("TRIM") {
		t.Error("expected case-insensitive built-in lookup")
	}

	if !isBuiltin("append") {
		t.Error("expected 'append' (an array/list member function name) to be recognized as builtin")
	}

	if isBuiltin("someUserDefinedFunctionXyz") {
		t.Error("expected a made-up function name to not be recognized as builtin")
	}
}

func TestIsMemberFunction(t *testing.T) {
	if !isMemberFunction("append") {
		t.Error("expected 'append' to be a known member function (from arrayAppend)")
	}

	if !isMemberFunction("APPEND") {
		t.Error("expected case-insensitive member function lookup")
	}

	if isMemberFunction("notARealMemberFunctionXyz") {
		t.Error("expected an unknown name to not be a member function")
	}
}

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
