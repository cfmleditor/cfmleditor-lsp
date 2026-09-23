package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cfmleditor/cfmleditor-lsp/internal/cflint"
	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
	"github.com/cfmleditor/cfmleditor-lsp/internal/daemon"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

const cflintUsage = "usage: cfmleditor-lsp cflint [--write] <project>\n"

// cmdCFLint runs CFLint over a whole project and writes the result as a
// known-issues report: to stdout, relative to the project's .cfmleditor.json,
// or with --write to the files the editor's cfmleditor.exportCFLint writes
// (config.GenerateTargets). It is that command for anything that cannot send
// the server one: a Zed task, CI, a shell.
func cmdCFLint(args []string) {
	write := false

	var roots []string

	for _, a := range args {
		switch a {
		case "--write":
			write = true
		default:
			roots = append(roots, a)
		}
	}

	if len(roots) == 0 {
		fmt.Fprint(os.Stderr, cflintUsage)
		os.Exit(1)
	}

	dir, _ := filepath.Abs(roots[0])
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

		if wf := cfg.WorkspaceFolders(); len(wf) > 0 {
			scanRoots = wf
		}

		fmt.Fprintf(os.Stderr, "Using config: %s\n", cfg.Path)
	} else if write {
		fmt.Fprint(os.Stderr, "--write needs a .cfmleditor.json to say where the report goes\n")
		os.Exit(1)
	}

	runner, err := cflint.NewRunner(minSeverity)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cflint unavailable: %v\n", err)
		os.Exit(1)
	}

	files := collectCFMLFiles(vfs.OS{}, scanRoots)
	fmt.Fprintf(os.Stderr, "Running CFLint over %d files...\n", len(files))

	found, err := runner.ScanFiles(context.Background(), files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cflint failed: %v\n", err)
		os.Exit(1)
	}

	// Printing writes one report, relative to the config's directory, as the
	// default file there would be.
	targets := []string{filepath.Join(configDir, "stdout")}
	if write {
		targets = config.GenerateTargets(knownIssues, config.GenerateCFLint, configDir)
	}

	reports, left := cflint.Reports(found, targets, version)

	for _, r := range reports {
		if !write {
			fmt.Print(r.Content)

			continue
		}

		if err := os.WriteFile(r.Path, []byte(r.Content), 0o644); err != nil { //nolint:gosec // a report committed to the project, read by everyone
			fmt.Fprintf(os.Stderr, "could not write %s: %v\n", r.Path, err)
			os.Exit(1)
		}

		fmt.Fprintf(os.Stderr, "Wrote %s (%d entries)\n", r.Path, r.Entries)
	}

	if left > 0 {
		fmt.Fprintf(os.Stderr, "%d issues under no report's directory left out\n", left)
	}
}
