package server

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
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
					if isCFCFile(f) {
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
		pr := s.parseContent(fileURI, content)
		s.index.IndexFileFromResult(fileURI, pr.Funcs, pr.Refs)
		s.index.SetThisVars(fileURI, pr.ThisVars())
		if pr.Persistent && s.isOrmPath(f) {
			s.index.SetEntity(cfcNameFromURI(fileURI), fileURI)
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

	// Build bean map: merge config bean paths with all Application.cfc bean paths in workspace
	allBeanPaths := make(map[string]string)
	// Discover Application.cfc bean paths from each workspace folder
	for _, root := range s.WorkspaceFolders {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			if strings.EqualFold(info.Name(), "Application.cfc") || strings.EqualFold(info.Name(), "Application.cfm") {
				appDir := filepath.Dir(path)
				for ns, dir := range cfpath.LoadAppBeanPaths(appDir) {
					if _, exists := allBeanPaths[ns]; !exists {
						allBeanPaths[ns] = dir
					}
				}
			}
			return nil
		})
	}
	// Config bean paths take precedence
	for ns, dir := range s.BeanPaths {
		allBeanPaths[ns] = dir
	}
	if len(allBeanPaths) > 0 {
		beans := buildBeanMap(allBeanPaths)
		s.index.SetBeans(beans)
		s.logger.Info("bean map built", zap.Int("beans", len(beans)))
	}
}

// cfcNameFromURI extracts the CFC filename without extension from a URI.
func cfcNameFromURI(fileURI uri.URI) string {
	path := strings.TrimPrefix(string(fileURI), "file://")
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// isOrmPath returns true if the file path is within the ORM entity scope.
// If cfcLocation is defined in Application.cfc, the file must be under one of those dirs.
// Otherwise, the file must be under the Application.cfc directory.
func (s *Server) isOrmPath(filePath string) bool {
	dir := filepath.Dir(filePath)
	appDir := findApplicationRoot(dir)
	if appDir == "" {
		return false
	}
	ormDirs := cfpath.LoadOrmLocations(appDir)
	if len(ormDirs) > 0 {
		for _, ormDir := range ormDirs {
			if strings.HasPrefix(filePath, ormDir+string(filepath.Separator)) || strings.HasPrefix(filePath, ormDir) {
				return true
			}
		}
		return false
	}
	// No cfcLocation — allow anything under the Application.cfc root
	return strings.HasPrefix(filePath, appDir)
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
		if isCFCFile(path) {
			files = append(files, path)
		}
		return nil
	})
	return files
}

func expandGlob(pattern string) []string {
	return cfpath.ExpandGlob(pattern)
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
		if isCFCFile(path) {
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
			pr := s.parseContent(fileURI, content)
			s.index.IndexFileFromResult(fileURI, pr.Funcs, pr.Refs)
			s.index.SetThisVars(fileURI, pr.ThisVars())
			if pr.Persistent && s.isOrmPath(path) {
				s.index.SetEntity(cfcNameFromURI(fileURI), fileURI)
			}
		}
		return nil
	})
	if err != nil {
		log.Fatalf("failed to walk directory: %s", err)
	}
}
