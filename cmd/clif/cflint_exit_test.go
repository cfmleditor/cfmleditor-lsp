package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runClif runs the CLI with args in a child process, through the hook
// TestEverySubcommandRejectsAnUnknownOption installs, and returns its combined
// output and exit status.
func runClif(t *testing.T, dir string, args ...string) (string, int) {
	t.Helper()

	return runClifEnv(t, dir, nil, args...)
}

// runClifEnv is runClif with extra environment variables.
func runClifEnv(t *testing.T, dir string, env []string, args ...string) (string, int) {
	t.Helper()

	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.CommandContext(t.Context(), exe, "-test.run=^TestEverySubcommandRejectsAnUnknownOption$")
	cmd.Dir = dir

	cmd.Env = append(append(os.Environ(), env...), "CLIF_CLI_TEST_ARGS="+strings.Join(args, "\x1f"))

	out, err := cmd.CombinedOutput()

	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		return string(out), exit.ExitCode()
	}

	if err != nil {
		t.Fatal(err)
	}

	return string(out), 0
}

// TestVersionFlagsPrintAndExit: `<binary> --version` is how tools check a
// binary is installed, and it used to start the language server, which waits
// on stdin and never returns.
func TestVersionFlagsPrintAndExit(t *testing.T) {
	for _, flag := range []string{"version", "--version", "-version", "-v"} {
		out, code := runClif(t, t.TempDir(), flag)
		if code != 0 || !strings.HasPrefix(out, "clif ") {
			t.Errorf("clif %s: exit %d, output %q; want exit 0 and the version", flag, code, out)
		}
	}
}

// TestCFLintErrorsAreNotFindings: a run that could not be trusted exits 2,
// before CFLint is needed, so a gate never reads it as clean (0) or as the
// code's findings (1).
func TestCFLintErrorsAreNotFindings(t *testing.T) {
	dir := t.TempDir()

	if out, code := runClif(t, dir, "cflint", filepath.Join(dir, "missing.cfm")); code != exitCFLintError {
		t.Errorf("a path that does not exist: exit %d, want %d; output:\n%s", code, exitCFLintError, out)
	}

	if err := os.WriteFile(filepath.Join(dir, ".cflintrc"), []byte(`{"includes": [], }`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "page.cfm"), []byte("<cfoutput>x</cfoutput>\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	out, code := runClif(t, dir, "cflint", dir)
	if code != exitCFLintError || !strings.Contains(out, ".cflintrc") {
		t.Errorf("a .cflintrc CFLint cannot parse: exit %d, want %d naming the file; output:\n%s", code, exitCFLintError, out)
	}
}

func TestCFLintExitCode(t *testing.T) {
	cases := []struct {
		findings int
		writing  bool
		want     int
	}{
		{0, false, 0},
		{3, false, exitCFLintFindings},
		// A written report is a regeneration, expected to hold findings.
		{3, true, 0},
		{0, true, 0},
	}

	for _, c := range cases {
		if got := cflintExitCode(c.findings, c.writing); got != c.want {
			t.Errorf("cflintExitCode(%d, %v) = %d, want %d", c.findings, c.writing, got, c.want)
		}
	}
}
