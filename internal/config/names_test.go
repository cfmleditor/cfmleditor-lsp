package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestADefaultReportKeepsItsLegacyName: a project that committed its report
// as .cfmleditor-cflint.txt keeps writing and reading it there, and one with
// neither file gets the new name.
func TestADefaultReportKeepsItsLegacyName(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, ".clif-cflint.txt")
	legacy := filepath.Join(dir, ".cfmleditor-cflint.txt")

	if got := defaultReport(dir, GenerateCFLint); got != current {
		t.Errorf("with neither file: %q, want %q", got, current)
	}

	if err := os.WriteFile(legacy, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	if got := GenerateTargets(nil, GenerateCFLint, dir); len(got) != 1 || got[0] != legacy {
		t.Errorf("with the legacy file: %v, want %q", got, legacy)
	}

	if err := os.WriteFile(current, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	if got := defaultReport(dir, GenerateCFLint); got != current {
		t.Errorf("with both files: %q, want %q", got, current)
	}

	// Listed by either name with no generate kind, the file is still that report.
	for _, name := range []string{".clif-unresolved.txt", ".cfmleditor-unresolved.txt"} {
		list := ResolveKnownIssues(&KnownIssuesConfig{Files: []KnownIssues{{File: name}}}, dir)
		if len(list) == 0 || !list[0].IsGenerated(GenerateUnresolved) {
			t.Errorf("%s listed bare is not the unresolved report: %+v", name, list)
		}
	}
}
