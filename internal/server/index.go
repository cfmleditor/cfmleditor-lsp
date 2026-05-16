package server

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cfmleditor/cfmleditor-lsp/internal/cfparser"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
	"go.uber.org/zap"
)

func (s *Server) indexWorkspace() {
	// Collect all .cfc files to index.
	var files []string
	if len(s.WorkspaceFolders) > 0 {
		if len(s.IndexGlobs) > 0 {
			for _, g := range s.IndexGlobs {
				for _, f := range expandGlob(g) {
					if strings.ToLower(filepath.Ext(f)) == ".cfc" {
						files = append(files, f)
					}
				}
			}
		} else {
			for _, folder := range s.WorkspaceFolders {
				files = append(files, collectCFCFiles(folder)...)
			}
		}
	} else {
		for _, root := range s.workspaceRoots {
			files = append(files, collectCFCFiles(root)...)
		}
	}

	total := len(files)
	s.logger.Info("indexing workspace", zap.Int("totalFiles", total))
	indexStart := time.Now()

	ctx := context.Background()
	token := "indexing"

	// Send progress begin.
	if s.conn != nil && total > 0 {
		_ = s.conn.Notify(ctx, protocol.MethodProgress, map[string]interface{}{
			"token": token,
			"value": map[string]interface{}{"kind": "begin", "title": "Indexing", "message": fmt.Sprintf("0/%d files", total), "percentage": 0},
		})
	}

	for i, f := range files {
		fileURI := uri.File(f)
		if _, open := s.getDocument(fileURI); open {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		content := string(data)
		if len(s.ComponentResolvers) > 0 {
			pr := cfparser.Parse(fileURI, content, s.cfResolvers())
			s.index.IndexFileFromResult(fileURI, pr.Funcs, pr.Refs)
			s.index.SetThisVars(fileURI, pr.ThisVars())
		} else {
			s.index.IndexFile(fileURI, content)
		}

		if s.conn != nil && total > 0 {
			pct := ((i + 1) * 100) / total
			_ = s.conn.Notify(ctx, protocol.MethodProgress, map[string]interface{}{
				"token": token,
				"value": map[string]interface{}{"kind": "report", "message": fmt.Sprintf("%d/%d files", i+1, total), "percentage": pct},
			})
		}
	}

	// Send progress end.
	if s.conn != nil && total > 0 {
		_ = s.conn.Notify(ctx, protocol.MethodProgress, map[string]interface{}{
			"token": token,
			"value": map[string]interface{}{"kind": "end", "message": fmt.Sprintf("Indexed %d files", total)},
		})
	}
	s.logger.Info("indexing complete", zap.Int("files", total), zap.Duration("dur", time.Since(indexStart)))
}

// collectCFCFiles walks root and returns all .cfc file paths.
func collectCFCFiles(root string) []string {
	var files []string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.ToLower(filepath.Ext(path)) == ".cfc" {
			files = append(files, path)
		}
		return nil
	})
	return files
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

	if err != nil {
		log.Fatalf("failed to walk glob: %s", err)
	}
	return out
}

func (s *Server) indexRoot(root string) {
	s.logger.Info("indexing workspace", zap.String("root", root))
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.ToLower(filepath.Ext(path)) == ".cfc" {
			fileURI := uri.File(path)
			// Skip files already open in the editor — their buffer
			// content was indexed via didOpen and may be newer than disk.
			if _, open := s.getDocument(fileURI); open {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			content := string(data)
			if len(s.ComponentResolvers) > 0 {
				pr := cfparser.Parse(fileURI, content, s.cfResolvers())
				s.index.IndexFileFromResult(fileURI, pr.Funcs, pr.Refs)
				s.index.SetThisVars(fileURI, pr.ThisVars())
			} else {
				s.index.IndexFile(fileURI, content)
			}
		}
		return nil
	})
	if err != nil {
		log.Fatalf("failed to walk directory: %s", err)
	}
}
