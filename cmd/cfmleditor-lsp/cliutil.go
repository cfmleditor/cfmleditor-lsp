package main

import (
	// The standard library on purpose — see the note in
	// internal/server/standalone_config.go: json/v2 would silently drop a
	// mis-cased key from a hand-written .cfmleditor.json.
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
				resolvers = append(resolvers, r.Parser())
			}
		}

		if jr := config.JavaStubResolver(cfg.JavaStubsPath); jr.Match != "" {
			resolvers = append(resolvers, jr.Parser())
		}

		return resolvers
	}

	return nil
}
