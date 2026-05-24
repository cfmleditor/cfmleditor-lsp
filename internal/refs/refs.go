// Package refs provides shared logic for finding function/component references across files.
package refs

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/cfmleditor/cfmleditor-lsp/internal/cfparser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
	"go.lsp.dev/uri"
)

// Entry represents a single reference found.
type Entry struct {
	File     string `json:"file"`
	Function string `json:"function,omitempty"`
	Variable string `json:"variable,omitempty"`
	Call     string `json:"call,omitempty"`
	Line     uint32 `json:"line"`
	Resolved bool   `json:"resolved"`
}

// Options configures what to search for.
type Options struct {
	FuncName          string                        // function name to find calls to (empty = skip)
	Component         string                        // component dot-path to find refs to (empty = skip)
	Resolvers         []cfparser.Resolver           // resolvers for ParseWithOptions
	PropertyResolvers []cfparser.PropertyResolver   // property resolvers
	BeanLookup        func(string) string           // bean name → dot-path lookup
	VerifyCall        func(component, funcName, fileDir string) bool // optional: verify the component has this function
}

// Find scans all CFML files under roots and returns matching references.
func Find(fsys vfs.FS, roots []string, opts Options) []Entry {
	files := collectFiles(fsys, roots)
	return findInFiles(fsys, files, opts)
}

// FindCalls is a convenience wrapper for finding function calls.
func FindCalls(fsys vfs.FS, roots []string, funcName string, resolvers []cfparser.Resolver) []Entry {
	return Find(fsys, roots, Options{FuncName: funcName, Resolvers: resolvers})
}

// FindComponentRefs is a convenience wrapper for finding component references.
func FindComponentRefs(fsys vfs.FS, roots []string, component string, resolvers []cfparser.Resolver) []Entry {
	return Find(fsys, roots, Options{Component: component, Resolvers: resolvers})
}

func collectFiles(fsys vfs.FS, roots []string) []string {
	var files []string
	for _, root := range roots {
		_ = fsys.Walk(root, func(path string, info os.FileInfo, _ error) error {
			if info.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext == ".cfc" || ext == ".cfm" || ext == ".cfml" || ext == ".cfs" {
				files = append(files, path)
			}
			return nil
		})
	}
	return files
}

func findInFiles(fsys vfs.FS, files []string, opts Options) []Entry {
	var mu sync.Mutex
	var results []Entry

	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)

	funcTarget := strings.ToLower(opts.FuncName)
	compTarget := strings.ToLower(opts.Component)

	for _, f := range files {
		f := f
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			data, err := fsys.ReadFile(f)
			if err != nil {
				return
			}

			content := string(data)
			absPath, _ := filepath.Abs(f)
			fileURI := uri.URI("file://" + absPath)

			// Parse once with resolvers and call scanning — scan all scopes
			parseOpts := cfparser.ParseOptions{
				Resolvers:         opts.Resolvers,
				PropertyResolvers: opts.PropertyResolvers,
				ScanAllScopes:     true,
			}
			// Build per-file bean lookup from nearest Application.cfc
			if opts.BeanLookup != nil {
				parseOpts.BeanLookup = opts.BeanLookup
			} else {
				fileDir := filepath.Dir(absPath)
				parseOpts.BeanLookup = fileBeanLookup(fsys, fileDir)
			}
			if funcTarget != "" {
				parseOpts.FindCalls = []string{opts.FuncName}
			}
			pr := cfparser.ParseWithOptions(fileURI, content, parseOpts)

			var entries []Entry

			// Component ref matching (from parsed refs — includes all scopes)
			if compTarget != "" {
				for _, ref := range pr.Refs {
					if strings.EqualFold(ref.Component, compTarget) {
						entries = append(entries, Entry{
							File: f, Variable: ref.Variable, Line: ref.Line, Resolved: true,
						})
					}
				}
			}

			// Function call matching (from parsed call sites — includes all scopes)
			if funcTarget != "" {
				for _, call := range pr.Calls {
					if opts.VerifyCall != nil && call.Component != "" {
						if !opts.VerifyCall(call.Component, call.FuncName, filepath.Dir(absPath)) {
							continue
						}
					}
					entries = append(entries, Entry{
						File: f, Function: call.Caller, Call: call.Text,
						Line: call.Line, Resolved: call.Resolved,
					})
				}
			}

			if len(entries) > 0 {
				mu.Lock()
				results = append(results, entries...)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return results
}

// fileBeanLookup creates a BeanLookup function that resolves from the nearest Application.cfc.
func fileBeanLookup(fsys vfs.FS, dir string) func(string) string {
	appDir := findAppRoot(fsys, dir)
	if appDir == "" {
		return nil
	}
	beanPaths := cfpath.LoadAppBeanPaths(appDir)
	if len(beanPaths) == 0 {
		return nil
	}
	// Build bean map from paths
	beans := make(map[string]string)
	for _, root := range beanPaths {
		_ = fsys.Walk(root, func(path string, info os.FileInfo, _ error) error {
			if info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(strings.ToLower(path), ".cfc") {
				return nil
			}
			name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			beans[strings.ToLower(name)] = path
			return nil
		})
	}
	return func(name string) string {
		return beans[strings.ToLower(name)]
	}
}

func findAppRoot(fsys vfs.FS, dir string) string {
	d := dir
	for {
		for _, name := range []string{"Application.cfc", "Application.cfm"} {
			if _, err := fsys.Stat(filepath.Join(d, name)); err == nil {
				return d
			}
		}
		parent := filepath.Dir(d)
		if parent == d {
			return ""
		}
		d = parent
	}
}
