package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
	"github.com/cfmleditor/cfmleditor-lsp/internal/daemon"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"github.com/cfmleditor/cfmleditor-lsp/internal/unresolved"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

const unresolvedUsage = "usage: cfmleditor-lsp unresolved [--json | --known-issues [--relative-to <dir>] [--include-workspace] | --write] [--global-defs] <dir> [...]\n"

func cmdUnresolved(args []string) {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, unresolvedUsage)
		os.Exit(1)
	}

	jsonOutput := false
	knownIssuesOutput := false
	writeTargets := false
	includeWorkspace := false
	verbose := false
	matchGlobalDefs := false

	var (
		filteredArgs []string
		relativeTo   string
	)

	for i := 0; i < len(args); i++ {
		a := args[i]

		if v, ok := strings.CutPrefix(a, "--relative-to="); ok {
			relativeTo = v

			continue
		}

		if a == "--relative-to" && i+1 < len(args) {
			i++
			relativeTo = args[i]

			continue
		}

		switch a {
		case "--json":
			jsonOutput = true
		case "--known-issues":
			knownIssuesOutput = true
		case "--write":
			writeTargets = true
		case "--include-workspace":
			includeWorkspace = true
		case "--verbose":
			verbose = true
		case "--global-defs":
			matchGlobalDefs = true
		default:
			filteredArgs = append(filteredArgs, a)
		}
	}

	args = filteredArgs

	if len(args) == 0 {
		fmt.Fprint(os.Stderr, unresolvedUsage)
		os.Exit(1)
	}

	fsys := vfs.OS{}

	// Find .cfmleditor.json config based on the first file/dir argument
	searchDir, _ := filepath.Abs(args[0])
	if info, err := os.Stat(searchDir); err == nil && !info.IsDir() {
		searchDir = filepath.Dir(searchDir)
	}

	opt := unresolved.Options{GlobalDefs: matchGlobalDefs}
	if verbose {
		opt.Verbose = os.Stderr
	}

	cfg, _ := daemon.FindConfig(searchDir)
	if cfg != nil {
		opt.WorkspaceFolders = cfg.WorkspaceFolders()
		opt.Mappings = cfg.Mappings()
		opt.ExpressionMappings = cfg.ExpressionMappings()
		opt.ServicePropertyResolvers = cfg.ServicePropertyResolvers()
		opt.InterpolateAll = !cfg.ResolvedFeatures().OutputContextInterpolation

		for _, r := range cfg.ComponentResolvers() {
			opt.Resolvers = append(opt.Resolvers, parser.Resolver{Match: r.Match, Resolve: r.Resolve, Prefix: r.Prefix, NoFollow: r.NoFollow, Anchored: r.Anchored, DynamicIfMissing: r.DynamicIfMissing})
		}

		fmt.Fprintf(os.Stderr, "Using config: %s\n", cfg.Path)
	} else {
		if writeTargets {
			fmt.Fprintf(os.Stderr, "--write needs a .cfmleditor.json to say where the report goes\n")
			os.Exit(1)
		}

		// Fallback: use args as workspace folders
		for _, a := range args {
			if info, err := os.Stat(a); err == nil && info.IsDir() {
				abs, _ := filepath.Abs(a)
				opt.WorkspaceFolders = append(opt.WorkspaceFolders, abs)
			}
		}
	}

	// Collect files from workspace folders or args
	scanRoots := args
	if len(opt.WorkspaceFolders) > 0 {
		scanRoots = opt.WorkspaceFolders
	}

	files := collectCFMLFiles(fsys, scanRoots)

	// Filter scan targets if specific files were passed
	var scanFiles []string

	for _, a := range args {
		if info, err := os.Stat(a); err == nil && !info.IsDir() {
			abs, _ := filepath.Abs(a)
			scanFiles = append(scanFiles, abs)
		}
	}

	fmt.Fprintf(os.Stderr, "Indexing %d files, then scanning for unresolved calls...\n", len(files))

	rep := unresolved.Scan(fsys, files, scanFiles, &opt)
	results := rep.Calls

	switch {
	case writeTargets:
		targets := config.GenerateTargets(cfg.KnownIssues(), config.GenerateUnresolved, filepath.Dir(cfg.Path))

		byTarget, rest := unresolved.SplitByTarget(results, targets)
		for _, t := range targets {
			n, err := writeReport(t, byTarget[t], unresolved.RegenerateHint)
			if err != nil {
				fmt.Fprintf(os.Stderr, "could not write %s: %v\n", t, err)
				os.Exit(1)
			}

			fmt.Fprintf(os.Stderr, "Wrote %s (%d entries)\n", t, n)
		}

		if len(rest) > 0 {
			fmt.Fprintf(os.Stderr, "%d entries under no report's directory left out\n", len(rest))
		}
	case len(results) == 0:
		fmt.Fprintf(os.Stderr, "No unresolved calls found.\n")

		return
	case jsonOutput:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(results)
	case knownIssuesOutput:
		baseDir, _ := filepath.Abs(searchDir)
		if cfg != nil {
			baseDir = filepath.Dir(cfg.Path)
		}

		if relativeTo != "" {
			if abs, err := filepath.Abs(relativeTo); err == nil {
				baseDir = abs
			}
		}

		regenerate := "cfmleditor-lsp unresolved --known-issues " + strings.Join(args, " ")
		if skipped := unresolved.WriteKnownIssues(os.Stdout, results, baseDir, includeWorkspace, regenerate, version); skipped > 0 {
			fmt.Fprintf(os.Stderr, "%d entries outside %s left out; --include-workspace writes them as ../ paths\n", skipped, baseDir)
		}
	default:
		for i := range results {
			r := &results[i]

			fmt.Printf("%s:%d: %s (%s)\n", r.File, r.Line+1, r.CallText(), r.Reason)
		}
	}

	fmt.Fprintf(os.Stderr, "%d unresolved calls found (%d resolved)\n", len(results), rep.Resolved)

	fmt.Fprintf(os.Stderr, "\nBenchmark:\n")
	fmt.Fprintf(os.Stderr, "  Index:  %v (%d files)\n", rep.IndexTime, rep.Indexed)
	fmt.Fprintf(os.Stderr, "  Scan:   %v (%d files)\n", rep.ScanTime, rep.Scanned)
	fmt.Fprintf(os.Stderr, "  Total:  %v\n", rep.IndexTime+rep.ScanTime)
}

// writeReport writes calls to path as a known-issues file relative to its own
// directory, and returns how many entries it holds.
func writeReport(path string, calls []unresolved.Call, regenerate string) (int, error) {
	var b strings.Builder

	skipped := unresolved.WriteKnownIssues(&b, calls, filepath.Dir(path), false, regenerate, version)

	return len(calls) - skipped, os.WriteFile(path, []byte(b.String()), 0o644) //nolint:gosec // a report committed to the project, read by everyone
}

func collectCFMLFiles(fsys vfs.FS, roots []string) []string {
	var files []string

	for _, root := range roots {
		_ = fsys.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				if path != root && skipDir(info.Name()) {
					return filepath.SkipDir
				}

				return nil
			}

			if cfpath.IsCFMLFile(path) {
				files = append(files, path)
			}

			return nil
		})
	}

	return files
}

// skipDir reports whether a directory is one no scan should descend into.
//
// Every dot-directory is skipped, not a hand-kept list of them. The list was
// ".git", ".svn", "node_modules", "target" and "vendor", and on a real workspace
// it let in ".claude/worktrees" — git worktrees living inside the project, each a
// complete second copy of the codebase. A whole-project map built over that counts
// every component twice, reports every function as having a mysterious duplicate,
// and inflates the unreferenced list with thousands of entries from a checkout
// nobody is editing. The same applies to any other tool's dot-directory, which is
// why this is a rule rather than another name on a list.
//
// The root itself is exempt, so pointing a scan at a dot-directory still works.
func skipDir(name string) bool {
	if strings.HasPrefix(name, ".") && name != "." && name != ".." {
		return true
	}

	switch name {
	case "node_modules", "target", "vendor", "bower_components":
		return true
	default:
		return false
	}
}
