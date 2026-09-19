package server

import (
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"go.lsp.dev/uri"
)

func (s *Server) indexWorkspace() {
	// Collect all .cfc files to index.
	var files []string

	var source string

	if len(s.WorkspaceFolders) > 0 {
		if len(s.IndexGlobs) > 0 {
			source = "globs"

			for _, g := range s.IndexGlobs {
				for _, f := range expandGlob(g) {
					if cfpath.IsCFCFile(f) {
						files = append(files, f)
					}
				}
			}
		} else {
			source = "workspaceFolders"

			for _, folder := range s.WorkspaceFolders {
				files = append(files, s.collectCFCFiles(folder)...)
			}
		}
	} else {
		source = "editorRoots"

		for _, root := range s.editorRoots() {
			files = append(files, s.collectCFCFiles(root)...)
		}
	}

	total := len(files)
	s.log.Info("indexing workspace", cflog.String("source", source), cflog.Int("totalFiles", total), cflog.Any("paths", s.WorkspaceFolders))

	indexStart := time.Now()

	type parseResult struct {
		fileURI uri.URI
		pr      *parser.ParseResult
		file    string
	}

	results := make(chan parseResult, 64)

	// Start consumer first (prevents deadlock when channel fills)
	var indexWg sync.WaitGroup

	indexWg.Add(1)

	indexed := 0

	go func() {
		defer indexWg.Done()

		for r := range results {
			s.applyIndexResult(r.fileURI, r.file, r.pr)

			indexed++
		}
	}()

	// Parallel read + parse
	var wg sync.WaitGroup

	sem := make(chan struct{}, 8)

	for _, f := range files {
		wg.Add(1)

		sem <- struct{}{}

		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			defer func() {
				if r := recover(); r != nil {
					s.log.Error("panic during indexing", cflog.String("file", f), cflog.Any("panic", r))
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

			if cfpath.IsBinary(data) {
				return
			}

			pr := s.parseContentForIndex(uri.File(f), string(data))
			results <- parseResult{fileURI: uri.File(f), pr: pr, file: f}
		}()
	}

	wg.Wait()
	close(results)
	indexWg.Wait()

	// Hand the scan's working memory back to the operating system.
	//
	// Indexing is a burst: every worker holds a file's text and its parse result,
	// and the heap the runtime grows to accommodate that is many times what the
	// index keeps. Go's scavenger returns it eventually and unhurriedly, so
	// without this the process sits at its indexing peak for a long time
	// afterwards — which for a daemon means the resident size a user sees is the
	// cost of a step it finished minutes ago.
	//
	// It forces a collection, so it belongs exactly here: once, at the end of a
	// phase that has just finished, and never on a path serving a request.
	var before runtime.MemStats

	runtime.ReadMemStats(&before)
	debug.FreeOSMemory()

	var after runtime.MemStats

	runtime.ReadMemStats(&after)

	s.log.Info("indexing complete",
		cflog.Int("files", indexed), cflog.Int("total", total),
		cflog.Duration("dur", time.Since(indexStart)),
		cflog.String("heapInUse", mb(after.HeapAlloc)),
		cflog.String("heapReserved", mb(after.HeapSys-after.HeapReleased)),
		cflog.String("returnedToOS", mb(sub(after.HeapReleased, before.HeapReleased))))
}

// sub is a-b without wrapping.
//
// HeapReleased can go down between two reads: the runtime counts pages it has
// handed back, and it takes them again when the heap next grows. An unguarded
// subtraction of two uint64s then prints 17592186044416MB, which is what this
// log line did the first time the number went the other way.
func sub(a, b uint64) uint64 {
	if a < b {
		return 0
	}

	return a - b
}

// mb formats a byte count for a log line.
func mb(b uint64) string {
	return strconv.FormatFloat(float64(b)/(1<<20), 'f', 1, 64) + "MB"
}

// applyIndexResult writes one parsed file into the index: its signatures and
// component refs, its this-scoped vars, and its ORM entity registration when
// the component is persistent and sits in ORM scope.
//
// This is the single definition of "what indexing a file means", called by the
// startup walk (indexWorkspace, indexRoot) and by the watched-file handler. It
// was previously written out at each of those sites, which is the shape that
// lets a later hop be added to one and forgotten at the others — a file picked
// up by a watcher would then be indexed differently from the identical file
// that happened to exist at startup.
func (s *Server) applyIndexResult(fileURI uri.URI, path string, pr *parser.ParseResult) {
	s.index.IndexFileFromResult(fileURI, pr.Funcs, pr.ComponentRefs)
	s.index.SetThisVars(fileURI, pr.ThisVars())

	if pr.Persistent && s.isOrmPath(path) {
		s.index.SetEntity(cfpath.CfcNameFromURI(string(fileURI)), fileURI)
	}
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
			name := info.Name()
			if name == ".git" || name == "node_modules" || name == ".svn" || name == "target" || name == "vendor" {
				return filepath.SkipDir
			}

			return nil
		}

		if cfpath.IsCFCFile(path) {
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
	s.log.Info("indexing workspace", cflog.String("root", root))

	err := s.FS.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if cfpath.IsCFCFile(path) {
			fileURI := uri.File(path)
			// Skip files already open in the editor — their buffer
			// content was indexed via didOpen and may be newer than disk.
			if _, open := s.getDocument(fileURI); open {
				return nil
			}

			data, err := s.FS.ReadFile(path)
			if err != nil {
				s.log.Warn("skipping file", cflog.String("path", path), cflog.Err(err))

				return nil
			}

			if cfpath.IsBinary(data) {
				return nil
			}

			s.applyIndexResult(fileURI, path, s.parseContentForIndex(fileURI, string(data)))
		}

		return nil
	})
	if err != nil {
		s.log.Error("failed to walk directory", cflog.String("root", root), cflog.Err(err))
	}
}
