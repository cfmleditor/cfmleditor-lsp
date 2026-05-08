package server

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"go.lsp.dev/uri"
	"go.uber.org/zap"
)

func (s *Server) indexWorkspace() {
	if len(s.WorkspaceFolders) > 0 {
		if len(s.IndexGlobs) > 0 {
			for _, g := range s.IndexGlobs {
				for _, f := range expandGlob(g) {
					if strings.ToLower(filepath.Ext(f)) == ".cfc" {
						if data, err := os.ReadFile(f); err == nil {
							s.index.IndexFile(uri.File(f), string(data))
						}
					}
				}
			}
		} else {
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
	err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
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

	if ( err != nil ) {
		 log.Fatalf("failed to write file: %s", err)
	}
	return out
}

func (s *Server) indexRoot(root string) {
	s.logger.Info("indexing workspace", zap.String("root", root))
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
	        return err // WRONG: returns a non-nil interface containing a nil pointer
	    }
		if info.IsDir() {
			return nil
		}
		if strings.ToLower(filepath.Ext(path)) == ".cfc" {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			s.index.IndexFile(uri.File(path), string(data))
		}
		return nil
	})
	if ( err != nil ) {
		 log.Fatalf("failed to write file: %s", err)
	}
}
