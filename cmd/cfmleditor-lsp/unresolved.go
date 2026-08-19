package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cfmleditor/cfmleditor-lsp/internal/daemon"
	"github.com/cfmleditor/cfmleditor-lsp/internal/docs"
	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"github.com/cfmleditor/cfmleditor-lsp/internal/resolve"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
	"go.lsp.dev/uri"
)

type UnresolvedCall struct {
	File     string `json:"file"`
	Line     uint32 `json:"line"`
	Caller   string `json:"caller,omitempty"`
	Variable string `json:"variable,omitempty"`
	Function string `json:"function"`
	Reason   string `json:"reason"`
	Text     string `json:"text"`
}

func cmdUnresolved(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: cfmleditor-lsp unresolved [--json] [--global-defs] <dir> [...]\n")
		os.Exit(1)
	}

	jsonOutput := false
	verbose := false
	matchGlobalDefs := false

	var filteredArgs []string

	for _, a := range args {
		switch a {
		case "--json":
			jsonOutput = true
		case "--verbose":
			verbose = true
		case "--global-defs":
			matchGlobalDefs = true
		default:
			filteredArgs = append(filteredArgs, a)
		}
	}

	args = filteredArgs

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: cfmleditor-lsp unresolved [--json] [--global-defs] <dir> [...]\n")
		os.Exit(1)
	}

	fsys := vfs.OS{}

	// Find .cfmleditor.json config based on the first file/dir argument
	searchDir, _ := filepath.Abs(args[0])
	if info, err := os.Stat(searchDir); err == nil && !info.IsDir() {
		searchDir = filepath.Dir(searchDir)
	}

	var (
		cfResolvers              []parser.Resolver
		mappings                 map[string]string
		expressionMappings       map[string]string
		servicePropertyResolvers map[string]string
		workspaceFolders         []string
	)

	cfg, _ := daemon.FindConfig(searchDir)
	if cfg != nil {
		workspaceFolders = cfg.WorkspaceFolders()

		mappings = cfg.Mappings()

		expressionMappings = cfg.ExpressionMappings()

		servicePropertyResolvers = cfg.ServicePropertyResolvers()
		for _, r := range cfg.ComponentResolvers() {
			cfResolvers = append(cfResolvers, parser.Resolver{Match: r.Match, Resolve: r.Resolve, Prefix: r.Prefix, NoFollow: r.NoFollow, Anchored: r.Anchored})
		}

		fmt.Fprintf(os.Stderr, "Using config: %s\n", cfg.Path)
	} else {
		// Fallback: use args as workspace folders
		for _, a := range args {
			if info, err := os.Stat(a); err == nil && info.IsDir() {
				abs, _ := filepath.Abs(a)
				workspaceFolders = append(workspaceFolders, abs)
			}
		}
	}

	// Collect files from workspace folders or args
	var scanRoots []string
	if len(workspaceFolders) > 0 {
		scanRoots = workspaceFolders
	} else {
		scanRoots = args
	}

	files := collectCFMLFiles(fsys, scanRoots)

	resolver := &resolve.Resolver{
		FS:                 fsys,
		Index:              index.New(),
		Resolvers:          cfResolvers,
		Mappings:           mappings,
		ExpressionMappings: expressionMappings,
		WorkspaceFolders:   workspaceFolders,
	}

	// First pass: index all CFC files for function lookups
	fmt.Fprintf(os.Stderr, "Indexing %d files...\n", len(files))

	indexStart := time.Now()

	for _, f := range files {
		if !cfpath.IsCFCFile(f) {
			continue
		}

		data, err := fsys.ReadFile(f)
		if err != nil || cfpath.IsBinary(data) {
			continue
		}

		fileURI := uri.URI("file://" + f)
		resolver.Index.IndexFile(fileURI, string(data))
	}

	// Filter scan targets if specific files were passed
	indexDur := time.Since(indexStart)

	var scanFiles []string

	for _, a := range args {
		if info, err := os.Stat(a); err == nil && !info.IsDir() {
			abs, _ := filepath.Abs(a)
			scanFiles = append(scanFiles, abs)
		}
	}

	if len(scanFiles) == 0 {
		scanFiles = files
	}

	var (
		mu       sync.Mutex
		results  []UnresolvedCall
		resolved int
		wg       sync.WaitGroup
	)

	sem := make(chan struct{}, 8)

	fmt.Fprintf(os.Stderr, "Scanning %d files for unresolved calls...\n", len(scanFiles))

	scanStart := time.Now()

	for _, f := range scanFiles {
		wg.Add(1)

		sem <- struct{}{}

		go func(file string) {
			defer wg.Done()
			defer func() { <-sem }()

			data, err := fsys.ReadFile(file)
			if err != nil || cfpath.IsBinary(data) {
				return
			}

			content := string(data)
			fileURI := uri.URI("file://" + file)
			baseDir := filepath.Dir(file)

			funcLookup := func(component, funcName string) string {
				fd := resolver.ResolveFunc(component, funcName, baseDir)
				if fd == nil {
					return ""
				}

				if fd.ReturnComponent != "" {
					return fd.ReturnComponent
				}

				if fd.ReturnType != "" && strings.Contains(fd.ReturnType, ".") {
					return fd.ReturnType
				}

				return ""
			}

			pr := parser.ParseWithOptions(fileURI, content, parser.ParseOptions{
				Resolvers:                cfResolvers,
				ExpressionMappings:       expressionMappings,
				ServicePropertyResolvers: servicePropertyResolvers,
				ExtractCalls:             true,
				ScanAllScopes:            true,
				FuncLookup:               funcLookup,
				BuiltinReturnLookup:      docs.LookupBuiltinReturnComponent,
			})

			pr.FuncLookup = funcLookup

			lastLine := strings.Count(content, "\n")

			calls := pr.FuncCalls(0, lastLine)
			for _, call := range calls {
				reason := resolver.CanResolveCall(call, pr, baseDir)
				if reason == "" {
					mu.Lock()
					resolved++
					mu.Unlock()

					if verbose {
						fmt.Fprintf(os.Stderr, "  ✓ %s:%d: %s.%s\n", filepath.Base(file), call.Line+1, call.Variable, call.FuncName)
					}

					continue
				}

				if isBuiltin(call.FuncName) {
					continue
				}

				if matchGlobalDefs && len(resolver.Index.Lookup(call.FuncName)) > 0 {
					mu.Lock()
					resolved++
					mu.Unlock()

					if verbose {
						fmt.Fprintf(os.Stderr, "  ✓ %s:%d: %s (global def)\n", filepath.Base(file), call.Line+1, call.FuncName)
					}

					continue
				}

				mu.Lock()

				results = append(results, UnresolvedCall{
					File:     file,
					Line:     call.Line,
					Caller:   call.Caller,
					Variable: call.Variable,
					Function: call.FuncName,
					Reason:   reason,
					Text:     call.Text,
				})
				mu.Unlock()
			}
		}(f)
	}

	wg.Wait()

	if len(results) == 0 {
		fmt.Fprintf(os.Stderr, "No unresolved calls found.\n")

		return
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(results)
	} else {
		for _, r := range results {
			call := r.Function
			if r.Variable != "" {
				call = r.Variable + "." + r.Function
			}

			fmt.Printf("%s:%d: %s (%s)\n", r.File, r.Line+1, call, r.Reason)
		}
	}

	fmt.Fprintf(os.Stderr, "%d unresolved calls found (%d resolved)\n", len(results), resolved)

	scanDur := time.Since(scanStart)

	fmt.Fprintf(os.Stderr, "\nBenchmark:\n")
	fmt.Fprintf(os.Stderr, "  Index:  %v (%d files)\n", indexDur, len(files))
	fmt.Fprintf(os.Stderr, "  Scan:   %v (%d files)\n", scanDur, len(scanFiles))
	fmt.Fprintf(os.Stderr, "  Total:  %v\n", indexDur+scanDur)
}

func isBuiltin(name string) bool {
	_, ok := docs.LookupFunction(name)
	if ok {
		return true
	}

	if parser.IsMemberMethod(name) {
		return true
	}

	return isMemberFunction(name)
}

var memberFuncSet = func() map[string]bool {
	m := make(map[string]bool)
	for _, mf := range docs.AllMemberFunctions() {
		m[strings.ToLower(mf.Name)] = true
	}

	return m
}()

func isMemberFunction(name string) bool {
	return memberFuncSet[strings.ToLower(name)]
}

func collectCFMLFiles(fsys vfs.FS, roots []string) []string {
	var files []string

	for _, root := range roots {
		_ = fsys.Walk(root, func(path string, info os.FileInfo, err error) error {
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

			if cfpath.IsCFMLFile(path) {
				files = append(files, path)
			}

			return nil
		})
	}

	return files
}
