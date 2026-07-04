package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseArgs_NoTargetsIsError(t *testing.T) {
	if _, err := parseArgs(nil); err == nil {
		t.Error("expected an error when no targets are given")
	}

	if _, err := parseArgs([]string{"-profile", "cpu.prof"}); err == nil {
		t.Error("expected an error when only -profile is given with no targets")
	}
}

func TestParseArgs_PlainTargets(t *testing.T) {
	pa, err := parseArgs([]string{"a.cfc", "b.cfc"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pa.profilePath != "" {
		t.Errorf("expected no profile path, got %q", pa.profilePath)
	}

	if len(pa.targets) != 2 || pa.targets[0] != "a.cfc" || pa.targets[1] != "b.cfc" {
		t.Errorf("expected targets [a.cfc b.cfc], got %v", pa.targets)
	}
}

func TestParseArgs_ProfileFlagExtracted(t *testing.T) {
	pa, err := parseArgs([]string{"-profile", "cpu.prof", "a.cfc"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pa.profilePath != "cpu.prof" {
		t.Errorf("expected profilePath cpu.prof, got %q", pa.profilePath)
	}

	if len(pa.targets) != 1 || pa.targets[0] != "a.cfc" {
		t.Errorf("expected targets [a.cfc], got %v", pa.targets)
	}
}

func TestParseArgs_MinusProfileAsBareTargetNotConfusedWithFlag(t *testing.T) {
	// "-profile" alone (no following path) isn't the flag form; it must be treated as a
	// single file/dir target, matching the original len(args) >= 2 guard.
	pa, err := parseArgs([]string{"-profile"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pa.profilePath != "" {
		t.Errorf("expected no profile path when -profile has no following arg, got %q", pa.profilePath)
	}

	if len(pa.targets) != 1 || pa.targets[0] != "-profile" {
		t.Errorf("expected \"-profile\" itself treated as a target, got %v", pa.targets)
	}
}

func TestCollectFiles_FileTargetIncludedRegardlessOfExtension(t *testing.T) {
	dir := t.TempDir()
	notCFML := filepath.Join(dir, "readme.md")

	if err := os.WriteFile(notCFML, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := collectFiles([]string{notCFML})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(files) != 1 || files[0] != notCFML {
		t.Errorf("expected the explicitly named file to be included regardless of extension, got %v", files)
	}
}

func TestCollectFiles_DirectoryFiltersByExtension(t *testing.T) {
	dir := t.TempDir()

	mustWrite := func(rel string) {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(full, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	mustWrite("a.cfc")
	mustWrite("b.cfm")
	mustWrite("c.cfml")
	mustWrite("d.cfs")
	mustWrite("readme.md")
	mustWrite("nested/deep/e.cfc")

	files, err := collectFiles([]string{dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(files) != 5 {
		t.Fatalf("expected 5 CFML files (a,b,c,d,nested/e), got %d: %v", len(files), files)
	}

	for _, f := range files {
		if filepath.Base(f) == "readme.md" {
			t.Errorf("expected readme.md to be excluded from a directory walk, got %v", files)
		}
	}
}

func TestCollectFiles_NonexistentPathIsError(t *testing.T) {
	_, err := collectFiles([]string{"/definitely/does/not/exist/xyz"})
	if err == nil {
		t.Error("expected an error for a nonexistent path")
	}
}

func TestRunBenchmark_ParsesFilesAndAggregatesStats(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.cfc")

	content := `component {
	function getUser() { return new models.User(); }
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer

	stats := runBenchmark([]string{path}, &out, &errOut)

	if stats.Files != 1 {
		t.Errorf("expected Files=1, got %d", stats.Files)
	}

	if stats.Funcs != 1 {
		t.Errorf("expected Funcs=1, got %d", stats.Funcs)
	}

	if errOut.Len() != 0 {
		t.Errorf("expected no skip notices, got %q", errOut.String())
	}

	if !strings.Contains(out.String(), "funcs=1") {
		t.Errorf("expected per-file summary line in stdout, got %q", out.String())
	}
}

func TestRunBenchmark_UnreadableFileSkippedNotFatal(t *testing.T) {
	dir := t.TempDir()
	goodPath := filepath.Join(dir, "good.cfc")

	if err := os.WriteFile(goodPath, []byte(`component { function f() {} }`), 0o644); err != nil {
		t.Fatal(err)
	}

	missingPath := filepath.Join(dir, "missing.cfc")

	var out, errOut bytes.Buffer

	stats := runBenchmark([]string{missingPath, goodPath}, &out, &errOut)

	if stats.Files != 1 {
		t.Errorf("expected only the readable file counted, got Files=%d", stats.Files)
	}

	if !strings.Contains(errOut.String(), "skip") || !strings.Contains(errOut.String(), "missing.cfc") {
		t.Errorf("expected a skip notice on errOut for the missing file, got %q", errOut.String())
	}

	if strings.Contains(out.String(), "missing.cfc") {
		t.Errorf("expected the skip notice to go to errOut, not stdout: %q", out.String())
	}
}

func TestPrintSummary_ZeroFilesNoDivideByZero(t *testing.T) {
	var out bytes.Buffer

	printSummary(&out, benchStats{})

	if !strings.Contains(out.String(), "total: 0 files") {
		t.Errorf("expected a zero-files summary without panicking, got %q", out.String())
	}
}

func TestPrintSummary_AverageComputedFromTotals(t *testing.T) {
	var out bytes.Buffer

	printSummary(&out, benchStats{Files: 2, Funcs: 4, Refs: 6, Dur: 100 * time.Millisecond})

	got := out.String()
	for _, want := range []string{"2 files", "4 funcs", "6 refs", "avg 50ms/file"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected summary to contain %q, got %q", want, got)
		}
	}
}
