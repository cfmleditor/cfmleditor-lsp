package config

import (
	"path/filepath"
	"testing"
)

// TestStartupFilesResolveAgainstTheConfig: a startupFiles entry is relative to
// the config that names it, as mappings are, and an absolute one is kept.
func TestStartupFilesResolveAgainstTheConfig(t *testing.T) {
	dir := t.TempDir()
	abs := filepath.Join(t.TempDir(), "elsewhere.cfm")

	r := Resolve(&JSON{StartupFiles: []string{"packages/core/bootstrap.cfm", abs}}, dir)

	want := []string{filepath.Join(dir, "packages", "core", "bootstrap.cfm"), abs}
	if len(r.StartupFiles) != len(want) || r.StartupFiles[0] != want[0] || r.StartupFiles[1] != want[1] {
		t.Errorf("StartupFiles = %v, want %v", r.StartupFiles, want)
	}
}
