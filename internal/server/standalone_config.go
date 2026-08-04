package server

import (
	"encoding/json"
	"path/filepath"

	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"
)

// loadConfigFromRoots searches workspace roots for .cfmleditor.json and loads
// settings in standalone mode.
//
// Each root is searched upwards to the filesystem root, matching what
// daemon.FindConfig does for the daemon-mode startup path. Checking only the
// root directory itself meant a config that daemon mode picks up happily was
// invisible in standalone mode, which silently dropped mappings, resolvers,
// and linting depending only on which mode the editor happened to start.
func (s *Server) loadConfigFromRoots() {
	for _, root := range s.workspaceRoots {
		p, cfg := s.findConfigUpwards(root)
		if cfg == nil {
			continue
		}

		s.log.Info("loaded config from workspace", cflog.String("path", p))
		s.applyConfig(config.Resolve(cfg, filepath.Dir(p)))

		return
	}
}

// findConfigUpwards walks from dir towards the filesystem root, returning the
// first readable, parseable .cfmleditor.json it finds along with its path. A
// file that exists but does not parse is skipped rather than aborting the
// walk, so one malformed config cannot mask a valid one further up.
func (s *Server) findConfigUpwards(dir string) (string, *config.JSON) {
	d, err := filepath.Abs(dir)
	if err != nil {
		return "", nil
	}

	for {
		p := filepath.Join(d, ".cfmleditor.json")

		if data, err := s.FS.ReadFile(p); err == nil {
			var cfg config.JSON
			if json.Unmarshal(data, &cfg) == nil {
				return p, &cfg
			}

			s.log.Warn("ignoring unparseable config", cflog.String("path", p))
		}

		parent := filepath.Dir(d)
		if parent == d {
			return "", nil
		}

		d = parent
	}
}

func (s *Server) applyConfig(r *config.Resolved) {
	s.invalidateResolver()

	if len(r.Mappings) > 0 && len(s.Mappings) == 0 {
		s.Mappings = r.Mappings
	}

	s.ComponentResolvers = append(s.ComponentResolvers, r.ComponentResolvers...)
	s.PropertyResolvers = append(s.PropertyResolvers, r.PropertyResolvers...)

	if len(r.BeanPaths) > 0 && len(s.BeanPaths) == 0 {
		s.BeanPaths = r.BeanPaths
	}

	s.Linting = r.Linting
	s.TagSnippets = r.TagSnippets
	s.FunctionSnippets = r.FunctionSnippets
	s.GlobalFunctionResolution = r.GlobalFunctionResolution
	s.Formatting = r.Formatting
}
