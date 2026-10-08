package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cfmleditor/clif/internal/cflint"
	"github.com/cfmleditor/clif/internal/config"
	"github.com/cfmleditor/clif/internal/daemon"
	"github.com/cfmleditor/clif/internal/vfs"
)

const cflintUsage = `usage: clif cflint [--write | --out <file>] <dir> [...]

Run CFLint over the given directories and print the result as a known-issues
report, one issue per line, paths relative to the .clif.json found above
the first directory. The CFLint binary is downloaded on first use.
linting.minSeverity in the config sets the least severe level reported.

Only the directories given are linted. Naming the config's own directory lints
the folders its workspacePaths list beneath it; folders outside it are there
for resolution, which CFLint does not do, and are never linted.

  --out <file>   write the report to this file instead, paths relative to its
                 directory, which must hold every directory linted
  --write        write the report files the config's knownIssues entries with
                 "generate": "cflint" name. A report whose directory holds more
                 than was linted is refused, since rewriting it would drop the
                 entries for everything else; lint its whole directory, or use
                 --out

Exit status:
  0  no findings, or a report was written (--write, --out)
  1  findings were printed
  2  the run could not be trusted: a path that does not exist, a .cflintrc
     CFLint cannot parse (it would silently lint with its default rules
     instead), CFLint unavailable or failing, or a usage error
`

// clif cflint's exit statuses. A gate reads 1 as "the code has findings" and
// anything else non-zero as "the check itself did not run", so a broken run
// never passes for a clean one and never reads as the code's fault either.
const (
	exitCFLintFindings = 1
	exitCFLintError    = 2
)

// cflintFailf reports an error that makes the run untrustworthy and exits with
// exitCFLintError.
func cflintFailf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(exitCFLintError)
}

// cflintExitCode is the status a run that completed exits with: findings fail
// a printed run, which is what a gate runs, and never a written report, which
// is a regeneration and expected to hold findings.
func cflintExitCode(findings int, writing bool) int {
	if findings > 0 && !writing {
		return exitCFLintFindings
	}

	return 0
}

// cflintFlags is what cmdCFLint's arguments ask for.
type cflintFlags struct {
	out   string
	roots []string
	write bool
}

func parseCFLintFlags(args []string) cflintFlags {
	var fl cflintFlags

	for i := 0; i < len(args); i++ {
		a := args[i]

		if v, ok := strings.CutPrefix(a, "--out="); ok {
			fl.out = v

			continue
		}

		switch a {
		case "--write":
			fl.write = true
		case "--out":
			if i+1 >= len(args) {
				cflintFailf("--out needs a file\n\n%s", cflintUsage)
			}

			i++
			fl.out = args[i]
		default:
			fl.roots = append(fl.roots, positionalOr(a, cflintUsage, exitCFLintError))
		}
	}

	if fl.write && fl.out != "" {
		cflintFailf("--write and --out cannot be used together\n\n%s", cflintUsage)
	}

	if len(fl.roots) == 0 {
		fmt.Fprint(os.Stderr, cflintUsage)
		os.Exit(exitCFLintError)
	}

	return fl
}

// cmdCFLint runs CFLint over a project and writes the result as a known-issues
// report: to stdout, relative to the project's .clif.json; with --out to
// one file; or with --write to the files the editor's clif.exportCFLint
// writes (config.GenerateTargets). It is that command for anything that cannot
// send the server one: a Zed task, CI, a shell.
func cmdCFLint(args []string) {
	fl := parseCFLintFlags(args)

	roots := make([]string, 0, len(fl.roots))
	for _, r := range fl.roots {
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

		fmt.Fprintf(os.Stderr, "Using config: %s\n", cfg.Path)
	} else if fl.write {
		cflintFailf("--write needs a .clif.json to say where the report goes; use --out\n")
	}

	targets := cflintTargets(&fl, knownIssues, configDir, roots, scanRoots)

	files := collectCFMLFiles(vfs.OS{}, scanRoots)

	if err := cflint.CheckConfigs(files); err != nil {
		cflintFailf("%v\n", err)
	}

	runner, err := cflint.NewRunner(context.Background(), minSeverity)
	if err != nil {
		cflintFailf("cflint unavailable: %v\n", err)
	}

	fmt.Fprintf(os.Stderr, "Running CFLint over %d files...\n", len(files))

	found, err := runner.ScanFiles(context.Background(), files)
	if err != nil {
		cflintFailf("cflint failed: %v\n", err)
	}

	reports, left := cflint.Reports(found, targets, version)

	for _, r := range reports {
		if !fl.write && fl.out == "" {
			fmt.Print(r.Content)

			continue
		}

		// Owner-only, as the server writes the same report for .exportCFLint.
		if err := os.WriteFile(r.Path, []byte(r.Content), 0o600); err != nil {
			cflintFailf("could not write %s: %v\n", r.Path, err)
		}

		fmt.Fprintf(os.Stderr, "Wrote %s (%d entries)\n", r.Path, r.Entries)
	}

	if left > 0 {
		fmt.Fprintf(os.Stderr, "%d issues under no report's directory left out\n", left)
	}

	findings := 0
	for _, diags := range found {
		findings += len(diags)
	}

	if code := cflintExitCode(findings, fl.write || fl.out != ""); code != 0 {
		os.Exit(code)
	}
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
