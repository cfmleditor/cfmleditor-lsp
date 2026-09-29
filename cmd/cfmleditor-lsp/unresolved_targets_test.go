package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

func TestScanTargetsNarrowsToADirectoryUnderAWiderIndex(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "a", "one.cfc")
	outside := filepath.Join(root, "b", "two.cfc")

	for _, p := range []string{inside, outside} {
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(p, []byte("component {}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	got := scanTargets(vfs.OS{}, []string{filepath.Join(root, "a")}, true)
	if len(got) != 1 || got[0] != inside {
		t.Errorf("directory under a wider index: got %v, want only %s", got, inside)
	}

	if got := scanTargets(vfs.OS{}, []string{filepath.Join(root, "a")}, false); len(got) != 0 {
		t.Errorf("no wider index: a directory must not narrow, got %v", got)
	}
}
