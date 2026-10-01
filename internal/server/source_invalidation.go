package server

import (
	"encoding/json"
	"path/filepath"
	"strings"

	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
)

// Mapping configuration includes named CFConfig files, which need not have a
// conventional basename. Cache selections until server/config invalidation.
func (s *Server) isMappingJSON(path string) bool {
	if !strings.EqualFold(filepath.Ext(path), ".json") {
		return false
	}

	base := filepath.Base(path)
	if strings.EqualFold(base, "server.json") || strings.EqualFold(base, ".cfconfig.json") {
		return true
	}

	s.resolverMu.Lock()
	defer s.resolverMu.Unlock()

	if s.mappingJSONFiles == nil {
		s.mappingJSONFiles = map[string]bool{}
	}

	if s.mappingJSONDirs == nil {
		s.mappingJSONDirs = map[string]bool{}
	}

	seen := s.mappingJSONDirs
	for _, root := range append(s.searchRoots(), filepath.Dir(path)) {
		for dir, n := filepath.Clean(root), 0; n < 32; n++ {
			if seen[dir] {
				break
			}

			seen[dir] = true

			data, err := s.FS.ReadFile(filepath.Join(dir, "server.json"))
			if err == nil {
				var config struct {
					CFConfig struct {
						File string `json:"file"`
					} `json:"cfconfig"`
				}
				if json.Unmarshal(data, &config) == nil && config.CFConfig.File != "" && !strings.ContainsAny(config.CFConfig.File, "#$") {
					file := filepath.FromSlash(config.CFConfig.File)
					if !filepath.IsAbs(file) {
						file = filepath.Join(dir, file)
					}

					s.mappingJSONFiles[filepath.Clean(file)] = true
				}
			}

			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}

			dir = parent
		}
	}

	return s.mappingJSONFiles[filepath.Clean(path)]
}

func (s *Server) invalidateSourceFile(file string) {
	if !cfpath.IsCFMLFile(file) && !s.isMappingJSON(file) {
		return
	}

	r := s.getResolver()
	if isApplicationFile(file) || s.isMappingJSON(file) || r.DiscoveryAffected(file) {
		s.invalidateResolver()
	} else {
		r.InvalidatePaths()
		s.invalidateRoutes()
	}

	if cfpath.IsCFMLFile(file) || s.isMappingJSON(file) {
		cfpath.InvalidateAppMappingsCache()
	}
}
