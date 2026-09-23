package config

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"testing"
)

// TestKnownIssuesBlockIsInherited covers the knownIssues block: a file is a
// path or an object, and a file's scope and severity are its own, or the
// block's when it sets none or "inherit", or open and information when the
// block sets none either. The default reports are implicit, with the block's
// settings, and one named bare keeps its kind.
func TestKnownIssuesBlockIsInherited(t *testing.T) {
	var cfg JSON

	err := json.Unmarshal([]byte(`{"knownIssues": {"scope": "workspace", "severity": "warning", "files": [
		".cfmleditor-unresolved.txt",
		{"file": "docs/todo.txt", "severity": "hint", "scope": "inherit", "source": "todo"},
		{"file": "docs/long.txt", "scope": "open", "severity": "Inherit"}
	]}}`), &cfg)
	if err != nil {
		t.Fatal(err)
	}

	dir := filepath.FromSlash("/repo")
	got := Resolve(&cfg, dir).KnownIssues

	want := []KnownIssues{
		{File: filepath.Join(dir, ".cfmleditor-unresolved.txt"), Scope: "workspace", Severity: "warning", Generate: GenerateUnresolved},
		{File: filepath.Join(dir, "docs", "todo.txt"), Scope: "workspace", Severity: "hint", Source: "todo"},
		{File: filepath.Join(dir, "docs", "long.txt"), Scope: "open", Severity: "warning"},
		{File: filepath.Join(dir, ".cfmleditor-cflint.txt"), Scope: "workspace", Severity: "warning", Generate: GenerateCFLint},
	}

	if !slices.Equal(got, want) {
		t.Errorf("got\n%+v\nwant\n%+v", got, want)
	}

	// A block that sets neither: its files and the implicit reports get the
	// defaults.
	got = ResolveKnownIssues(&KnownIssuesConfig{Files: []KnownIssues{{File: "todo.txt", Scope: "inherit"}}}, dir)
	for _, k := range got {
		if k.Scope != "open" || k.Severity != "information" {
			t.Errorf("block without scope or severity: %+v", k)
		}
	}

	// No block at all: the implicit reports, with the defaults.
	got = ResolveKnownIssues(nil, dir)
	if len(got) != 2 || got[0].Scope != "open" || got[0].Severity != "information" || got[1].Generate != GenerateCFLint {
		t.Errorf("no block: %+v", got)
	}
}

// TestGenerateTargets covers where each kind of report is written: the entries
// marked for it, or its default file beside the config when none is.
func TestGenerateTargets(t *testing.T) {
	dir := filepath.FromSlash("/repo")
	list := ResolveKnownIssues(&KnownIssuesConfig{Files: []KnownIssues{
		{File: "todo.txt"},
		{File: "tassweb/.unresolved.txt", Generate: "unresolved"},
		{File: "kiosk/.unresolved.txt", Generate: "Unresolved"},
	}}, dir)

	got := GenerateTargets(list, GenerateUnresolved, dir)
	if len(got) != 2 || got[0] != filepath.Join(dir, "tassweb", ".unresolved.txt") || got[1] != filepath.Join(dir, "kiosk", ".unresolved.txt") {
		t.Errorf("unresolved: %v", got)
	}

	if got := GenerateTargets(list, GenerateCFLint, dir); len(got) != 1 || got[0] != filepath.Join(dir, ".cfmleditor-cflint.txt") {
		t.Errorf("cflint default: %v", got)
	}
}
