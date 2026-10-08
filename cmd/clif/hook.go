package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	cflog "github.com/cfmleditor/clif/internal/log"
)

const hookUsage = `usage: clif hook install [--dir <dir>] [--changed-lines] [--log <file>] [--force]
       clif hook uninstall [--dir <dir>]
       clif hook check
       clif hook run pre-commit [--changed-lines] [--log <file>]
       clif hook run post-commit [--log <file>]

A git pre-commit hook that runs CFLint over what is being committed, through
clif cflint --staged --strict: the staged content, not the working tree, every
CFLint level failing the commit.

  install     write pre-commit (and, with --log, post-commit) into <dir>
              (default .githooks, at the repository's root) and point this
              clone's core.hooksPath at it. Commit <dir>, so the hook is in
              the repository; git does not copy core.hooksPath, so each clone
              runs install once, and check catches one that has not. A hook
              there that clif did not write is left alone unless --force
  uninstall   remove the hooks install wrote, and core.hooksPath if it points
              at them
  check       exit 1 when the repository has a .cflintrc and its pre-commit
              hook does not run clif, so a clone without the hook is caught
              (by CI, or a setup check) rather than silently not linting
  run         what the hooks call

  --changed-lines   fail only on findings in lines the commit adds or alters;
                    a finding elsewhere in a touched file was there before it
  --log <file>      append one line per commit attempt to <file>:
                      TIMESTAMP | TICKET | HASH | PASS/FAIL | REPO | MESSAGE
                    The pre-commit hook writes "--------" for the hash and
                    message, and the post-commit hook fills them in, so a
                    commit the hook blocked keeps the placeholder. TICKET is
                    the first ABC-123 in the commit subject, else the branch
                    name. A skipped commit is logged as SKIP.
                    CLIF_COMMIT_LOG (or CFLINT_COMMIT_LOG) overrides the path

Skip the hook for one commit with SKIP=cflint (or clif), or PREK_SKIP=cflint.

Exit status:
  0  passed, skipped, or done
  1  the commit has findings (run pre-commit), or the hook is missing (check)
  2  not a git repository, a hook in the way, or a usage error
`

// hookMarker identifies a hook file clif wrote, so install rewrites its own
// and leaves anyone else's alone.
const hookMarker = "# clif hook: written by `clif hook install`; change it with that command."

// clifExecutable is the binary the pre-commit hook runs clif cflint with:
// this one. A var so a test, where this binary is the test binary, can make
// it fail fast rather than have the hook start the test binary again.
var clifExecutable = os.Executable

// placeholder is the commit log's hash and message before the commit exists.
const placeholder = "--------"

// ticketRe finds an issue key such as ABC-123.
var ticketRe = regexp.MustCompile(`\b[A-Z][A-Z0-9]+-\d+\b`)

func cmdHook(args []string) {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, hookUsage)
		os.Exit(exitCFLintError)
	}

	switch args[0] {
	case "install":
		hookInstall(args[1:])
	case "uninstall":
		hookUninstall(args[1:])
	case "check":
		hookCheck(args[1:])
	case "run":
		if len(args) < 2 {
			cflintFailf("clif hook run needs pre-commit or post-commit\n\n%s", hookUsage)
		}

		switch args[1] {
		case "pre-commit":
			os.Exit(hookPreCommit(args[2:]))
		case "post-commit":
			hookPostCommit(args[2:])
		default:
			cflintFailf("unknown hook %q\n\n%s", args[1], hookUsage)
		}
	default:
		positionalOr(args[0], hookUsage, exitCFLintError)
		cflintFailf("unknown hook command %q\n\n%s", args[0], hookUsage)
	}
}

type hookFlags struct {
	dir   string
	log   string
	lines bool
	force bool
}

func parseHookFlags(args []string, allowed ...string) hookFlags {
	fl := hookFlags{dir: ".githooks"}
	rest := args

	for len(rest) > 0 {
		arg := rest[0]
		rest = rest[1:]

		if !strings.HasPrefix(arg, "-") {
			cflintFailf("unexpected argument %q\n\n%s", arg, hookUsage)
		}

		if !slices.Contains(allowed, arg) {
			cflintFailf("unknown option %q\n\n%s", arg, hookUsage)
		}

		value := func() string {
			if len(rest) == 0 {
				cflintFailf("%s needs a value\n\n%s", arg, hookUsage)
			}

			v := rest[0]
			rest = rest[1:]

			return v
		}

		switch arg {
		case "--dir":
			fl.dir = value()
		case "--log":
			fl.log = value()
		case "--changed-lines":
			fl.lines = true
		case "--force":
			fl.force = true
		}
	}

	return fl
}

// hookRepo is the repository holding the working directory.
func hookRepo() string {
	wd, err := os.Getwd()
	if err != nil {
		cflintFailf("%v\n", err)
	}

	repo, err := gitTopLevel(wd)
	if err != nil {
		cflintFailf("%v\n", err)
	}

	return repo
}

// shellQuote quotes s for a POSIX shell: git runs hooks through sh on every
// platform, Git for Windows included.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// hookScript is a hook file running `clif hook run <name> <args>`. It checks
// clif is on PATH first, so a machine without it fails the commit saying why,
// rather than with sh's "command not found". SKIP is clif's to honour, so a
// skipped commit is logged; the script honours it only when clif is missing,
// so a commit can still be made on such a machine.
func hookScript(name string, args []string) string {
	quoted := make([]string, 0, len(args))
	for _, a := range args {
		quoted = append(quoted, shellQuote(a))
	}

	run := strings.TrimSpace("clif hook run " + name + " " + strings.Join(quoted, " "))

	missing := `  case ",${SKIP:-},${PREK_SKIP:-}," in
    *,cflint,*|*,clif,*) exit 0 ;;
  esac
  echo "clif is not on PATH, and this repository's pre-commit hook runs it." >&2
  echo "Install it (https://github.com/cfmleditor/clif#install), or skip this once with SKIP=cflint." >&2
  exit 1`
	if name == "post-commit" {
		// After the commit there is nothing to block: say nothing.
		missing = "  exit 0"
	}

	return fmt.Sprintf(`#!/bin/sh
%s
if ! command -v clif >/dev/null 2>&1; then
%s
fi
exec %s
`, hookMarker, missing, run)
}

func hookInstall(args []string) {
	fl := parseHookFlags(args, "--dir", "--log", "--changed-lines", "--force")
	repo := hookRepo()

	dir := fl.dir
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(repo, dir)
	}

	var preArgs []string
	if fl.lines {
		preArgs = append(preArgs, "--changed-lines")
	}

	hooks := map[string]string{}

	if fl.log != "" {
		preArgs = append(preArgs, "--log", fl.log)
		hooks["post-commit"] = hookScript("post-commit", []string{"--log", fl.log})
	}

	hooks["pre-commit"] = hookScript("pre-commit", preArgs)

	for name := range hooks {
		path := filepath.Join(dir, name)

		data, err := os.ReadFile(path)
		if err == nil && !bytes.Contains(data, []byte(hookMarker)) && !fl.force {
			cflintFailf("%s exists and clif did not write it. Add a line running\n  clif hook run %s\nto it, or replace it with --force.\n", path, name)
		}
	}

	if err := os.MkdirAll(dir, 0o750); err != nil {
		cflintFailf("%v\n", err)
	}

	for name, script := range hooks {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(script), 0o755); err != nil { //nolint:gosec // a hook must be executable
			cflintFailf("could not write %s: %v\n", path, err)
		}
	}

	rel, err := filepath.Rel(repo, dir)
	if err != nil || strings.HasPrefix(rel, "..") {
		rel = dir
	}

	rel = filepath.ToSlash(rel)

	if prev := gitConfig(repo, "core.hooksPath"); prev != rel {
		if _, err := git(repo, "config", "core.hooksPath", rel); err != nil {
			cflintFailf("%v\n", err)
		}

		if prev != "" {
			fmt.Fprintf(os.Stderr, "core.hooksPath was %s; this clone no longer runs the hooks there.\n", prev)
		}
	}

	fmt.Fprintf(os.Stderr, "Installed the clif pre-commit hook in %s and set core.hooksPath to it.\n", rel)
	fmt.Fprintf(os.Stderr, "Commit %s so every clone has it. Git does not copy core.hooksPath, so each clone runs `clif hook install` once; `clif hook check` fails on one that has not.\n", rel)
}

// gitConfig is a git setting's effective value, or "" when unset.
func gitConfig(repo, key string) string {
	out, err := git(repo, "config", "--get", key)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(out))
}

func hookUninstall(args []string) {
	fl := parseHookFlags(args, "--dir")
	repo := hookRepo()

	dir := fl.dir
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(repo, dir)
	}

	for _, name := range []string{"pre-commit", "post-commit"} {
		path := filepath.Join(dir, name)

		data, err := os.ReadFile(path)
		if err != nil || !bytes.Contains(data, []byte(hookMarker)) {
			continue
		}

		if err := os.Remove(path); err != nil {
			cflintFailf("%v\n", err)
		}

		fmt.Fprintf(os.Stderr, "Removed %s\n", path)
	}

	rel, _ := filepath.Rel(repo, dir)

	if local, _ := git(repo, "config", "--local", "--get", "core.hooksPath"); strings.TrimSpace(string(local)) == filepath.ToSlash(rel) {
		if _, err := git(repo, "config", "--local", "--unset", "core.hooksPath"); err != nil {
			cflintFailf("%v\n", err)
		}

		fmt.Fprintf(os.Stderr, "Unset core.hooksPath\n")
	}
}

// runsClif reports whether a hook file, or a hook manager's config, runs clif.
func runsClif(data []byte) bool {
	return bytes.Contains(data, []byte("clif hook run pre-commit")) ||
		bytes.Contains(data, []byte("clif cflint")) ||
		bytes.Contains(data, []byte("id: clif-cflint")) ||
		bytes.Contains(data, []byte(`id = "clif-cflint"`))
}

func hookCheck(args []string) {
	parseHookFlags(args)

	repo := hookRepo()

	out, err := git(repo, "rev-parse", "--git-path", "hooks")
	if err != nil {
		cflintFailf("%v\n", err)
	}

	hooksDir := strings.TrimSpace(string(out))
	if !filepath.IsAbs(hooksDir) {
		hooksDir = filepath.Join(repo, hooksDir)
	}

	hook, err := os.ReadFile(filepath.Join(hooksDir, "pre-commit"))
	if err == nil && runsClif(hook) {
		fmt.Printf("ok: the pre-commit hook runs clif (%s)\n", filepath.Join(hooksDir, "pre-commit"))

		return
	}

	// A hook manager's committed config naming clif's hook counts once the
	// manager's own pre-commit hook is installed.
	for _, name := range []string{".pre-commit-config.yaml", "prek.toml"} {
		cfg, err := os.ReadFile(filepath.Join(repo, name))
		if err == nil && runsClif(cfg) && hook != nil {
			fmt.Printf("ok: %s runs clif's hook\n", name)

			return
		}
	}

	if !hasCFLintConfig(repo) {
		fmt.Println("ok: no .cflintrc here, so there is nothing for the hook to enforce")

		return
	}

	fmt.Printf("the pre-commit hook does not run clif in %s. Install it with:\n  clif hook install\n", repo)
	os.Exit(exitCFLintFindings)
}

// hasCFLintConfig reports whether a .cflintrc is anywhere under repo, outside
// dot-directories, which is where a project keeps its rules.
func hasCFLintConfig(repo string) bool {
	found := false

	_ = filepath.WalkDir(repo, func(path string, d os.DirEntry, err error) error {
		if err != nil || found {
			return filepath.SkipDir
		}

		if d.IsDir() && path != repo && strings.HasPrefix(d.Name(), ".") {
			return filepath.SkipDir
		}

		if !d.IsDir() && d.Name() == ".cflintrc" {
			found = true

			return filepath.SkipAll
		}

		return nil
	})

	return found
}

// skipped reports whether SKIP or PREK_SKIP (comma lists, as pre-commit and
// prek read them) names this hook.
func skipped() bool {
	for _, env := range []string{"SKIP", "PREK_SKIP"} {
		for id := range strings.SplitSeq(os.Getenv(env), ",") {
			if id = strings.TrimSpace(id); id == "cflint" || id == "clif" {
				return true
			}
		}
	}

	return false
}

// hookPreCommit lints the staged CFML, prints the findings and returns the
// exit status that decides the commit.
func hookPreCommit(args []string) int {
	fl := parseHookFlags(args, "--changed-lines", "--log")
	repo := hookRepo()

	if skipped() {
		fmt.Fprintln(os.Stderr, "clif: CFLint skipped (SKIP)")
		appendCommitLog(fl.log, repo, "SKIP")

		return 0
	}

	exe, err := clifExecutable()
	if err != nil {
		cflintFailf("%v\n", err)
	}

	lint := []string{"cflint", "--staged", "--strict", "-q"}
	if fl.lines {
		lint = append(lint, "--changed-lines")
	}

	cmd := exec.CommandContext(context.Background(), exe, lint...) //nolint:gosec // clif itself
	cmd.Dir = repo
	cmd.Stderr = os.Stderr

	var stdout bytes.Buffer

	cmd.Stdout = &stdout

	code := 0

	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			cflintFailf("running clif cflint: %v\n", err)
		}

		code = exit.ExitCode()
	}

	// The report's header is for a file; at a commit, the findings are what
	// to read.
	sc := bufio.NewScanner(&stdout)
	for sc.Scan() {
		if line := sc.Text(); !strings.HasPrefix(line, "#") {
			fmt.Println(line)
		}
	}

	status := map[int]string{0: "PASS", 1: "FAIL"}[code]
	if status == "" {
		status = "ERROR"
	}

	appendCommitLog(fl.log, repo, status)

	if code == exitCFLintFindings {
		fmt.Fprintln(os.Stderr, "\nclif: CFLint found issues in the staged files, so the commit was stopped. Fix them, or, for one that predates this change, suppress it where it is with <!--- @CFLintIgnore RULE ---> on the line above (// cflint ignore:RULE at the end of a script line).")
	}

	return code
}

// commitLogPath is where the commit log goes: CLIF_COMMIT_LOG, else
// CFLINT_COMMIT_LOG, else the hook's --log. "" means no log.
func commitLogPath(flag string) string {
	path := cflog.Getenv("CLIF_COMMIT_LOG", "CFLINT_COMMIT_LOG")
	if path == "" {
		path = flag
	}

	if rest, ok := strings.CutPrefix(path, "~"); ok {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, rest)
		}
	}

	return path
}

// logField keeps a value from breaking the log's " | " columns or its lines.
func logField(s string) string {
	s = strings.ReplaceAll(s, "|", "/")

	return strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
}

// appendCommitLog appends the pre-commit line, with the hash and message left
// as placeholders for the post-commit hook. A failure to log is reported and
// does not change the commit's outcome.
func appendCommitLog(flag, repo, status string) {
	path := commitLogPath(flag)
	if path == "" {
		return
	}

	ticket := "-"
	if branch := gitConfigless(repo, "rev-parse", "--abbrev-ref", "HEAD"); ticketRe.MatchString(branch) {
		ticket = ticketRe.FindString(branch)
	}

	line := strings.Join([]string{
		time.Now().Format("2006-01-02 15:04:05"),
		ticket,
		placeholder,
		status,
		logField(filepath.Base(repo)),
		placeholder,
	}, " | ")

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		fmt.Fprintf(os.Stderr, "clif: commit log: %v\n", err)

		return
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "clif: commit log: %v\n", err)

		return
	}

	defer func() { _ = f.Close() }()

	if _, err := fmt.Fprintln(f, line); err != nil {
		fmt.Fprintf(os.Stderr, "clif: commit log: %v\n", err)
	}
}

// gitConfigless runs git and returns its trimmed output, or "" on failure.
func gitConfigless(repo string, args ...string) string {
	out, err := git(repo, args...)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(out))
}

// hookPostCommit fills in the hash and message of this repository's latest
// placeholder line, and the ticket when the branch named none.
func hookPostCommit(args []string) {
	fl := parseHookFlags(args, "--log")

	path := commitLogPath(fl.log)
	if path == "" {
		return
	}

	repo := hookRepo()
	hash := gitConfigless(repo, "rev-parse", "--short", "HEAD")
	subject := gitConfigless(repo, "log", "-1", "--format=%s")

	if err := fillCommitLog(path, filepath.Base(repo), hash, subject); err != nil {
		fmt.Fprintf(os.Stderr, "clif: commit log: %v\n", err)
	}
}

// fillCommitLog rewrites the last line for repo still holding the hash
// placeholder, replacing the file whole so a reader never sees half of it.
func fillCommitLog(path, repoName, hash, subject string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")

	for i := len(lines) - 1; i >= 0; i-- {
		f := strings.Split(lines[i], " | ")
		if len(f) != 6 || f[2] != placeholder || f[4] != logField(repoName) {
			continue
		}

		f[2] = hash
		f[5] = logField(subject)

		// The subject names the commit's own ticket; the branch's, written
		// at pre-commit, is the fallback.
		if ticketRe.MatchString(subject) {
			f[1] = ticketRe.FindString(subject)
		}

		lines[i] = strings.Join(f, " | ")

		tmp := path + ".tmp"
		if err := os.WriteFile(tmp, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
			return err
		}

		return os.Rename(tmp, path)
	}

	return nil
}
