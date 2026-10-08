package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cfmleditor/clif/internal/language"
	cfpath "github.com/cfmleditor/clif/internal/path"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

func cmdScan(args []string) {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, scanUsage)
		os.Exit(1)
	}

	var files []string

	for _, arg := range args {
		info, err := os.Stat(positional(arg, scanUsage))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s: %v\n", arg, err)
			os.Exit(1)
		}

		if info.IsDir() {
			files = append(files, cfmlFilesUnder(arg)...)
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
			totalErrors += printErrors(f, label, tree.RootNode(), content)
		}

		// Scan injection languages based on injections.scm rules
		totalErrors += scanInjections(f, tree, content)
		tree.Close()
	}

	if totalErrors == 0 {
		fmt.Printf("No parse errors found in %d files.\n", len(files))
	}
}

// printErrors reports every error node under n and returns how many it found,
// so the caller can tell "clean" from "reported problems" — the count it kept
// was never incremented, so `scan` printed each error and then finished with
// "No parse errors found in N files.".
func printErrors(file string, lang string, n *sitter.Node, src []byte) int {
	return printErrorNodes(file, lang, n, src)
}

func printErrorNodes(file string, lang string, n *sitter.Node, src []byte) int {
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

		return 1
	}

	count := 0
	for i := range n.ChildCount() {
		count += printErrorNodes(file, lang, n.Child(i), src)
	}

	return count
}

func scanInjections(file string, tree *sitter.Tree, src []byte) int {
	count := 0

	for _, inj := range language.FindInjections(tree, src) {
		grammar := language.GrammarForLanguage(inj.Language)
		if grammar < 0 {
			continue
		}

		content := src[inj.Node.StartByte():inj.Node.EndByte()]

		injTree := language.Parse(grammar, content, nil)
		if injTree != nil {
			if injTree.RootNode().HasError() {
				count += printErrorsOffset(file, inj.Language, injTree.RootNode(), content, inj.Node.StartPosition().Row)
			}

			injTree.Close()
		}
	}

	return count
}

func printErrorsOffset(file string, lang string, n *sitter.Node, src []byte, lineOffset uint) int {
	count := 0

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

			count++

			return
		}

		for i := range n.ChildCount() {
			walk(n.Child(i))
		}
	}

	walk(n)

	return count
}

// cfmlFilesUnder lists the CFML files beneath root, for the parse and scan
// commands. An unreadable entry is skipped rather than ending the walk, so the
// callback returns no error and Walk has none to report.
func cfmlFilesUnder(root string) []string {
	var files []string

	_ = filepath.Walk(root, func(path string, _ os.FileInfo, err error) error {
		if err == nil && hasCFMLExt(path) {
			files = append(files, path)
		}

		return nil
	})

	return files
}

// hasCFMLExt reports whether path names a file the parse and scan commands read.
func hasCFMLExt(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".cfc", ".cfm", ".cfml", ".cfs":
		return true
	}

	return false
}
