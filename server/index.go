package server

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"go.lsp.dev/uri"
	"go.uber.org/zap"
)

type FunctionDef struct {
	Name string
	URI  uri.URI
	Line uint32
}

type Index struct {
	mu    sync.RWMutex
	funcs map[string][]FunctionDef // lowercase name -> definitions
}

func NewIndex() *Index {
	return &Index{funcs: make(map[string][]FunctionDef)}
}

func (idx *Index) Lookup(name string) []FunctionDef {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.funcs[strings.ToLower(name)]
}

func (idx *Index) AllFunctions() []FunctionDef {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	var all []FunctionDef
	for _, defs := range idx.funcs {
		all = append(all, defs...)
	}
	return all
}

func (idx *Index) indexFile(fileURI uri.URI, content string) {
	defs := parseFunctionDefs(fileURI, content)

	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.removeFileEntries(fileURI)

	for _, d := range defs {
		key := strings.ToLower(d.Name)
		idx.funcs[key] = append(idx.funcs[key], d)
	}
}

func (idx *Index) removeFile(fileURI uri.URI) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.removeFileEntries(fileURI)
}

func (idx *Index) removeFilesUnder(prefix string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	for key, entries := range idx.funcs {
		filtered := entries[:0]
		for _, e := range entries {
			if !strings.HasPrefix(string(e.URI), prefix) {
				filtered = append(filtered, e)
			}
		}
		if len(filtered) == 0 {
			delete(idx.funcs, key)
		} else {
			idx.funcs[key] = filtered
		}
	}
}

func (idx *Index) removeFileEntries(fileURI uri.URI) {
	for key, entries := range idx.funcs {
		filtered := entries[:0]
		for _, e := range entries {
			if e.URI != fileURI {
				filtered = append(filtered, e)
			}
		}
		if len(filtered) == 0 {
			delete(idx.funcs, key)
		} else {
			idx.funcs[key] = filtered
		}
	}
}

// Tag-based: <cffunction name="myFunc"
var tagFuncRe = regexp.MustCompile(`(?i)<cffunction\s[^>]*name\s*=\s*["']([^"']+)["']`)

// Script-based: access? returntype? function name(
var scriptFuncRe = regexp.MustCompile(`(?im)(?:(?:public|private|remote|package)\s+)?(?:\w+\s+)?function\s+(\w+)\s*\(`)

func parseFunctionDefs(fileURI uri.URI, content string) []FunctionDef {
	lines := strings.Split(content, "\n")
	var defs []FunctionDef
	seen := make(map[string]bool)

	for i, line := range lines {
		for _, m := range tagFuncRe.FindAllStringSubmatch(line, -1) {
			name := m[1]
			key := strings.ToLower(name) + ":" + string(rune(i))
			if !seen[key] {
				seen[key] = true
				defs = append(defs, FunctionDef{Name: name, URI: fileURI, Line: uint32(i)})
			}
		}
		for _, m := range scriptFuncRe.FindAllStringSubmatch(line, -1) {
			name := m[1]
			key := strings.ToLower(name) + ":" + string(rune(i))
			if !seen[key] {
				seen[key] = true
				defs = append(defs, FunctionDef{Name: name, URI: fileURI, Line: uint32(i)})
			}
		}
	}
	return defs
}

func (s *Server) indexWorkspace() {
	if len(s.WorkspaceFolders) > 0 {
		if len(s.IndexGlobs) > 0 {
			// Expand globs to find specific files
			for _, g := range s.IndexGlobs {
				for _, f := range expandGlob(g) {
					if strings.ToLower(filepath.Ext(f)) == ".cfc" {
						if data, err := os.ReadFile(f); err == nil {
							s.index.indexFile(uri.File(f), string(data))
						}
					}
				}
			}
		} else {
			// Index all .cfc files in workspace folders
			for _, folder := range s.WorkspaceFolders {
				s.indexRoot(folder)
			}
		}
		return
	}
	for _, root := range s.workspaceRoots {
		s.indexRoot(root)
	}
}

// expandGlob expands a glob pattern, handling ** for recursive matching.
func expandGlob(pattern string) []string {
	if !strings.Contains(pattern, "**") {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil
		}
		return matches
	}
	idx := strings.Index(pattern, "**")
	base := filepath.Clean(pattern[:idx])
	suffix := pattern[idx+2:]
	suffix = strings.TrimPrefix(suffix, string(filepath.Separator))

	var out []string
	filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if suffix == "" {
			out = append(out, path)
			return nil
		}
		if matched, _ := filepath.Match(suffix, filepath.Base(path)); matched {
			out = append(out, path)
		}
		return nil
	})
	return out
}

func (s *Server) indexPaths(paths []string) {
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if info.IsDir() {
			s.indexRoot(p)
		} else if strings.ToLower(filepath.Ext(p)) == ".cfc" {
			data, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			s.index.indexFile(uri.File(p), string(data))
		}
	}
}

func (s *Server) indexRoot(root string) {
	s.logger.Info("indexing workspace", zap.String("root", root))
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.ToLower(filepath.Ext(path)) == ".cfc" {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			fileURI := uri.File(path)
			s.index.indexFile(fileURI, string(data))
		}
		return nil
	})
}
