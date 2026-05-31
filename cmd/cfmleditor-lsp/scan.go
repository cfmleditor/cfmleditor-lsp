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

		// Determine initial grammar by extension (matching zed-cfml config)
		ext := strings.ToLower(filepath.Ext(f))

		var grammar language.Grammar
		if ext == ".cfs" {
			grammar = language.CFScript
		} else {
			grammar = language.CFML
		}

		tree := language.Parse(grammar, content, nil)
		if tree == nil {
			continue
		}

		label := "cfml"
		if grammar == language.CFScript {
			label = "cfscript"
		}

		if tree.RootNode().HasError() {
			printErrors(f, label, tree.RootNode(), content)
		}

		// Scan injection languages based on injections.scm rules
		scanInjections(f, tree, content)
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

func scanInjections(file string, tree *sitter.Tree, src []byte) {
	for _, inj := range language.FindInjections(tree, src) {
		grammar := language.GrammarForLanguage(inj.Language)
		if grammar < 0 {
			continue
		}

		content := src[inj.Node.StartByte():inj.Node.EndByte()]

		injTree := language.Parse(grammar, content, nil)
		if injTree != nil {
			if injTree.RootNode().HasError() {
				printErrorsOffset(file, inj.Language, injTree.RootNode(), content, inj.Node.StartPosition().Row)
			}

			injTree.Close()
		}
	}
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
