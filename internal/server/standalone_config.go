package server

import (
	"encoding/json"
	"path/filepath"

	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
	"go.uber.org/zap"
)

// loadConfigFromRoots searches workspace roots for .cfmleditor.json and loads
// settings in standalone mode.
func (s *Server) loadConfigFromRoots() {
	for _, root := range s.workspaceRoots {
		p := filepath.Join(root, ".cfmleditor.json")
		data, err := s.FS.ReadFile(p)
		if err != nil {
			continue
		}
		var cfg config.JSON
		if json.Unmarshal(data, &cfg) != nil {
			continue
		}
		s.logger.Info("loaded config from workspace", zap.String("path", p))
		s.applyConfig(config.Resolve(&cfg, filepath.Dir(p)))
		return
	}
}

func (s *Server) applyConfig(r *config.Resolved) {
	if len(r.Mappings) > 0 && len(s.Mappings) == 0 {
		s.Mappings = r.Mappings
	}
	s.ComponentResolvers = append(s.ComponentResolvers, r.ComponentResolvers...)
	s.PropertyResolvers = append(s.PropertyResolvers, r.PropertyResolvers...)
	if len(r.BeanPaths) > 0 && len(s.BeanPaths) == 0 {
		s.BeanPaths = r.BeanPaths
	}
	s.Linting = r.Linting
	s.Formatting = r.Formatting
}
