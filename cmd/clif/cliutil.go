package main

import (
	// The standard library on purpose — see the note in
	// internal/server/standalone_config.go: json/v2 would silently drop a
	// mis-cased key from a hand-written .clif.json.
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/cfmleditor/clif/internal/config"
	"github.com/cfmleditor/clif/internal/parser"
)

// loadResolversFromConfig finds .clif.json (or .clif.json) in the given paths and returns resolvers.
func loadResolversFromConfig(paths []string) []parser.Resolver {
	for _, p := range paths {
		dir := p
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			dir = filepath.Dir(p)
		}

		// The directory, then its parent; in each, .clif.json before the legacy
		// .clif.json.
		var data []byte

		for _, cfgPath := range append(config.CandidatesIn(dir), config.CandidatesIn(filepath.Dir(dir))...) {
			if b, err := os.ReadFile(cfgPath); err == nil {
				data = b

				break
			}
		}

		if data == nil {
			continue
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
