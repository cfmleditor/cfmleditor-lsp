package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
)

// loadResolversFromConfig finds .cfmleditor.json in the given paths and returns resolvers.
func loadResolversFromConfig(paths []string) []parser.Resolver {
	for _, p := range paths {
		dir := p
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			dir = filepath.Dir(p)
		}

		cfgPath := filepath.Join(dir, ".cfmleditor.json")

		data, err := os.ReadFile(cfgPath)
		if err != nil {
			// Try parent
			cfgPath = filepath.Join(filepath.Dir(dir), ".cfmleditor.json")

			data, err = os.ReadFile(cfgPath)
			if err != nil {
				continue
			}
		}

		var cfg config.JSON
		if json.Unmarshal(data, &cfg) != nil {
			continue
		}

		var resolvers []parser.Resolver

		for _, r := range cfg.ComponentResolvers {
			if r.Match != "" && r.Resolve != "" {
				resolvers = append(resolvers, parser.Resolver{Match: r.Match, Resolve: r.Resolve, Prefix: r.Prefix})
			}
		}

		return resolvers
	}

	return nil
}

// isBinary returns true if data appears to be binary (contains null bytes in the first 512 bytes).
func isBinary(data []byte) bool {
	n := 512
	if len(data) < n {
		n = len(data)
	}

	for i := 0; i < n; i++ {
		if data[i] == 0 {
			return true
		}
	}

	return false
}
