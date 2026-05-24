package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/cfmleditor/cfmleditor-lsp/internal/cfparser"
	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
)

// loadResolversFromConfig finds .cfmleditor.json in the given paths and returns resolvers.
func loadResolversFromConfig(paths []string) []cfparser.Resolver {
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
		var resolvers []cfparser.Resolver
		for _, r := range cfg.ComponentResolvers {
			if r.Match != "" && r.Resolve != "" {
				resolvers = append(resolvers, cfparser.Resolver{Match: r.Match, Resolve: r.Resolve, Prefix: r.Prefix})
			}
		}
		return resolvers
	}
	return nil
}
