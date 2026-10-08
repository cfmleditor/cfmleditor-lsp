package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// gitRepo makes a repository in a temporary directory, resolved past any
// symlink (macOS's /tmp), with one commit holding files.
func gitRepo(t *testing.T, files map[string]string) string {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	for name, content := range files {
		writeTestFile(t, filepath.Join(dir, name), content)
	}

	for _, args := range [][]string{
		{"init", "-q"},
		{"add", "-A"},
		{"-c", "user.email=t@example.com", "-c", "user.name=t", "commit", "-q", "-m", "init"},
	} {
		gitIn(t, dir, args...)
	}

	return dir
}

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func rel(t *testing.T, base string, paths []string) []string {
	t.Helper()

	out := make([]string, 0, len(paths))

	for _, p := range paths {
		r, err := filepath.Rel(base, p)
		if err != nil {
			t.Fatal(err)
		}

		out = append(out, filepath.ToSlash(r))
	}

	slices.Sort(out)

	return out
}

// TestStagedLintsTheIndexNotTheWorkingTree: a pre-commit hook judges what is
// being committed, so an unstaged fix must not hide a staged problem, and an
// unstaged problem must not block a clean commit. Deleted and non-CFML files
// are left out; a name with a space survives.
func TestStagedLintsTheIndexNotTheWorkingTree(t *testing.T) {
	repo := gitRepo(t, map[string]string{"keep.cfm": "x", "gone.cfm": "x", "sub/old.cfc": "x"})

	writeTestFile(t, filepath.Join(repo, "has space.cfm"), "<cfdump var=\"staged\">")
	writeTestFile(t, filepath.Join(repo, "sub", "old.cfc"), "component { staged }")
	writeTestFile(t, filepath.Join(repo, "notes.txt"), "not CFML")
	gitIn(t, repo, "add", "-A")
	gitIn(t, repo, "rm", "-q", "gone.cfm")

	// Edited after staging: the hook must lint what was staged.
	writeTestFile(t, filepath.Join(repo, "has space.cfm"), "<cfoutput>fixed but unstaged</cfoutput>")
	// Changed and not staged at all: not part of the commit.
	writeTestFile(t, filepath.Join(repo, "keep.cfm"), "<cfdump var=\"unstaged\">")

	fl := cflintFlags{staged: true, format: "report"}
	sel := selectCFLintFiles(&fl, repo, []string{repo}, []string{repo})

	if got, want := rel(t, repo, sel.files), []string{"has space.cfm", "sub/old.cfc"}; !slices.Equal(got, want) {
		t.Fatalf("staged files = %v, want %v", got, want)
	}

	for _, s := range sel.sources {
		if strings.Contains(string(s.Content), "unstaged") {
			t.Errorf("%s: linted the working tree (%q), not the index", s.Path, s.Content)
		}
	}

	// From a subdirectory with no paths given, the whole repository's staged
	// files, as git diff --cached reports them; with a path, those under it.
	sub := filepath.Join(repo, "sub")
	if got := rel(t, repo, selectCFLintFiles(&fl, sub, []string{sub}, []string{sub}).files); len(got) != 2 {
		t.Errorf("from sub/ with no paths: %v, want both staged files", got)
	}

	narrowed := cflintFlags{staged: true, format: "report", roots: []string{sub}}
	if got := rel(t, repo, selectCFLintFiles(&narrowed, sub, []string{sub}, []string{sub}).files); !slices.Equal(got, []string{"sub/old.cfc"}) {
		t.Errorf("narrowed to sub/: %v", got)
	}
}

// TestChangedListsFilesSinceTheMergeBase: CI lints what a branch changed,
// three-dot, so changes on the base branch since it forked are not the
// branch's.
func TestChangedListsFilesSinceTheMergeBase(t *testing.T) {
	repo := gitRepo(t, map[string]string{"a.cfm": "x", "b.cfm": "x"})

	gitIn(t, repo, "branch", "base")
	writeTestFile(t, filepath.Join(repo, "a.cfm"), "changed on the branch")
	writeTestFile(t, filepath.Join(repo, "new.cfc"), "added on the branch")
	gitIn(t, repo, "add", "-A")
	gitIn(t, repo, "-c", "user.email=t@example.com", "-c", "user.name=t", "commit", "-q", "-m", "branch work")

	// A change on base after the fork must not count.
	gitIn(t, repo, "checkout", "-q", "base")
	writeTestFile(t, filepath.Join(repo, "b.cfm"), "changed on base")
	gitIn(t, repo, "-c", "user.email=t@example.com", "-c", "user.name=t", "commit", "-q", "-am", "base work")
	gitIn(t, repo, "checkout", "-q", "-")

	fl := cflintFlags{changed: "base", format: "report"}
	sel := selectCFLintFiles(&fl, repo, []string{repo}, []string{repo})

	if got, want := rel(t, repo, sel.files), []string{"a.cfm", "new.cfc"}; !slices.Equal(got, want) {
		t.Errorf("changed files = %v, want %v", got, want)
	}

	if sel.sources != nil {
		t.Error("--changed read staged content; it lints the working tree")
	}
}

// TestCFLintFlagsRefuseNonsense: each combination that cannot mean anything
// exits 2, before any file is read or CFLint is needed.
func TestCFLintFlagsRefuseNonsense(t *testing.T) {
	dir := t.TempDir()

	for _, args := range [][]string{
		{"--format", "xml", dir},
		{"--write", "--format", "json", dir},
		{"--write", "--out", filepath.Join(dir, "r.txt"), dir},
		{"--staged", "--changed", "main"},
		{"--strict", "--min-severity", "ERROR", dir},
		{"--min-severity", "LOUD", dir},
		{"--format"},
	} {
		if out, code := runClif(t, dir, append([]string{"cflint"}, args...)...); code != exitCFLintError {
			t.Errorf("clif cflint %v: exit %d, want %d; output:\n%s", args, code, exitCFLintError, out)
		}
	}
}

// TestOnlyARegeneratedReportIgnoresFindings: --out with a known-issues report
// is a regeneration; --out with JSON or SARIF is a CI artefact, and the run
// must still fail on findings.
func TestOnlyARegeneratedReportIgnoresFindings(t *testing.T) {
	cases := []struct {
		fl   cflintFlags
		want bool
	}{
		{cflintFlags{format: "report"}, false},
		{cflintFlags{format: "report", out: "r.txt"}, true},
		{cflintFlags{format: "report", write: true}, true},
		{cflintFlags{format: "json", out: "r.json"}, false},
		{cflintFlags{format: "sarif", out: "r.sarif"}, false},
	}

	for _, c := range cases {
		if got := c.fl.regenerating(); got != c.want {
			t.Errorf("%+v: regenerating = %v, want %v", c.fl, got, c.want)
		}
	}
}

// TestCFLintOptionsTakeTheirValues: an option's value is consumed with it, in
// both spellings, and is never also read as a path. A loop that stopped
// skipping values passed every refusal test, since a bad combination exits 2
// either way, while --format json linted a directory called json.
func TestCFLintOptionsTakeTheirValues(t *testing.T) {
	fl := parseCFLintFlags([]string{"--format", "json", "--min-severity=ERROR", "--out", "r.json", "src", "--changed", "origin/main", "lib"})

	if fl.format != "json" || fl.minSeverity != "ERROR" || fl.out != "r.json" || fl.changed != "origin/main" {
		t.Errorf("values = format %q, min-severity %q, out %q, changed %q", fl.format, fl.minSeverity, fl.out, fl.changed)
	}

	if !slices.Equal(fl.roots, []string{"src", "lib"}) {
		t.Errorf("paths = %v, want [src lib]: an option's value was read as a path", fl.roots)
	}
}
