package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"github.com/cfmleditor/cfmleditor-lsp/internal/route"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

const routesUsage = `usage: cfmleditor-lsp routes [options] <dir> [...]

Report the framework routes found in a workspace and what they resolve to,
so the "routes" block in .cfmleditor.json can be checked against the code
rather than guessed at.

  --format <f>   text (default), md, json
  --out <file>   write here instead of stdout
  --unresolved   list only the routes that resolved to nothing
  --json         shorthand for --format json
  --limit <n>    routes listed per group, 0 for all (default 5)
  --quiet        no progress on stderr

The md report is the one to keep: it groups what did not resolve by shape
and by leading segments, says which prefixes resolve elsewhere, and cites
every site, so it can be committed beside the config or handed to whoever
knows the framework.

A route resolving to nothing is not always a defect: these attributes and
functions carry panel ids and runtime expressions too. What matters is
whether a *shape* of route is missing, which is what the grouping shows.
`

type routeFinding struct {
	Route   string   `json:"route"`
	Source  string   `json:"source"`
	Name    string   `json:"name"`
	File    string   `json:"file"`
	Line    uint32   `json:"line"`
	Targets []string `json:"targets,omitempty"`
}

func cmdRoutes(args []string) {
	var (
		onlyUnresolved bool
		format         = "text"
		outPath        string
		quiet          bool
		limit          = 5
		paths          []string
	)

	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "--unresolved":
			onlyUnresolved = true
		case "--json":
			format = "json"
		case "--format":
			if i+1 >= len(args) {
				fatalf("--format needs a value\n\n%s", routesUsage)
			}

			i++
			format = args[i]
		case "--out":
			if i+1 >= len(args) {
				fatalf("--out needs a value\n\n%s", routesUsage)
			}

			i++
			outPath = args[i]
		case "--quiet":
			quiet = true
		case "--limit":
			if i+1 >= len(args) {
				fatalf("--limit needs a value\n\n%s", routesUsage)
			}

			i++

			if _, err := fmt.Sscanf(args[i], "%d", &limit); err != nil {
				fatalf("bad --limit %q\n", args[i])
			}
		case "-h", "--help":
			fmt.Fprint(os.Stderr, routesUsage)

			return
		default:
			if strings.HasPrefix(a, "-") {
				fatalf("unknown option %q\n\n%s", a, routesUsage)
			}

			paths = append(paths, a)
		}
	}

	if len(paths) == 0 {
		fmt.Fprint(os.Stderr, routesUsage)
		os.Exit(1)
	}

	resolved, unresolved, scanned := scanRoutes(paths, quiet)

	out := os.Stdout

	if outPath != "" {
		file, err := os.Create(outPath)
		if err != nil {
			fatalf("creating %s: %v\n", outPath, err)
		}

		defer func() { _ = file.Close() }()

		out = file
	}

	w := bufio.NewWriter(out)

	var err error

	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		err = enc.Encode(map[string]any{"scanned": scanned, "resolved": resolved, "unresolved": unresolved})
	case "md", "markdown":
		err = writeRouteMarkdown(w, resolved, unresolved, scanned, limit)
	case "text":
		err = writeRouteText(w, resolved, unresolved, scanned, onlyUnresolved, limit)
	default:
		fatalf("unknown format %q (want text, md or json)\n", format)
	}

	if err != nil {
		fatalf("%v\n", err)
	}

	if err := w.Flush(); err != nil {
		fatalf("writing output: %v\n", err)
	}

	if outPath != "" && !quiet {
		fmt.Fprintf(os.Stderr, "Wrote %s (%d unresolved of %d routes)\n", outPath, len(unresolved), scanned)
	}
}

// scanRoutes walks the workspace once, resolving every route it finds under the
// config that governs each file.
func scanRoutes(paths []string, quiet bool) (resolved, unresolved []routeFinding, scanned int) {
	fsys := vfs.OS{}

	root, err := filepath.Abs(paths[0])
	if err != nil {
		fatalf("%v\n", err)
	}

	if info, err := os.Stat(root); err == nil && !info.IsDir() {
		root = filepath.Dir(root)
	}

	configs, files := routeScanSetup(fsys, root, paths, quiet)

	var (
		mu sync.Mutex
		wg sync.WaitGroup
	)

	sem := make(chan struct{}, 8)

	for _, f := range files {
		wg.Add(1)

		sem <- struct{}{}

		go func(file string) {
			defer wg.Done()
			defer func() { <-sem }()

			cfg := configs.For(file)
			if cfg.Routes == nil || !cfg.Routes.Config.Enabled() {
				return
			}

			data, err := fsys.ReadFile(file)
			if err != nil || cfpath.IsBinary(data) {
				return
			}

			rel, _ := filepath.Rel(root, file)
			rel = filepath.ToSlash(rel)

			for _, ref := range route.Scan(string(data), &cfg.Routes.Config) {
				if !route.Plausible(ref.Value) {
					continue
				}

				finding := routeFinding{
					Route: ref.Value, Source: string(ref.Source), Name: ref.Name,
					File: rel, Line: ref.Line + 1,
				}

				for _, t := range cfg.Routes.Resolve(ref.Value) {
					if t.Kind == route.KindController {
						finding.Targets = append(finding.Targets, t.Component+"."+t.Method+"()")
					} else {
						tr, _ := filepath.Rel(root, t.Path)
						finding.Targets = append(finding.Targets, filepath.ToSlash(tr))
					}
				}

				mu.Lock()

				scanned++

				if len(finding.Targets) > 0 {
					resolved = append(resolved, finding)
				} else {
					unresolved = append(unresolved, finding)
				}

				mu.Unlock()
			}
		}(f)
	}

	wg.Wait()

	sort.Slice(unresolved, func(i, j int) bool { return unresolved[i].Route < unresolved[j].Route })
	sort.Slice(resolved, func(i, j int) bool { return resolved[i].Route < resolved[j].Route })

	return resolved, unresolved, scanned
}

func routeScanSetup(fsys vfs.FS, root string, paths []string, quiet bool) (*configSet, []string) {
	f := graphFlags{paths: paths, quiet: quiet}

	scanRoots, fallback, shared, _ := routeWorkspace(fsys, root, &f)

	configs := newConfigSet(fsys, shared, fallback)
	configs.preload(scanRoots)

	return configs, collectCFMLFiles(fsys, scanRoots)
}

// printRouteReport groups the findings by the leading segments of the route,
// which is what makes a missing *shape* visible: one unresolved route is usually
// noise, forty sharing a prefix is a rule the config does not have.
func writeRouteText(w io.Writer, resolved, unresolved []routeFinding, scanned int, onlyUnresolved bool, limit int) error {
	var b strings.Builder

	fmt.Fprintf(&b, "%d routes found, %d resolved, %d not\n", scanned, len(resolved), len(unresolved))

	if scanned > 0 {
		fmt.Fprintf(&b, "  %.1f%% resolved\n", float64(len(resolved))*100/float64(scanned))
	}

	bySource := map[string][2]int{}

	for _, f := range resolved {
		c := bySource[f.Source]
		bySource[f.Source] = [2]int{c[0] + 1, c[1]}
	}

	for _, f := range unresolved {
		c := bySource[f.Source]
		bySource[f.Source] = [2]int{c[0], c[1] + 1}
	}

	kinds := make([]string, 0, len(bySource))
	for k := range bySource {
		kinds = append(kinds, k)
	}

	sort.Strings(kinds)

	b.WriteString("\nBy source:\n")

	for _, k := range kinds {
		c := bySource[k]
		fmt.Fprintf(&b, "  %-10s %5d resolved, %5d not\n", k, c[0], c[1])
	}

	if !onlyUnresolved && len(resolved) > 0 {
		fmt.Fprintf(&b, "\nResolved, for example:\n")

		for i, f := range resolved {
			if i >= limit {
				break
			}

			fmt.Fprintf(&b, "  %s\n    -> %s\n", f.Route, strings.Join(f.Targets, "\n    -> "))
		}
	}

	if len(unresolved) == 0 {
		_, err := io.WriteString(w, b.String())

		return err
	}

	groups := map[string][]routeFinding{}

	for _, f := range unresolved {
		segs := strings.Split(f.Route, ".")
		key := segs[0]

		if len(segs) > 1 {
			key += "." + segs[1]
		}

		groups[key] = append(groups[key], f)
	}

	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}

	sort.Slice(keys, func(i, j int) bool {
		if len(groups[keys[i]]) != len(groups[keys[j]]) {
			return len(groups[keys[i]]) > len(groups[keys[j]])
		}

		return keys[i] < keys[j]
	})

	fmt.Fprintf(&b, "\nUnresolved, grouped by leading segments:\n")

	for _, k := range keys {
		g := groups[k]
		fmt.Fprintf(&b, "\n  %-34s %d\n", k+".*", len(g))

		seen := map[string]bool{}
		shown := 0

		for _, f := range g {
			if seen[f.Route] || shown >= limit {
				continue
			}

			seen[f.Route] = true
			shown++

			fmt.Fprintf(&b, "      %s\n        %s:%d  (%s %s)\n", f.Route, f.File, f.Line, f.Source, f.Name)
		}
	}

	_, err := io.WriteString(w, b.String())

	return err
}
