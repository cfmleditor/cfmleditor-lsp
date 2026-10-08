package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestParseAddedLines(t *testing.T) {
	diff := `diff --git a/a.cfm b/a.cfm
--- a/a.cfm
+++ b/a.cfm
@@ -3,0 +4,2 @@ x
+one
+two
@@ -10 +12 @@
-old
+new
@@ -20,2 +21,0 @@
-gone
-gone
diff --git a/gone.cfm b/gone.cfm
--- a/gone.cfm
+++ /dev/null
@@ -1 +0,0 @@
-x
diff --git "a/has \"q\".cfm" "b/has \"q\".cfm"
--- "a/has \"q\".cfm"
+++ "b/has \"q\".cfm"
@@ -0,0 +1 @@
+x
`
	repo := filepath.FromSlash("/repo")
	got := parseAddedLines(repo, diff)

	lines := func(file string) []int {
		var out []int
		for n := range got[file] {
			out = append(out, n)
		}

		slices.Sort(out)

		return out
	}

	if want := []int{4, 5, 12}; !slices.Equal(lines(filepath.Join(repo, "a.cfm")), want) {
		t.Errorf("a.cfm added lines = %v, want %v (a removal-only hunk adds none)", lines(filepath.Join(repo, "a.cfm")), want)
	}

	if _, ok := got[filepath.Join(repo, "gone.cfm")]; ok {
		t.Error("a deleted file has added lines")
	}

	if want := []int{1}; !slices.Equal(lines(filepath.Join(repo, `has "q".cfm`)), want) {
		t.Errorf("a quoted name: %v, want %v; keys %v", lines(filepath.Join(repo, `has "q".cfm`)), want, got)
	}
}

// TestChangedLinesAreTheStagedOnes: --changed-lines keeps a finding only on a
// line the commit adds or alters, read from git's own diff of the index.
func TestChangedLinesAreTheStagedOnes(t *testing.T) {
	repo := gitRepo(t, map[string]string{"a.cfm": "1\n2\n3\n4\n"})

	writeTestFile(t, filepath.Join(repo, "a.cfm"), "1\nTWO\n3\n4\nfive\n")
	gitIn(t, repo, "add", "a.cfm")
	// Unstaged on top: not part of the commit.
	writeTestFile(t, filepath.Join(repo, "a.cfm"), "ONE\nTWO\n3\n4\nfive\n")

	added, err := gitAddedLines(repo, true, "")
	if err != nil {
		t.Fatal(err)
	}

	var got []int
	for n := range added[filepath.Join(repo, "a.cfm")] {
		got = append(got, n)
	}

	slices.Sort(got)

	if want := []int{2, 5}; !slices.Equal(got, want) {
		t.Errorf("staged added lines = %v, want %v", got, want)
	}
}

// TestHookInstallLeavesAForeignHookAlone: install writes its hooks and sets
// core.hooksPath, rewrites its own hook freely, refuses one it did not write
// unless forced, and uninstall takes away only what it wrote.
func TestHookInstallLeavesAForeignHookAlone(t *testing.T) {
	repo := gitRepo(t, map[string]string{"page.cfm": "x"})

	if out, code := runClif(t, repo, "hook", "install", "--log", "~/my log's.txt"); code != 0 {
		t.Fatalf("install: exit %d\n%s", code, out)
	}

	pre, err := os.ReadFile(filepath.Join(repo, ".githooks", "pre-commit"))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(pre), hookMarker) || !strings.Contains(string(pre), "clif hook run pre-commit") {
		t.Errorf("pre-commit hook:\n%s", pre)
	}

	if got := gitConfig(repo, "core.hooksPath"); got != ".githooks" {
		t.Errorf("core.hooksPath = %q", got)
	}

	if _, err := os.Stat(filepath.Join(repo, ".githooks", "post-commit")); err != nil {
		t.Errorf("--log wrote no post-commit hook: %v", err)
	}

	// Its own hook is rewritten without --force.
	if out, code := runClif(t, repo, "hook", "install", "--changed-lines"); code != 0 {
		t.Fatalf("reinstall: exit %d\n%s", code, out)
	}

	foreign := filepath.Join(repo, ".githooks", "pre-commit")
	writeTestFile(t, foreign, "#!/bin/sh\nnpm test\n")

	if out, code := runClif(t, repo, "hook", "install"); code != exitCFLintError {
		t.Errorf("over a foreign hook: exit %d, want %d\n%s", code, exitCFLintError, out)
	}

	if data, _ := os.ReadFile(foreign); string(data) != "#!/bin/sh\nnpm test\n" {
		t.Errorf("the foreign hook was changed:\n%s", data)
	}

	if out, code := runClif(t, repo, "hook", "install", "--force"); code != 0 {
		t.Fatalf("--force: exit %d\n%s", code, out)
	}

	if out, code := runClif(t, repo, "hook", "uninstall"); code != 0 {
		t.Fatalf("uninstall: exit %d\n%s", code, out)
	}

	if _, err := os.Stat(foreign); !os.IsNotExist(err) {
		t.Error("uninstall left its pre-commit hook")
	}

	if got := gitConfig(repo, "core.hooksPath"); got != "" {
		t.Errorf("uninstall left core.hooksPath = %q", got)
	}
}

// TestHookCheckCatchesAMissingHook: a repository with a .cflintrc and no clif
// hook fails the check, so a clone that never ran install is caught.
func TestHookCheckCatchesAMissingHook(t *testing.T) {
	repo := gitRepo(t, map[string]string{"page.cfm": "x"})

	if out, code := runClif(t, repo, "hook", "check"); code != 0 {
		t.Errorf("no .cflintrc: exit %d, want 0\n%s", code, out)
	}

	writeTestFile(t, filepath.Join(repo, "src", ".cflintrc"), "{}")

	if out, code := runClif(t, repo, "hook", "check"); code != exitCFLintFindings {
		t.Errorf("a .cflintrc and no hook: exit %d, want %d\n%s", code, exitCFLintFindings, out)
	}

	if out, code := runClif(t, repo, "hook", "install"); code != 0 {
		t.Fatalf("install: exit %d\n%s", code, out)
	}

	if out, code := runClif(t, repo, "hook", "check"); code != 0 {
		t.Errorf("installed: exit %d, want 0\n%s", code, out)
	}
}

// TestHookSkipIsLogged: SKIP passes the commit and still writes the log line,
// as SKIP, with the placeholders the post-commit hook fills.
func TestHookSkipIsLogged(t *testing.T) {
	repo := gitRepo(t, map[string]string{"page.cfm": "x"})
	gitIn(t, repo, "checkout", "-q", "-b", "feature/ABC-42-thing")

	log := filepath.Join(t.TempDir(), "logs", "commits.log")

	out, code := runClifEnv(t, repo, []string{"SKIP=lint,cflint", "CLIF_COMMIT_LOG=", "CFLINT_COMMIT_LOG="}, "hook", "run", "pre-commit", "--log", log)
	if code != 0 {
		t.Fatalf("skipped pre-commit: exit %d\n%s", code, out)
	}

	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}

	f := strings.Split(strings.TrimSpace(string(data)), " | ")
	if len(f) != 6 || f[1] != "ABC-42" || f[2] != placeholder || f[3] != "SKIP" || f[4] != filepath.Base(repo) || f[5] != placeholder {
		t.Errorf("log line = %q", data)
	}
}

// TestCommitLogIsFilledAfterTheCommit: the post-commit hook fills this
// repository's latest placeholder line with the hash and subject, takes the
// ticket from the subject, and leaves other repositories' lines alone.
func TestCommitLogIsFilledAfterTheCommit(t *testing.T) {
	log := filepath.Join(t.TempDir(), "commits.log")
	writeTestFile(t, log, strings.Join([]string{
		"2026-01-01 10:00:00 | ABC-1 | -------- | FAIL | app | --------",
		"2026-01-01 10:05:00 | ABC-1 | -------- | PASS | app | --------",
		"2026-01-01 10:06:00 | - | -------- | PASS | other | --------",
	}, "\n")+"\n")

	if err := fillCommitLog(log, "app", "abc1234", "ABC-7 | fix the thing"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}

	got := strings.Split(strings.TrimSpace(string(data)), "\n")
	want := []string{
		"2026-01-01 10:00:00 | ABC-1 | -------- | FAIL | app | --------",
		"2026-01-01 10:05:00 | ABC-7 | abc1234 | PASS | app | ABC-7 / fix the thing",
		"2026-01-01 10:06:00 | - | -------- | PASS | other | --------",
	}

	if !slices.Equal(got, want) {
		t.Errorf("log:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestShellQuoteSurvivesTheShell: the hook passes paths to clif through sh,
// so a quote or space in one must arrive intact.
func TestShellQuoteSurvivesTheShell(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}

	for _, s := range []string{"plain", "with space", "it's", `"double"`, "$HOME", "a;b"} {
		out, err := exec.CommandContext(context.Background(), "sh", "-c", "printf %s "+shellQuote(s)).Output()
		if err != nil || string(out) != s {
			t.Errorf("%q came through sh as %q (%v)", s, out, err)
		}
	}
}
