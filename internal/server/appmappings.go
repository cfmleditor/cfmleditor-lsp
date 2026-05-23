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

// resolveComponentPath resolves a component dot-path to an absolute .cfc file path
// using the standard fallback chain: baseDir → Application.cfc root → workspace folders.
func (s *Server) resolveComponentPath(component, baseDir string) string {
	mappings := s.effectiveMappings(baseDir)
	if p := cfpath.ResolvePath(component, baseDir, mappings); p != "" {
		return p
	}
	if appDir := findApplicationRoot(baseDir); appDir != "" {
		if p := cfpath.ResolvePath(component, appDir, mappings); p != "" {
			return p
		}
	}
	for _, root := range s.WorkspaceFolders {
		if p := cfpath.ResolvePath(component, root, mappings); p != "" {
			return p
		}
	}
	return ""
}
