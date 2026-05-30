package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/language"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

func cmdScan(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: cfmleditor-lsp scan <file-or-dir> [...]\n")
		os.Exit(1)
	}

	var files []string

	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s: %v\n", arg, err)
			os.Exit(1)
		}

		if info.IsDir() {
			filepath.Walk(arg, func(path string, _ os.FileInfo, err error) error { //nolint:errcheck
				if err != nil {
					return nil //nolint:nilerr
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

	var totalErrors int

	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  skip %s: %v\n", f, err)

			continue
		}

		if cfpath.IsBinary(content) {
			continue
		}

		tree := language.Parse(language.CFML, content, nil)
		if tree == nil {
			continue
		}

		if tree.RootNode().HasError() {
			printErrors(f, "cfml", tree.RootNode(), content)
		}

		// Scan injection languages (cfscript, cfquery)
		scanInjections(f, tree.RootNode(), content)
		tree.Close()
	}

	if totalErrors == 0 {
		fmt.Printf("No parse errors found in %d files.\n", len(files))
	}
}

func printErrors(file string, lang string, n *sitter.Node, src []byte) {
	printErrorNodes(file, lang, n, src)
}

func printErrorNodes(file string, lang string, n *sitter.Node, src []byte) {
	if n.IsError() || n.IsMissing() {
		pos := n.StartPosition()

		snippet := string(src[n.StartByte():n.EndByte()])
		if len(snippet) > 50 {
			snippet = snippet[:50] + "..."
		}

		snippet = strings.ReplaceAll(snippet, "\n", "\\n")

		if n.IsMissing() {
			fmt.Printf("%s:%d:%d: [%s] missing %s\n", file, pos.Row+1, pos.Column+1, lang, n.Kind())
		} else {
			fmt.Printf("%s:%d:%d: [%s] parse error near %q\n", file, pos.Row+1, pos.Column+1, lang, snippet)
		}

		return
	}

	for i := uint(0); i < n.ChildCount(); i++ {
		printErrorNodes(file, lang, n.Child(i), src)
	}
}

func scanInjections(file string, node *sitter.Node, src []byte) {
	var walk func(*sitter.Node)

	walk = func(n *sitter.Node) {
		switch n.Kind() {
		case "cf_script_content":
			content := src[n.StartByte():n.EndByte()]

			tree := language.Parse(language.CFScript, content, nil)
			if tree != nil {
				if tree.RootNode().HasError() {
					printErrorsOffset(file, "cfscript", tree.RootNode(), content, n.StartPosition().Row)
				}

				tree.Close()
			}
		case "cf_query_content":
			content := src[n.StartByte():n.EndByte()]

			tree := language.Parse(language.CFQuery, content, nil)
			if tree != nil {
				if tree.RootNode().HasError() {
					printErrorsOffset(file, "cfquery", tree.RootNode(), content, n.StartPosition().Row)
				}

				tree.Close()
			}
		}

		for i := uint(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(node)
}

func printErrorsOffset(file string, lang string, n *sitter.Node, src []byte, lineOffset uint) {
	var walk func(*sitter.Node)

	walk = func(n *sitter.Node) {
		if n.IsError() || n.IsMissing() {
			pos := n.StartPosition()

			snippet := string(src[n.StartByte():n.EndByte()])
			if len(snippet) > 50 {
				snippet = snippet[:50] + "..."
			}

			snippet = strings.ReplaceAll(snippet, "\n", "\\n")

			if n.IsMissing() {
				fmt.Printf("%s:%d:%d: [%s] missing %s\n", file, pos.Row+lineOffset+1, pos.Column+1, lang, n.Kind())
			} else {
				fmt.Printf("%s:%d:%d: [%s] parse error near %q\n", file, pos.Row+lineOffset+1, pos.Column+1, lang, snippet)
			}

			return
		}

		for i := uint(0); i < n.ChildCount(); i++ {
			walk(n.Child(i))
		}
	}
	walk(n)
}
