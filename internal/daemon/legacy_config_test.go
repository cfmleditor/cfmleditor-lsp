package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindConfigPrefersClifJSON(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	legacy := filepath.Join(dir, ".cfmleditor.json")
	if err := os.WriteFile(legacy, []byte(`{"workspaceName":"old"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if cfg, _ := FindConfig(dir); cfg == nil || cfg.Path != legacy || cfg.Name != "old" {
		t.Fatalf("with only the legacy file: %+v", cfg)
	}

	current := filepath.Join(dir, ".clif.json")
	if err := os.WriteFile(current, []byte(`{"workspaceName":"new"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if cfg, _ := FindConfig(filepath.Join(dir, "sub")); cfg == nil || cfg.Path != current || cfg.Name != "new" {
		t.Errorf("with both: %+v, want %s", cfg, current)
	}
}
