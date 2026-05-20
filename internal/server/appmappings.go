package server

import (
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
)

// effectiveMappings returns config mappings merged with Application.cfc mappings.
// Config mappings take precedence over Application.cfc mappings.
func (s *Server) effectiveMappings(baseDir string) map[string]string {
	appDir := findApplicationRoot(baseDir)
	if appDir == "" {
		return s.Mappings
	}

	appMappings := cfpath.LoadAppMappings(appDir)
	if len(appMappings) == 0 {
		return s.Mappings
	}

	if len(s.Mappings) == 0 {
		return appMappings
	}

	// Merge: config takes precedence
	merged := make(map[string]string, len(appMappings)+len(s.Mappings))
	for k, v := range appMappings {
		merged[k] = v
	}
	for k, v := range s.Mappings {
		merged[k] = v
	}
	return merged
}
