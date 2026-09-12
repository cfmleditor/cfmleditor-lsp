package server

import (
	"encoding/json"
	"path/filepath"

	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"
	"go.lsp.dev/protocol"
)

// configureSession applies the configuration this session runs with: the
// governing .cfmleditor.json, merged with the editor's initializationOptions,
// applied exactly once.
//
// The governing file is the nearest one to the workspace roots the editor
// reported, and the daemon's own (s.ConfigPath) only when that walk finds
// nothing. The two are usually the same file and often are not both present:
// the daemon walks up from the process's working directory, which for an IDE
// that sets none is wherever it was launched from, while a project's own
// config sits under the folder the editor opened. Preferring the session's
// walk is what keeps a project config from being masked by whichever one the
// daemon happened to pass on its way up.
//
// initializationOptions are merged whatever happens, including when no file
// can be read at all: for a client with no config file to write — the IntelliJ
// plugin sends every formatter setting this way — they are the entire
// configuration, and dropping them silently is the one failure this path
// cannot afford.
//
// Applying once matters because applyConfig appends resolvers and keeps the
// first map it is given. A daemon session arrives with those fields already
// filled from Settings, so they are cleared first and the merged result — the
// same file, plus the editor's gaps — replaces them exactly.
func (s *Server) configureSession(editorCfg *config.JSON) {
	baseDir := ""
	if roots := s.editorRoots(); len(roots) > 0 {
		baseDir = roots[0]
	}

	path, fileCfg := s.governingConfig()

	if fileCfg == nil && editorCfg == nil {
		return
	}

	// Relative paths are meaningless once the two sides are merged, since they
	// no longer share a base directory — resolve each against its own before
	// combining. ResolvePaths passes absolute values through untouched.
	if editorCfg != nil {
		editorCfg.Mappings = config.ResolvePaths(editorCfg.Mappings, baseDir)
		editorCfg.BeanPaths = config.ResolvePaths(editorCfg.BeanPaths, baseDir)
	}

	dir := baseDir

	if fileCfg != nil {
		dir = filepath.Dir(path)
		fileCfg.Mappings = config.ResolvePaths(fileCfg.Mappings, dir)
		fileCfg.BeanPaths = config.ResolvePaths(fileCfg.BeanPaths, dir)

		s.log.Info("loaded config from workspace", cflog.String("path", path))
	} else {
		s.log.Info("no .cfmleditor.json found; using editor initializationOptions")
	}

	merged := config.Merge(editorCfg, fileCfg)

	// config.Resolve leaves an absent formatting block at its zero value on
	// purpose (see config.DefaultResolvedFormatting), which is not what the
	// daemon resolved for the same file: daemon.Config's accessors apply each
	// field's default whether or not the block exists. Applying the zero value
	// would hand the session WhitespaceOnly=false — the formatter's safety
	// guard, off — whenever neither side mentions formatting.
	prevFormatting := s.Formatting

	s.Mappings = nil
	s.ExpressionMappings = nil
	s.ServicePropertyResolvers = nil
	s.ComponentResolvers = nil
	s.PropertyResolvers = nil
	s.BeanPaths = nil

	s.applyConfig(config.Resolve(merged, dir))

	if merged.Formatting == nil {
		s.Formatting = prevFormatting
	}
}

// governingConfig finds the .cfmleditor.json this session should run with, and
// returns its path alongside it. Nil when there is none to be had.
func (s *Server) governingConfig() (string, *config.JSON) {
	for _, root := range s.editorRoots() {
		if p, cfg := s.findConfigUpwards(root); cfg != nil {
			return p, cfg
		}
	}

	if s.ConfigPath != "" {
		if cfg := s.readConfigFile(s.ConfigPath); cfg != nil {
			return s.ConfigPath, cfg
		}

		s.log.Warn("the daemon's config file is no longer readable", cflog.String("path", s.ConfigPath))
	}

	return "", nil
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
