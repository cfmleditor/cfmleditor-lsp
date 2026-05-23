package server

import (
	"encoding/json"
	"os"
	"path/filepath"

	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"go.uber.org/zap"
)

// standaloneConfig is the subset of .cfmleditor.json used in standalone mode.
type standaloneConfig struct {
	Mappings           map[string]string `json:"mappings"`
	ComponentResolvers []struct {
		Match   string `json:"match"`
		Resolve string `json:"resolve"`
		Prefix  string `json:"prefix"`
	} `json:"componentResolvers"`
	PropertyResolvers []struct {
		Match     string `json:"match"`
		Resolve   string `json:"resolve"`
		Attribute string `json:"attribute"`
	} `json:"propertyResolvers"`
	BeanPaths map[string]string `json:"beanPaths"`
}

// loadConfigFromRoots searches workspace roots for .cfmleditor.json and loads
// resolvers/mappings in standalone mode.
func (s *Server) loadConfigFromRoots() {
	for _, root := range s.workspaceRoots {
		p := filepath.Join(root, ".cfmleditor.json")
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var cfg standaloneConfig
		if json.Unmarshal(data, &cfg) != nil {
			continue
		}
		s.logger.Info("loaded config from workspace", zap.String("path", p))
		dir := filepath.Dir(p)
		if len(cfg.Mappings) > 0 && len(s.Mappings) == 0 {
			s.Mappings = cfpath.ResolveMappings(cfg.Mappings, dir)
		}
		for _, r := range cfg.ComponentResolvers {
			if r.Match != "" && r.Resolve != "" {
				s.ComponentResolvers = append(s.ComponentResolvers, ComponentResolver{Match: r.Match, Resolve: r.Resolve, Prefix: r.Prefix})
			}
		}
		for _, r := range cfg.PropertyResolvers {
			if r.Match != "" && r.Resolve != "" && r.Attribute != "" {
				s.PropertyResolvers = append(s.PropertyResolvers, PropertyResolver{Match: r.Match, Resolve: r.Resolve, Attribute: r.Attribute})
			}
		}
		if len(cfg.BeanPaths) > 0 && len(s.BeanPaths) == 0 {
			s.BeanPaths = cfpath.ResolveMappings(cfg.BeanPaths, dir)
		}
		return // use first config found
	}
}
