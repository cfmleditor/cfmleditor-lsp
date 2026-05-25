// Command cfparse benchmarks the cfparser against files or directories.
//
// Usage:
//
//	go run ./cmd/cfparse path/to/file.cfc
//	go run ./cmd/cfparse path/to/directory
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/uri"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: cfparse [-profile cpu.prof] <file-or-dir> [...]\n")
		os.Exit(1)
	}

	args := os.Args[1:]
	var profilePath string
	if len(args) >= 2 && args[0] == "-profile" {
		profilePath = args[1]
		args = args[2:]
	}

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: cfparse [-profile cpu.prof] <file-or-dir> [...]\n")
		os.Exit(1)
	}

	var files []string
	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s: %v\n", arg, err)
			os.Exit(1)
		}
		if info.IsDir() {
			filepath.Walk(arg, func(path string, _ os.FileInfo, err error) error { //nolint:errcheck
				if err != nil {
					return nil //nolint:nilerr
				}
				ext := strings.ToLower(filepath.Ext(path))
				if ext == ".cfc" || ext == ".cfm" || ext == ".cfml" || ext == ".cfs" {
					files = append(files, path)
				}
				return nil
			})
		} else {
			files = append(files, arg)
		}
	}

	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "no CFML files found\n")
		os.Exit(1)
	}

	var totalDur time.Duration
	var totalFuncs, totalRefs, totalFiles int

	if profilePath != "" {
		f, err := os.Create(profilePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error creating profile: %v\n", err)
			os.Exit(1)
		}
		pprof.StartCPUProfile(f) //nolint:errcheck
		defer pprof.StopCPUProfile()
	}

	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  skip %s: %v\n", f, err)
			continue
		}

		fileURI := uri.URI("file://" + f)
		start := time.Now()
		pr := parser.Parse(fileURI, string(content))
		dur := time.Since(start)

		totalDur += dur
		totalFuncs += len(pr.Funcs)
		totalRefs += len(pr.Refs)
		totalFiles++

		fmt.Printf("  %s  funcs=%d refs=%d scopes=%d  %v\n", f, len(pr.Funcs), len(pr.Refs), len(pr.Scopes), dur)
	}

	fmt.Printf("\n  total: %d files, %d funcs, %d refs in %v (avg %v/file)\n",
		totalFiles, totalFuncs, totalRefs, totalDur, totalDur/time.Duration(max(totalFiles, 1)))
}
