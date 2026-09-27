package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/uri"
)

// parsedArgs holds cfparse's parsed command-line arguments.
type parsedArgs struct {
	profilePath string
	targets     []string
}

// parseArgs parses cfparse's arguments, extracting an optional "-profile <path>" pair ahead of
// the file/dir targets. It touches neither the filesystem nor the process — callers decide how
// to report the returned error (main exits 1; tests just assert on it).
func parseArgs(args []string) (parsedArgs, error) {
	var pa parsedArgs

	if len(args) >= 2 && args[0] == "-profile" {
		pa.profilePath = args[1]
		args = args[2:]
	}

	if len(args) == 0 {
		return pa, errors.New("usage: cfparse [-profile cpu.prof] <file-or-dir> [...]")
	}

	pa.targets = args

	return pa, nil
}

// collectFiles expands targets (files or directories) into a flat file list. Directory targets
// are walked and filtered to CFML extensions; file targets are included as-is regardless of
// extension, matching the user's explicit choice to name that file.
func collectFiles(targets []string) ([]string, error) {
	var files []string

	for _, target := range targets {
		info, err := os.Stat(target)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", target, err)
		}

		if !info.IsDir() {
			files = append(files, target)

			continue
		}

		// An unreadable entry is skipped rather than ending the walk, so the
		// callback returns no error and Walk has none to report.
		_ = filepath.Walk(target, func(path string, _ os.FileInfo, err error) error {
			if err == nil && hasCFMLExt(path) {
				files = append(files, path)
			}

			return nil
		})
	}

	return files, nil
}

// benchStats aggregates parse results across all benchmarked files.
type benchStats struct {
	Files int
	Funcs int
	Refs  int
	Dur   time.Duration
}

// runBenchmark parses each file with parser.Parse, writing one summary line per successfully
// parsed file to out and a skip notice for unreadable files to errOut. It returns the aggregated
// stats so callers (including tests) can assert on numbers directly instead of scraping text.
func runBenchmark(files []string, out, errOut io.Writer) benchStats {
	var stats benchStats

	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			_, _ = fmt.Fprintf(errOut, "  skip %s: %v\n", f, err)

			continue
		}

		fileURI := uri.URI("file://" + f)
		start := time.Now()
		pr := parser.Parse(fileURI, string(content))
		dur := time.Since(start)

		stats.Dur += dur
		stats.Funcs += len(pr.Funcs)
		stats.Refs += len(pr.ComponentRefs)
		stats.Files++

		_, _ = fmt.Fprintf(out, "  %s  funcs=%d refs=%d scopes=%d  %v\n", f, len(pr.Funcs), len(pr.ComponentRefs), len(pr.Scopes), dur)
	}

	return stats
}

// printSummary writes the final aggregate line in the format cfparse has always printed.
func printSummary(out io.Writer, stats benchStats) {
	avg := time.Duration(0)
	if stats.Files > 0 {
		avg = stats.Dur / time.Duration(stats.Files)
	}

	_, _ = fmt.Fprintf(out, "\n  total: %d files, %d funcs, %d refs in %v (avg %v/file)\n",
		stats.Files, stats.Funcs, stats.Refs, stats.Dur, avg)
}

// hasCFMLExt reports whether path names a file cfparse reads.
func hasCFMLExt(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".cfc", ".cfm", ".cfml", ".cfs":
		return true
	}

	return false
}
