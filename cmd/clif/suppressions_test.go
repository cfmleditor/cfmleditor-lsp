package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSuppressionsRatchet: the baseline is written, an unchanged tree passes,
// one more suppression fails with exit 1 naming the rule, and a missing
// baseline is an error rather than a pass.
func TestSuppressionsRatchet(t *testing.T) {
	dir := t.TempDir()
	page := filepath.Join(dir, "page.cfm")
	baseline := filepath.Join(dir, "suppressions.json")

	writeTestFile(t, page, "<!--- @CFLintIgnore AVOID_USING_CFDUMP_TAG --->\n<cfdump var=\"x\">\n")

	if out, code := runClif(t, dir, "suppressions", "--baseline", baseline, "--update-baseline", dir); code != 0 {
		t.Fatalf("--update-baseline: exit %d\n%s", code, out)
	}

	if out, code := runClif(t, dir, "suppressions", "--baseline", baseline, dir); code != 0 {
		t.Errorf("unchanged: exit %d, want 0\n%s", code, out)
	}

	writeTestFile(t, page, "<!--- @CFLintIgnore AVOID_USING_CFDUMP_TAG --->\n<cfdump var=\"x\">\n<!--- @CFLintIgnore AVOID_USING_CFDUMP_TAG --->\n<cfdump var=\"y\">\n")

	out, code := runClif(t, dir, "suppressions", "--baseline", baseline, dir)
	if code != exitCFLintFindings || !strings.Contains(out, "AVOID_USING_CFDUMP_TAG") {
		t.Errorf("one more suppression: exit %d, want %d naming the rule\n%s", code, exitCFLintFindings, out)
	}

	if err := os.Remove(baseline); err != nil {
		t.Fatal(err)
	}

	if out, code := runClif(t, dir, "suppressions", "--baseline", baseline, dir); code != exitCFLintError {
		t.Errorf("missing baseline: exit %d, want %d\n%s", code, exitCFLintError, out)
	}
}

// TestCFLintBaselineMustExist: a missing findings baseline is exit 2 before
// any linting, never "every finding is new" or "nothing to report".
func TestCFLintBaselineMustExist(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "page.cfm"), "x")

	if out, code := runClif(t, dir, "cflint", "--baseline", filepath.Join(dir, "missing.txt"), dir); code != exitCFLintError {
		t.Errorf("exit %d, want %d\n%s", code, exitCFLintError, out)
	}
}
