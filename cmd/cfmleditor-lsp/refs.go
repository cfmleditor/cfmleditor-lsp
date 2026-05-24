package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/refs"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

// RefResult holds all references to a target.
type RefResult struct {
	Target string       `json:"target"`
	Files  int          `json:"filesScanned"`
	Refs   []refs.Entry `json:"refs"`
}

func cmdRefs(args []string) {
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

	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: cfmleditor-lsp refs [--mermaid] <component-or-function> <dir> [...]\n")
		fmt.Fprintf(os.Stderr, "  e.g. cfmleditor-lsp refs packages.finance.service ./src\n")
		fmt.Fprintf(os.Stderr, "       cfmleditor-lsp refs getReport ./src\n")
		os.Exit(1)
	}

	target := args[0]
	dirs := args[1:]
	resolvers := loadResolversFromConfig(dirs)
	fsys := vfs.OS{}

	isComponentTarget := strings.Contains(target, ".")
	var entries []refs.Entry
	if isComponentTarget {
		entries = refs.FindComponentRefs(fsys, dirs, target, resolvers)
	} else {
		entries = refs.FindCalls(fsys, dirs, target, resolvers)
	}

	result := RefResult{
		Target: target,
		Refs:   entries,
	}

	switch format {
	case "mermaid":
		printMermaidRefs(target, entries)
	default:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(result)
	}
}

func printMermaidRefs(target string, entries []refs.Entry) {
	fmt.Println("graph LR")
	targetNode := strings.ReplaceAll(target, ".", "_")
	fmt.Printf("    %s[%s]\n", targetNode, target)
	for i, ref := range entries {
		nodeID := fmt.Sprintf("ref%d", i)
		label := filepath.Base(ref.File)
		if ref.Function != "" {
			label += "::" + ref.Function
		}
		style := "-->"
		if !ref.Resolved {
			style = "-.->"
		}
		fmt.Printf("    %s[%s] %s %s\n", nodeID, label, style, targetNode)
	}
}
