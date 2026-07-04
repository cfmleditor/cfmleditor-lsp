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
	"runtime/pprof"
)

func main() {
	pa, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	files, err := collectFiles(pa.targets)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "no CFML files found")
		os.Exit(1)
	}

	if pa.profilePath != "" {
		f, err := os.Create(pa.profilePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error creating profile: %v\n", err)
			os.Exit(1)
		}

		pprof.StartCPUProfile(f) //nolint:errcheck

		defer pprof.StopCPUProfile()
	}

	stats := runBenchmark(files, os.Stdout, os.Stderr)
	printSummary(os.Stdout, stats)
}
