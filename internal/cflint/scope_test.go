package cflint

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestCFLintLintsOnlyTheDirectoriesGiven(t *testing.T) {
	ws := filepath.FromSlash("/tassdev")
	tassweb := filepath.Join(ws, "tassweb")
	prs := filepath.Join(ws, "prs")
	kiosk := filepath.Join(ws, "kiosk")

	cases := []struct {
		name      string
		roots     []string
		configDir string
		folders   []string
		want      []string
	}{
		{
			// prs has no config of its own; the workspace's lists every repo.
			name: "a project under the workspace config", roots: []string{prs},
			configDir: ws, folders: []string{tassweb, prs, kiosk}, want: []string{prs},
		},
		{
			name: "the workspace config's own directory", roots: []string{ws},
			configDir: ws, folders: []string{tassweb, prs, kiosk}, want: []string{tassweb, prs, kiosk},
		},
		{
			// tassweb's own config lists its siblings for resolution.
			name: "a project whose config lists its siblings", roots: []string{tassweb},
			configDir: tassweb, folders: []string{kiosk, prs, tassweb}, want: []string{tassweb},
		},
		{
			name: "a config listing no folders", roots: []string{ws},
			configDir: ws, want: []string{ws},
		},
		{
			name: "several directories", roots: []string{prs, kiosk},
			configDir: ws, folders: []string{tassweb, prs, kiosk}, want: []string{prs, kiosk},
		},
	}

	for _, c := range cases {
		if got := ScanRoots(c.roots, c.configDir, c.folders); !slices.Equal(got, c.want) {
			t.Errorf("%s: scan roots = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestCFLintWriteNeverRewritesAReportFromAPartialScan(t *testing.T) {
	ws := filepath.FromSlash("/tassdev")
	prs := filepath.Join(ws, "prs")
	wsReport := filepath.Join(ws, ".cfmleditor-cflint.txt")
	prsReport := filepath.Join(prs, ".cfmleditor-cflint.txt")
	kioskReport := filepath.Join(ws, "kiosk", ".cfmleditor-cflint.txt")

	if _, err := WriteTargets([]string{wsReport}, []string{prs}); err == nil ||
		!strings.Contains(err.Error(), wsReport) {
		t.Errorf("a workspace report from a prs scan: err = %v, want a refusal naming it", err)
	}

	got, err := WriteTargets([]string{prsReport, kioskReport}, []string{prs})
	if err != nil || !slices.Equal(got, []string{prsReport}) {
		t.Errorf("prs and kiosk reports from a prs scan = %v, %v; want only prs's", got, err)
	}

	if _, err := WriteTargets([]string{kioskReport}, []string{prs}); err == nil {
		t.Error("no report under the scan: want an error rather than writing nothing")
	}

	// The config's own directory stands for its workspace: ScanRoots expands
	// it to the folders beneath, and the workspace report is still its own.
	if got, err := WriteTargets([]string{wsReport}, []string{ws}); err != nil || !slices.Equal(got, []string{wsReport}) {
		t.Errorf("the workspace report from a workspace lint = %v, %v; want it written", got, err)
	}

	got, err = WriteTargets([]string{wsReport, prsReport}, []string{ws})
	if err != nil || !slices.Equal(got, []string{wsReport, prsReport}) {
		t.Errorf("a whole-workspace scan = %v, %v; want both reports", got, err)
	}
}

func TestWithin(t *testing.T) {
	cases := []struct {
		path, dir string
		want      bool
	}{
		{"/a/b", "/a", true},
		{"/a", "/a", true},
		{"/ab", "/a", false},
		{"/a", "/a/b", false},
		{"/c", "/a", false},
	}

	for _, c := range cases {
		if got := Within(filepath.FromSlash(c.path), filepath.FromSlash(c.dir)); got != c.want {
			t.Errorf("within(%q, %q) = %v, want %v", c.path, c.dir, got, c.want)
		}
	}
}
