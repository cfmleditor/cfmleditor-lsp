package server

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cfmleditor/cfmleditor-lsp/internal/cfparser"

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
				files = append(files, s.collectCFCFiles(folder)...)
			}
		}
	} else {
		for _, root := range s.workspaceRoots {
			files = append(files, s.collectCFCFiles(root)...)
		}
	}

	total := len(files)
	s.logger.Info("indexing workspace", zap.Int("totalFiles", total))
	indexStart := time.Now()

	ctx := context.Background()
	token := "indexing"

	// Send progress begin.
	if s.conn != nil && total > 0 {
		s.notify(ctx, protocol.MethodProgress, map[string]interface{}{
			"token": token,
			"value": map[string]interface{}{"kind": "begin", "title": "Indexing", "message": fmt.Sprintf("0/%d files", total), "percentage": 0},
		})
	}

	type parseResult struct {
		fileURI    uri.URI
		pr         *cfparser.ParseResult
		file       string
		persistent bool
	}
	results := make(chan parseResult, 64)

	// Start consumer first (prevents deadlock when channel fills)
	var indexWg sync.WaitGroup
	indexWg.Add(1)
	indexed := 0
	go func() {
		defer indexWg.Done()
		for r := range results {
			s.index.IndexFileFromResult(r.fileURI, r.pr.Funcs, r.pr.Refs)
			s.index.SetThisVars(r.fileURI, r.pr.ThisVars())
			if r.persistent && s.isOrmPath(r.file) {
				s.index.SetEntity(cfcNameFromURI(r.fileURI), r.fileURI)
			}
			indexed++
			if s.conn != nil && total > 0 && indexed%100 == 0 {
				pct := (indexed * 100) / total
				s.notify(ctx, protocol.MethodProgress, map[string]interface{}{
					"token": token,
					"value": map[string]interface{}{"kind": "report", "message": fmt.Sprintf("%d/%d files", indexed, total), "percentage": pct},
				})
			}
		}
	}()

	// Parallel read + parse
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for _, f := range files {
		f := f
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			defer func() {
				if r := recover(); r != nil {
					s.logger.Error("panic during indexing", zap.String("file", f), zap.Any("panic", r))
				}
			}()
			fileURI := uri.File(f)
			if _, open := s.getDocument(fileURI); open {
				return
			}
			data, err := s.FS.ReadFile(f)
			if err != nil {
				return
			}
			pr := s.parseContentForIndex(uri.File(f), string(data))
			results <- parseResult{fileURI: uri.File(f), pr: pr, file: f, persistent: pr.Persistent}
		}()
	}
	wg.Wait()
	close(results)
	indexWg.Wait()

	// Send progress end.
	if s.conn != nil && total > 0 {
		s.notify(ctx, protocol.MethodProgress, map[string]interface{}{
			"token": token,
			"value": map[string]interface{}{"kind": "end", "message": fmt.Sprintf("Indexed %d files", total)},
		})
	}
	s.logger.Info("indexing complete", zap.Int("files", indexed), zap.Int("total", total), zap.Duration("dur", time.Since(indexStart)))
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
	appDir := s.getResolver().FindApplicationRoot(dir)
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
func (s *Server) collectCFCFiles(root string) []string {
	var files []string
	_ = s.FS.Walk(root, func(path string, info os.FileInfo, err error) error {
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
	err := s.FS.Walk(root, func(path string, info os.FileInfo, err error) error {
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
			data, err := s.FS.ReadFile(path)
			if err != nil {
				return err
			}
			content := string(data)
			pr := s.parseContentForIndex(fileURI, content)
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
