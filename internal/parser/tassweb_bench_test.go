package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.lsp.dev/uri"
)

func TestBenchTasswebParse(t *testing.T) {
	root := os.ExpandEnv("$HOME/tassdev/tassweb")
	if _, err := os.Stat(root); err != nil {
		t.Skip("tassweb not found")
	}

	resolvers := []Resolver{
		{Match: `getService("$1")`, Resolve: "packages.$1.service", Prefix: "getService"},
	}

	var files []string

	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && filepath.Ext(path) == ".cfc" {
			files = append(files, path)
		}

		return nil
	})

	fmt.Printf("Files: %d\n", len(files))

	start := time.Now()

	var totalFuncs, totalRefs, totalLinks int

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}

		pr := ParseWithOptions(uri.URI("file://"+f), string(data), ParseOptions{
			Resolvers:    resolvers,
			ExtractLinks: true,
		})

		totalFuncs += len(pr.Funcs)
		totalRefs += len(pr.ComponentRefs)
		totalLinks += len(pr.Links)
	}

	dur := time.Since(start)
	fmt.Printf("Duration: %v\n", dur)
	fmt.Printf("Per file: %v\n", dur/time.Duration(len(files)))
	fmt.Printf("Functions: %d\n", totalFuncs)
	fmt.Printf("Refs: %d\n", totalRefs)
	fmt.Printf("Links: %d\n", totalLinks)

	// Shallow scan (signatures only — used by workspace indexer)
	start = time.Now()

	var shallowFuncs int

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}

		pr := ParseWithOptions(uri.URI("file://"+f), string(data), ParseOptions{
			Shallow: true,
		})

		shallowFuncs += len(pr.Funcs)
	}

	shallowDur := time.Since(start)

	fmt.Printf("\nShallow scan:\n")
	fmt.Printf("Duration: %v\n", shallowDur)
	fmt.Printf("Per file: %v\n", shallowDur/time.Duration(len(files)))
	fmt.Printf("Functions: %d\n", shallowFuncs)
}
