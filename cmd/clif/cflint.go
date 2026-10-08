package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cfmleditor/clif/internal/cflint"
	"github.com/cfmleditor/clif/internal/config"
	"github.com/cfmleditor/clif/internal/daemon"
	cfpath "github.com/cfmleditor/clif/internal/path"
	"github.com/cfmleditor/clif/internal/vfs"
)

const cflintUsage = `usage: clif cflint [options] <path> [...]
       clif cflint --staged | --changed <ref> [options] [<path> ...]

Run CFLint over files or directories. The CFLint binary is downloaded on first
use; .cflintrc files and @CFLintIgnore comments apply exactly as CFLint applies
them, and a .cflintrc CFLint cannot parse is an error (exit 2) rather than
being silently replaced by CFLint's default rules.

Only the paths given are linted. Naming the config's own directory lints the
folders its workspacePaths list beneath it; folders outside it are there for
resolution, which CFLint does not do, and are never linted.

What to lint:
  <path> ...          files and directories
  --staged            the staged .cfm/.cfc files, as staged: the index
                      content, not the working tree, so an unstaged edit
                      neither hides nor adds a finding (a pre-commit hook)
  --changed <ref>     the .cfm/.cfc files changed between <ref> and HEAD
                      (git diff <ref>...HEAD), from the working tree (CI)
  With --staged or --changed, paths narrow the files to those under them; with
  none, the git repository holding the working directory is used.

Output:
  --format <f>        report (default): a known-issues report, one issue per
                      line, paths relative to the .clif.json above the first
                      path, which the editors read as diagnostics
                      text: CFLint's -text report, ending "Total issues:N"
                      json: CFLint's -json report (top-level "issues"), with
                      totals per rule and per severity over the whole run
                      sarif: SARIF 2.1.0, rule ids being CFLint's codes, paths
                      relative to the repository or the config's directory
  --out <file>        write to this file instead of stdout. A report is written
                      with paths relative to the file's directory, which must
                      hold every path linted
  --write             write the report files the config's knownIssues entries
                      with "generate": "cflint" name (report format only). A
                      report whose directory holds more than was linted is
                      refused, since rewriting it would drop the entries for
                      everything else; lint its whole directory, or use --out
  -q, --quiet         no progress on stderr

Severity:
  --min-severity <l>  report, and fail on, CFLint levels at or above <l>:
                      FATAL, CRITICAL, ERROR, WARNING, CAUTION, INFO, COSMETIC.
                      Overrides linting.minSeverity in the config
  --strict            report and fail on every level, whatever the config says

Exit status:
  0  no findings, or a known-issues report was written (--write, or --out in
     the report format: a regeneration, expected to hold findings)
  1  findings
  2  the run could not be trusted: a path that does not exist, a .cflintrc
     CFLint cannot parse, CFLint unavailable or failing, not a git repository
     for --staged/--changed, or a usage error
`

// clif cflint's exit statuses. A gate reads 1 as "the code has findings" and
// anything else non-zero as "the check itself did not run", so a broken run
// never passes for a clean one and never reads as the code's fault either.
const (
	exitCFLintFindings = 1
	exitCFLintError    = 2
)

// cflintFormats are the --format values.
var cflintFormats = []string{"report", "text", "json", "sarif"}

// cflintFailf reports an error that makes the run untrustworthy and exits with
// exitCFLintError.
func cflintFailf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(exitCFLintError)
}

// cflintExitCode is the status a run that completed exits with: findings fail
// it, unless it was writing a known-issues report, which is a regeneration and
// expected to hold findings.
func cflintExitCode(findings int, regenerating bool) int {
	if findings > 0 && !regenerating {
		return exitCFLintFindings
	}

	return 0
}

// cflintFlags is what cmdCFLint's arguments ask for.
type cflintFlags struct {
	out         string
	format      string
	minSeverity string
	changed     string
	roots       []string
	write       bool
	strict      bool
	quiet       bool
	staged      bool
}

// regenerating reports whether the run writes a known-issues report, the one
// output that holds findings by design and so does not fail on them.
func (fl *cflintFlags) regenerating() bool {
	return fl.format == "report" && (fl.write || fl.out != "")
}

func parseCFLintFlags(args []string) cflintFlags {
	fl := cflintFlags{format: "report"}

	// The arguments are consumed from the front, an option taking its value
	// with it. A counted loop advancing its own index from a closure read as
	// a plain range to the linter, whose fix silently stopped skipping values.
	rest := args

	for len(rest) > 0 {
		arg := rest[0]
		rest = rest[1:]
		name, inline, hasInline := strings.Cut(arg, "=")

		// value reads an option's value, given as --opt value or --opt=value.
		value := func() string {
			if hasInline {
				return inline
			}

			if len(rest) == 0 {
				cflintFailf("%s needs a value\n\n%s", name, cflintUsage)
			}

			v := rest[0]
			rest = rest[1:]

			return v
		}

		switch name {
		case "--write":
			fl.write = true
		case "--strict":
			fl.strict = true
		case "-q", "--quiet":
			fl.quiet = true
		case "--staged":
			fl.staged = true
		case "--out":
			fl.out = value()
		case "--format":
			fl.format = value()
		case "--min-severity":
			fl.minSeverity = value()
		case "--changed":
			fl.changed = value()
		default:
			fl.roots = append(fl.roots, positionalOr(arg, cflintUsage, exitCFLintError))
		}
	}

	checkCFLintFlags(&fl)

	return fl
}

// checkCFLintFlags refuses combinations that cannot mean anything.
func checkCFLintFlags(fl *cflintFlags) {
	switch {
	case !slices.Contains(cflintFormats, fl.format):
		cflintFailf("--format %q: want one of %s\n\n%s", fl.format, strings.Join(cflintFormats, ", "), cflintUsage)
	case fl.write && fl.out != "":
		cflintFailf("--write and --out cannot be used together\n\n%s", cflintUsage)
	case fl.write && fl.format != "report":
		cflintFailf("--write writes known-issues reports; use --out for --format %s\n\n%s", fl.format, cflintUsage)
	case fl.staged && fl.changed != "":
		cflintFailf("--staged and --changed cannot be used together\n\n%s", cflintUsage)
	case fl.strict && fl.minSeverity != "":
		cflintFailf("--strict and --min-severity cannot be used together\n\n%s", cflintUsage)
	case len(fl.roots) == 0 && !fl.staged && fl.changed == "":
		fmt.Fprint(os.Stderr, cflintUsage)
		os.Exit(exitCFLintError)
	}

	if fl.minSeverity != "" {
		if _, ok := cflint.MinSeverityRank(fl.minSeverity); !ok {
			cflintFailf("--min-severity %q is not a CFLint level\n\n%s", fl.minSeverity, cflintUsage)
		}
	}
}

// cmdCFLint runs CFLint and writes the result: a known-issues report (to
// stdout relative to the project's .clif.json, to one file with --out, or to
// the files the editor's clif.exportCFLint writes with --write), or CFLint's
// own text or JSON, or SARIF. It is that command for anything that cannot send
// the server one: a git hook, CI, a shell.
func cmdCFLint(args []string) {
	fl := parseCFLintFlags(args)

	logf := func(format string, args ...any) {
		if !fl.quiet {
			fmt.Fprintf(os.Stderr, format, args...)
		}
	}

	roots := cflintRoots(&fl)

	dir := roots[0]
	if info, err := os.Stat(dir); err == nil && !info.IsDir() {
		dir = filepath.Dir(dir)
	}

	configDir := dir
	scanRoots := roots
	minSeverity := ""

	var knownIssues []config.KnownIssues

	if cfg, _ := daemon.FindConfig(dir); cfg != nil {
		configDir = filepath.Dir(cfg.Path)
		minSeverity = cfg.LintMinSeverity()
		knownIssues = cfg.KnownIssues()
		scanRoots = cflint.ScanRoots(roots, configDir, cfg.WorkspaceFolders())

		logf("Using config: %s\n", cfg.Path)
	} else if fl.write {
		cflintFailf("--write needs a .clif.json to say where the report goes; use --out\n")
	}

	switch {
	case fl.strict:
		minSeverity = ""
	case fl.minSeverity != "":
		minSeverity = fl.minSeverity
	}

	var targets []string
	if fl.format == "report" {
		targets = cflintTargets(&fl, knownIssues, configDir, roots, scanRoots)
	}

	sel := selectCFLintFiles(&fl, dir, roots, scanRoots)

	if err := cflint.CheckConfigs(sel.files); err != nil {
		cflintFailf("%v\n", err)
	}

	runner, err := cflint.NewRunner(context.Background(), minSeverity)
	if err != nil {
		cflintFailf("cflint unavailable: %v\n", err)
	}

	logf("Running CFLint over %d files...\n", len(sel.files))

	run, err := lintSelection(runner, &sel)
	if err != nil {
		cflintFailf("%v\n", err)
	}

	if fl.format == "report" {
		writeCFLintReports(&fl, run, targets, logf)
	} else {
		root := configDir
		if sel.repo != "" {
			root = sel.repo
		}

		writeCFLintFormat(&fl, run, root, logf)
	}

	if code := cflintExitCode(len(run.Issues), fl.regenerating()); code != 0 {
		os.Exit(code)
	}
}

// cflintRoots makes the paths given absolute and checks each exists. With
// --staged or --changed and no paths, the working directory stands in.
func cflintRoots(fl *cflintFlags) []string {
	given := fl.roots
	if len(given) == 0 {
		given = []string{"."}
	}

	roots := make([]string, 0, len(given))

	for _, r := range given {
		abs, err := filepath.Abs(r)
		if err != nil {
			cflintFailf("%s: %v\n", r, err)
		}

		// A path that does not exist used to lint nothing and exit 0, which a
		// hook or CI job reads as clean.
		if _, err := os.Stat(abs); err != nil {
			cflintFailf("%s does not exist\n", r)
		}

		roots = append(roots, abs)
	}

	return roots
}

// cflintSelection is what a run lints: files from disk, or, for --staged, each
// file's staged content under its path.
type cflintSelection struct {
	files   []string
	sources []cflint.Source
	// repo is the git repository's root for --staged and --changed, which
	// SARIF paths are written relative to.
	repo string
}

func selectCFLintFiles(fl *cflintFlags, dir string, roots, scanRoots []string) cflintSelection {
	if !fl.staged && fl.changed == "" {
		return cflintSelection{files: collectCFMLFiles(vfs.OS{}, scanRoots)}
	}

	repo, err := gitTopLevel(dir)
	if err != nil {
		cflintFailf("%s: %v\n", dir, err)
	}

	var paths []string
	if fl.staged {
		paths, err = gitStagedFiles(repo)
	} else {
		paths, err = gitChangedFiles(repo, fl.changed)
	}

	if err != nil {
		cflintFailf("%v\n", err)
	}

	sel := cflintSelection{repo: repo}

	// git names files under the repository's real path, so the paths given are
	// compared by theirs: on macOS /tmp is /private/tmp, and a path through a
	// symlinked directory held none of git's files. With no paths given, the
	// whole repository is linted, as git diff --cached reports it from any
	// directory in it.
	within := []string{repo}

	if len(fl.roots) > 0 {
		within = within[:0]

		for _, r := range roots {
			if resolved, err := filepath.EvalSymlinks(r); err == nil {
				r = resolved
			}

			within = append(within, r)
		}
	}

	for _, p := range paths {
		if !cfpath.IsCFMLFile(p) || !slices.ContainsFunc(within, func(r string) bool { return cflint.Within(p, r) }) {
			continue
		}

		sel.files = append(sel.files, p)

		if fl.staged {
			content, err := gitStagedContent(repo, p)
			if err != nil {
				cflintFailf("%v\n", err)
			}

			sel.sources = append(sel.sources, cflint.Source{Path: p, Content: content})
		}
	}

	return sel
}

// lintSelection runs CFLint over a selection: the staged content when there is
// any, the files on disk otherwise.
func lintSelection(runner *cflint.Runner, sel *cflintSelection) (*cflint.Run, error) {
	if sel.sources != nil {
		return runner.LintContent(context.Background(), sel.sources)
	}

	return runner.Lint(context.Background(), sel.files)
}

// writeCFLintReports writes the known-issues report: to stdout, or to the
// files --out or --write name.
func writeCFLintReports(fl *cflintFlags, run *cflint.Run, targets []string, logf func(string, ...any)) {
	reports, left := cflint.Reports(run.Diagnostics(), targets, version)

	for _, r := range reports {
		if !fl.write && fl.out == "" {
			fmt.Print(r.Content)

			continue
		}

		// Owner-only, as the server writes the same report for .exportCFLint.
		if err := os.WriteFile(r.Path, []byte(r.Content), 0o600); err != nil {
			cflintFailf("could not write %s: %v\n", r.Path, err)
		}

		logf("Wrote %s (%d entries)\n", r.Path, r.Entries)
	}

	if left > 0 {
		logf("%d issues under no report's directory left out\n", left)
	}
}

// writeCFLintFormat writes CFLint's text or JSON report, or SARIF, to stdout
// or the --out file.
func writeCFLintFormat(fl *cflintFlags, run *cflint.Run, root string, logf func(string, ...any)) {
	var b strings.Builder

	var err error

	switch fl.format {
	case "text":
		err = cflint.WriteText(&b, run)
	case "json":
		err = cflint.WriteJSON(&b, run)
	case "sarif":
		err = cflint.WriteSARIF(&b, run, root)
	}

	if err != nil {
		cflintFailf("writing the %s report: %v\n", fl.format, err)
	}

	if fl.out == "" {
		fmt.Print(b.String())

		return
	}

	if err := os.WriteFile(fl.out, []byte(b.String()), 0o600); err != nil {
		cflintFailf("could not write %s: %v\n", fl.out, err)
	}

	logf("Wrote %s (%d issues)\n", fl.out, len(run.Issues))
}

// cflintTargets is where a run's report goes, settled before CFLint runs so a
// refusal costs nothing. Printing writes one report relative to the config's
// directory, as the default file there would be.
func cflintTargets(fl *cflintFlags, knownIssues []config.KnownIssues, configDir string, asked, scanRoots []string) []string {
	switch {
	case fl.out != "":
		out, err := filepath.Abs(fl.out)
		if err != nil {
			cflintFailf("--out %s: %v\n", fl.out, err)
		}

		for _, r := range scanRoots {
			if !cflint.Within(r, filepath.Dir(out)) {
				cflintFailf("--out %s: its directory does not hold %s, so those paths could not be written relative to it\n", fl.out, r)
			}
		}

		return []string{out}
	case fl.write:
		targets, err := cflint.WriteTargets(config.GenerateTargets(knownIssues, config.GenerateCFLint, configDir), asked)
		if err != nil {
			cflintFailf("--write: %v. Lint that whole directory, or write this run with --out\n", err)
		}

		return targets
	default:
		return []string{filepath.Join(configDir, "stdout")}
	}
}
