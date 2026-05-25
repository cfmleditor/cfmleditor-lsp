package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/uri"
)

// DepEdge represents a dependency from one function to another.
type DepEdge struct {
	FromFile     string `json:"fromFile"`
	FromFunction string `json:"fromFunction,omitempty"`
	ToComponent  string `json:"toComponent"`
	ToFunction   string `json:"toFunction,omitempty"`
	Line         uint32 `json:"line"`
}

// DepResult holds the full dependency graph.
type DepResult struct {
	Files int       `json:"files"`
	Edges []DepEdge `json:"edges"`
}

func cmdDeps(args []string) {
	format := "json"
	var filteredArgs []string
	for _, a := range args {
		switch a {
		case "--mermaid":
			format = "mermaid"
		default:
			filteredArgs = append(filteredArgs, a)
		}
	}
	args = filteredArgs

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: cfmleditor-lsp deps [--mermaid] <dir-or-file> [...]\n")
		os.Exit(1)
	}

	// Collect all CFML files
	var files []string
	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s: %v\n", arg, err)
			os.Exit(1)
		}
		if info.IsDir() {
			_ = filepath.Walk(arg, func(path string, fi os.FileInfo, _ error) error {
				if fi.IsDir() {
					return nil
				}
				ext := strings.ToLower(filepath.Ext(path))
				if ext == ".cfc" || ext == ".cfm" || ext == ".cfml" || ext == ".cfs" {
					files = append(files, path)
				}
				return nil
			})
		} else {
			files = append(files, arg)
		}
	}

	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "no CFML files found\n")
		os.Exit(1)
	}

	edges := make([]DepEdge, 0, len(files))
	resolvers := loadResolversFromConfig(args)

	type fileResult struct {
		entries []DepEdge
	}

	workers := 8
	fileCh := make(chan string, len(files))
	resultCh := make(chan fileResult, len(files))

	for i := 0; i < workers; i++ {
		go func() {
			for f := range fileCh {
				content, err := os.ReadFile(f)
				if err != nil {
					resultCh <- fileResult{}
					continue
				}
				absPath, _ := filepath.Abs(f)
				fileURI := uri.URI("file://" + absPath)
				pr := parser.ParseWithOptions(fileURI, string(content), parser.ParseOptions{
					Resolvers: resolvers,
				})

				var entries []DepEdge

				for _, ref := range pr.Refs {
					entries = append(entries, DepEdge{
						FromFile: f, ToComponent: ref.Component, Line: ref.Line,
					})
				}

				for _, sc := range pr.Scopes {
					funcName := ""
					for _, fn := range pr.Funcs {
						if int(fn.Line) == sc.Start {
							funcName = fn.Name
							break
						}
					}
					refs, _ := pr.FuncRefs(sc.Start, sc.End)
					for _, ref := range refs {
						entries = append(entries, DepEdge{
							FromFile: f, FromFunction: funcName, ToComponent: ref.Component, Line: ref.Line,
						})
					}
				}

				resultCh <- fileResult{entries: entries}
			}
		}()
	}

	for _, f := range files {
		fileCh <- f
	}
	close(fileCh)

	for range files {
		r := <-resultCh
		edges = append(edges, r.entries...)
	}

	result := DepResult{
		Files: len(files),
		Edges: edges,
	}

	switch format {
	case "mermaid":
		printMermaidDeps(result)
	default:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(result)
	}
}

func printMermaidDeps(result DepResult) {
	fmt.Println("graph LR")
	seen := make(map[string]bool)
	for _, e := range result.Edges {
		from := filepath.Base(e.FromFile)
		if e.FromFunction != "" {
			from += "::" + e.FromFunction
		}
		to := e.ToComponent
		key := from + "|" + to
		if seen[key] {
			continue
		}
		seen[key] = true
		fromID := strings.ReplaceAll(strings.ReplaceAll(from, ".", "_"), ":", "_")
		toID := strings.ReplaceAll(to, ".", "_")
		fmt.Printf("    %s[%s] --> %s[%s]\n", fromID, from, toID, to)
	}
}
