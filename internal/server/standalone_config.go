package server

import (
	"encoding/json"
	"path/filepath"

	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"
	"go.lsp.dev/protocol"
)

// loadWorkspaceConfig applies the nearest .cfmleditor.json, using editorCfg
// (the client's initializationOptions) to fill in anything that file does not
// state. The file wins on every key it sets, so adding editor settings never
// changes how an existing project behaves.
//
// editorCfg may be nil, and so may the file — with neither, nothing is applied.
func (s *Server) loadWorkspaceConfig(editorCfg *config.JSON) {
	baseDir := ""
	if len(s.workspaceRoots) > 0 {
		baseDir = s.workspaceRoots[0]
	}

	// Relative paths are meaningless once the two sides are merged, since they
	// no longer share a base directory — resolve each against its own before
	// combining. ResolvePaths passes absolute values through untouched.
	if editorCfg != nil {
		editorCfg.Mappings = config.ResolvePaths(editorCfg.Mappings, baseDir)
		editorCfg.BeanPaths = config.ResolvePaths(editorCfg.BeanPaths, baseDir)
	}

	for _, root := range s.workspaceRoots {
		p, fileCfg := s.findConfigUpwards(root)
		if fileCfg == nil {
			continue
		}

		dir := filepath.Dir(p)
		fileCfg.Mappings = config.ResolvePaths(fileCfg.Mappings, dir)
		fileCfg.BeanPaths = config.ResolvePaths(fileCfg.BeanPaths, dir)

		s.log.Info("loaded config from workspace", cflog.String("path", p))
		s.applyConfig(config.Resolve(config.Merge(editorCfg, fileCfg), dir))

		return
	}

	if editorCfg == nil {
		return
	}

	s.log.Info("no .cfmleditor.json found; using editor initializationOptions")
	s.applyConfig(config.Resolve(editorCfg, baseDir))
}

// editorConfig decodes the client's initializationOptions, which carry the
// same shape as .cfmleditor.json. Editors that expose no way to set them (or
// set nothing) send an empty value, which yields nil.
//
// Note `debug` is ignored here: the logger is built from the on-disk config
// before the client connects, so by this point it is too late to change.
func (s *Server) editorConfig(raw protocol.LSPAny) *config.JSON {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}

	var cfg config.JSON
	if err := json.Unmarshal(raw, &cfg); err != nil {
		s.log.Warn("ignoring unparseable initializationOptions", cflog.Err(err))

		return nil
	}

	s.log.Info("loaded config from editor initializationOptions")

	return &cfg
}

// findConfigUpwards walks from dir towards the filesystem root, returning the
// first readable, parseable .cfmleditor.json it finds along with its path. A
// file that exists but does not parse is skipped rather than aborting the
// walk, so one malformed config cannot mask a valid one further up.
//
// Walking upwards matches what daemon.FindConfig does for the daemon-mode
// startup path. Checking only the root directory itself meant a config that
// daemon mode picks up happily was invisible in standalone mode, which
// silently dropped mappings, resolvers, and linting depending only on which
// mode the editor happened to start.
// readConfigFile parses one .cfmleditor.json, or returns nil if it is missing
// or unreadable as config.
func (s *Server) readConfigFile(path string) *config.JSON {
	data, err := s.FS.ReadFile(path)
	if err != nil {
		return nil
	}

	var cfg config.JSON
	if err := json.Unmarshal(data, &cfg); err != nil {
		s.log.Warn("ignoring unparseable config", cflog.String("path", path), cflog.Err(err))

		return nil
	}

	return &cfg
}

// overlayEditorConfig merges the editor's initializationOptions with the config
// file the daemon already applied to this session, and applies the result.
//
// The daemon reads .cfmleditor.json once and hands every session the same
// settings, but it never sees initializationOptions: those arrive per session,
// at initialize, and for an IDE that has no config file of its own to write —
// IntelliLucee sends every formatter setting from its settings UI this way —
// they are the entire configuration. Skipping them whenever the daemon happened
// to find a config file would mean an editor's settings applied or not
// depending on which directory its process was started from.
//
// This re-resolves from the daemon's own file rather than searching again, so
// the two can't disagree, and clears what the daemon applied first: applyConfig
// appends resolvers and keeps the first map it is given, both of which assume
// it runs once. The merged result is a superset of what was cleared — same
// file, plus the editor's gaps — so one application replaces it exactly.
func (s *Server) overlayEditorConfig(editorCfg *config.JSON) {
	fileCfg := s.readConfigFile(s.ConfigPath)
	if fileCfg == nil {
		return
	}

	baseDir := ""
	if len(s.workspaceRoots) > 0 {
		baseDir = s.workspaceRoots[0]
	}

	editorCfg.Mappings = config.ResolvePaths(editorCfg.Mappings, baseDir)
	editorCfg.BeanPaths = config.ResolvePaths(editorCfg.BeanPaths, baseDir)

	dir := filepath.Dir(s.ConfigPath)
	fileCfg.Mappings = config.ResolvePaths(fileCfg.Mappings, dir)
	fileCfg.BeanPaths = config.ResolvePaths(fileCfg.BeanPaths, dir)

	s.Mappings = nil
	s.ExpressionMappings = nil
	s.ServicePropertyResolvers = nil
	s.ComponentResolvers = nil
	s.PropertyResolvers = nil
	s.BeanPaths = nil

	s.log.Info("merging editor initializationOptions with the daemon's config", cflog.String("path", s.ConfigPath))
	s.applyConfig(config.Resolve(config.Merge(editorCfg, fileCfg), dir))
}

func (s *Server) findConfigUpwards(dir string) (string, *config.JSON) {
	d, err := filepath.Abs(dir)
	if err != nil {
		return "", nil
	}

	for {
		p := filepath.Join(d, ".cfmleditor.json")
		if cfg := s.readConfigFile(p); cfg != nil {
			return p, cfg
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

	if len(r.ExpressionMappings) > 0 && len(s.ExpressionMappings) == 0 {
		s.ExpressionMappings = r.ExpressionMappings
	}

	if len(r.ServicePropertyResolvers) > 0 && len(s.ServicePropertyResolvers) == 0 {
		s.ServicePropertyResolvers = r.ServicePropertyResolvers
	}

	s.ComponentResolvers = append(s.ComponentResolvers, r.ComponentResolvers...)
	s.PropertyResolvers = append(s.PropertyResolvers, r.PropertyResolvers...)

	if len(r.BeanPaths) > 0 && len(s.BeanPaths) == 0 {
		s.BeanPaths = r.BeanPaths
	}

	s.Linting = r.Linting
	s.References = r.References
	s.TagSnippets = r.TagSnippets
	s.FunctionSnippets = r.FunctionSnippets
	s.GlobalFunctionResolution = r.GlobalFunctionResolution
	s.Formatting = r.Formatting
}
