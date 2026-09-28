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

// unresolvedFlags is what cmdUnresolved's flags ask for.
type unresolvedFlags struct {
	relativeTo       string
	json             bool
	knownIssues      bool
	write            bool
	includeWorkspace bool
	verbose          bool
	globalDefs       bool
}

// parseUnresolvedFlags splits the flags from the files and directories.
func parseUnresolvedFlags(args []string) (unresolvedFlags, []string) {
	var (
		fl    unresolvedFlags
		paths []string
	)

	for i := 0; i < len(args); i++ {
		a := args[i]

		if v, ok := strings.CutPrefix(a, "--relative-to="); ok {
			fl.relativeTo = v

			continue
		}

		if a == "--relative-to" && i+1 < len(args) {
			i++
			fl.relativeTo = args[i]

			continue
		}

		switch a {
		case "--json":
			fl.json = true
		case "--known-issues":
			fl.knownIssues = true
		case "--write":
			fl.write = true
		case "--include-workspace":
			fl.includeWorkspace = true
		case "--verbose":
			fl.verbose = true
		case "--global-defs":
			fl.globalDefs = true
		default:
			paths = append(paths, a)
		}
	}

	return fl, paths
}

func cmdUnresolved(args []string) {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, unresolvedUsage)
		os.Exit(1)
	}

	fl, args := parseUnresolvedFlags(args)

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

	cfg, _ := daemon.FindConfig(searchDir)
	opt := unresolvedOptions(cfg, args, &fl)

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

	rep := unresolved.Scan(fsys, files, scanFiles, opt)
	if !writeUnresolved(rep.Calls, cfg, args, searchDir, &fl) {
		return
	}

	fmt.Fprintf(os.Stderr, "%d unresolved calls found (%d resolved)\n", len(rep.Calls), rep.Resolved)

	if bases := unresolved.MissingBases(rep.Calls); len(bases) > 0 {
		fmt.Fprintf(os.Stderr, "\nBase components that do not resolve (a mapping or workspace path for each checks its calls):\n")

		for _, b := range bases {
			fmt.Fprintf(os.Stderr, "  %-50s %5d files %7d calls\n", b.Component, b.Files, b.Calls)
		}
	}

	fmt.Fprintf(os.Stderr, "\nBenchmark:\n")
	fmt.Fprintf(os.Stderr, "  Index:  %v (%d files)\n", rep.IndexTime, rep.Indexed)
	fmt.Fprintf(os.Stderr, "  Scan:   %v (%d files)\n", rep.ScanTime, rep.Scanned)
	fmt.Fprintf(os.Stderr, "  Total:  %v\n", rep.IndexTime+rep.ScanTime)
}

// unresolvedOptions builds the scan's options from the config, or from the
// paths given when there is none.
func unresolvedOptions(cfg *daemon.Config, args []string, fl *unresolvedFlags) *unresolved.Options {
	opt := &unresolved.Options{GlobalDefs: fl.globalDefs}
	if fl.verbose {
		opt.Verbose = os.Stderr
	}

	if cfg != nil {
		opt.WorkspaceFolders = cfg.WorkspaceFolders()
		opt.Mappings = cfg.Mappings()
		opt.ExpressionMappings = cfg.ExpressionMappings()
		opt.ServicePropertyResolvers = cfg.ServicePropertyResolvers()
		opt.BeanPaths = cfg.BeanPaths()
		opt.PropertyResolvers = configPropertyResolvers(cfg)
		opt.InterpolateAll = !cfg.ResolvedFeatures().OutputContextInterpolation

		for _, r := range cfg.ComponentResolvers() {
			opt.Resolvers = append(opt.Resolvers, parser.Resolver{Match: r.Match, Resolve: r.Resolve, Prefix: r.Prefix, NoFollow: r.NoFollow, Anchored: r.Anchored, DynamicIfMissing: r.DynamicIfMissing})
		}

		fmt.Fprintf(os.Stderr, "Using config: %s\n", cfg.Path)

		return opt
	}

	if fl.write {
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

	return opt
}

// writeUnresolved writes the results the way the flags ask, and reports
// whether the summary should follow: an empty result says so and stops.
func writeUnresolved(results []unresolved.Call, cfg *daemon.Config, args []string, searchDir string, fl *unresolvedFlags) bool {
	switch {
	case fl.write:
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

		return false
	case fl.json:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")

		if err := enc.Encode(results); err != nil {
			fmt.Fprintf(os.Stderr, "writing JSON: %v\n", err)
			os.Exit(1)
		}
	case fl.knownIssues:
		baseDir, _ := filepath.Abs(searchDir)
		if cfg != nil {
			baseDir = filepath.Dir(cfg.Path)
		}

		if fl.relativeTo != "" {
			if abs, err := filepath.Abs(fl.relativeTo); err == nil {
				baseDir = abs
			}
		}

		regenerate := "cfmleditor-lsp unresolved --known-issues " + strings.Join(args, " ")
		if skipped := unresolved.WriteKnownIssues(os.Stdout, results, baseDir, fl.includeWorkspace, regenerate, version); skipped > 0 {
			fmt.Fprintf(os.Stderr, "%d entries outside %s left out; --include-workspace writes them as ../ paths\n", skipped, baseDir)
		}
	default:
		for i := range results {
			r := &results[i]

			fmt.Printf("%s:%d: %s (%s)\n", r.File, r.Line+1, r.CallText(), r.Reason)
		}
	}

	return true
}

// writeReport writes calls to path as a known-issues file relative to its own
// directory, and returns how many entries it holds.
func writeReport(path string, calls []unresolved.Call, regenerate string) (int, error) {
	var b strings.Builder

	skipped := unresolved.WriteKnownIssues(&b, calls, filepath.Dir(path), false, regenerate, version)

	// Owner-only, as the server writes the same report for .exportUnresolved.
	return len(calls) - skipped, os.WriteFile(path, []byte(b.String()), 0o600)
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
