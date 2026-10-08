package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
	"github.com/cfmleditor/cfmleditor-lsp/internal/daemon"
	"github.com/cfmleditor/cfmleditor-lsp/internal/frameworkapi"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"github.com/cfmleditor/cfmleditor-lsp/internal/unresolved"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

const unresolvedUsage = `usage: cfmleditor-lsp unresolved [options] <dir> [...]

Report every component or method call that does not resolve: a receiver with
no known component, a method the component does not declare, a component that
does not exist. Reads the .cfmleditor.json above the first directory.

  --json               print the results as JSON
  --known-issues       print them as a known-issues report, paths relative to
                       the config's directory
  --relative-to <dir>  with --known-issues, make paths relative to this instead
  --include-workspace  with --known-issues, keep entries outside that
                       directory, as ../ paths
  --write              write the report files the config's knownIssues entries
                       with "generate": "unresolved" name
  --verbose            also list each call that resolved, on stderr
  --global-defs        resolve bare calls against functions declared anywhere
                       in the workspace
  --no-infer-args      do not type an untyped argument from what its callers
                       pass it

To see why one call resolved as it did: cfmleditor-lsp explain <file> <line>
`

// unresolvedFlags is what cmdUnresolved's flags ask for.
type unresolvedFlags struct {
	relativeTo       string
	json             bool
	knownIssues      bool
	write            bool
	includeWorkspace bool
	verbose          bool
	globalDefs       bool
	noInferArgs      bool
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
		case "--no-infer-args":
			fl.noInferArgs = true
		default:
			paths = append(paths, positional(a, unresolvedUsage))
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

	rep, cfg, searchDir := scanUnresolved(args, &fl)
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

	if hint := presetHint(cfg, searchDir); hint != "" {
		fmt.Fprintf(os.Stderr, "\n%s\n", hint)
	}

	fmt.Fprintf(os.Stderr, "\nBenchmark:\n")
	fmt.Fprintf(os.Stderr, "  Index:  %v (%d files)\n", rep.IndexTime, rep.Indexed)
	fmt.Fprintf(os.Stderr, "  Scan:   %v (%d files)\n", rep.ScanTime, rep.Scanned)
	fmt.Fprintf(os.Stderr, "  Total:  %v\n", rep.IndexTime+rep.ScanTime)
}

// scanUnresolved runs the scan the unresolved command reports: it indexes the
// workspace the config above the first path names, and checks the calls in the
// paths given. The MCP server's find_unresolved_calls runs it too.
func scanUnresolved(args []string, fl *unresolvedFlags) (unresolved.Report, *daemon.Config, string) {
	fsys := vfs.OS{}

	// Find .cfmleditor.json config based on the first file/dir argument
	searchDir, _ := filepath.Abs(args[0])
	if info, err := os.Stat(searchDir); err == nil && !info.IsDir() {
		searchDir = filepath.Dir(searchDir)
	}

	cfg, _ := daemon.FindConfig(searchDir)
	opt := unresolvedOptions(cfg, args, fl)

	// Collect files from workspace folders or args
	scanRoots := args

	configuredRoots := cfg != nil && len(cfg.WorkspaceFolders()) > 0
	if configuredRoots {
		scanRoots = opt.WorkspaceFolders
	}

	files := collectCFMLFiles(fsys, scanRoots)

	scanFiles := scanTargets(fsys, args, configuredRoots)

	fmt.Fprintf(os.Stderr, "Indexing %d files, then scanning for unresolved calls...\n", len(files))

	return unresolved.Scan(fsys, files, scanFiles, opt), cfg, searchDir
}

// unresolvedOptions builds the scan's options from the config, or from the
// paths given when there is none.
func unresolvedOptions(cfg *daemon.Config, args []string, fl *unresolvedFlags) *unresolved.Options {
	opt := &unresolved.Options{GlobalDefs: fl.globalDefs, InferArgs: !fl.noInferArgs, WorkspaceFolders: cliWorkspaceFolders(vfs.OS{}, cfg, args)}
	if fl.verbose {
		opt.Verbose = os.Stderr
	}

	if cfg != nil {
		opt.Mappings = cfg.Mappings()
		opt.StartupFiles = cfg.StartupFiles()
		opt.ExpressionMappings = cfg.ExpressionMappings()
		opt.ServicePropertyResolvers = cfg.ServicePropertyResolvers()
		opt.BeanPaths = cfg.BeanPaths()
		opt.PropertyResolvers = configPropertyResolvers(cfg)
		opt.ImplicitExtends = config.ImplicitExtends(cfg.Frameworks())
		opt.HelperScope = config.HelperScope(cfg.Frameworks())
		opt.Stubs = frameworkapi.For(cfg.Frameworks())
		opt.InterpolateAll = !cfg.ResolvedFeatures().OutputContextInterpolation

		if unknown := config.UnknownFrameworks(cfg.Frameworks()); len(unknown) > 0 {
			fmt.Fprintf(os.Stderr, "frameworks: no preset named %s (known: %s)\n",
				strings.Join(unknown, ", "), strings.Join(config.KnownFrameworks(), ", "))
		}

		for _, r := range cfg.ComponentResolvers() {
			opt.Resolvers = append(opt.Resolvers, r.Parser())
		}

		fmt.Fprintf(os.Stderr, "Using config: %s\n", cfg.Path)

		return opt
	}

	if fl.write {
		fmt.Fprintf(os.Stderr, "--write needs a .cfmleditor.json to say where the report goes\n")
		os.Exit(1)
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

// presetHint suggests the framework presets the project's box.json implies
// and its config does not name — how a project finds out a preset exists.
// It reads box.json beside the config, or in the scanned directory without one.
func presetHint(cfg *daemon.Config, searchDir string) string {
	dir, have := searchDir, []string(nil)
	if cfg != nil {
		dir, have = filepath.Dir(cfg.Path), cfg.Frameworks()
	}

	data, err := os.ReadFile(filepath.Join(dir, "box.json"))
	if err != nil {
		return ""
	}

	suggest := config.SuggestFrameworks(data, have)
	if len(suggest) == 0 {
		return ""
	}

	all := slices.Concat(have, suggest)

	where := "a .cfmleditor.json"
	if cfg != nil {
		where = cfg.Path
	}

	return fmt.Sprintf("box.json names %s, which have framework presets: add `\"frameworks\": [\"%s\"]` to %s\n"+
		"so the values those frameworks hand your code are typed rather than reported.",
		strings.Join(suggest, ", "), strings.Join(all, `", "`), where)
}

// scanTargets is the files a report covers, as absolute paths; nil covers
// everything indexed. Files are always named. A directory narrows the report
// only when the config's workspace folders set the index wider than the
// arguments: the index must stay whole, since resolving a call reads every
// other component, but the report should hold what was asked for. Without
// workspace folders the arguments are the whole index and nothing is narrowed.
func scanTargets(fsys vfs.FS, args []string, indexWiderThanArgs bool) []string {
	abs := make([]string, 0, len(args))

	for _, a := range args {
		p, err := filepath.Abs(a)
		if err != nil {
			p = a
		}

		abs = append(abs, p)
	}

	if indexWiderThanArgs {
		return collectCFMLFiles(fsys, abs)
	}

	var files []string

	for _, p := range abs {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			files = append(files, p)
		}
	}

	return files
}
