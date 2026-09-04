package formatter

import (
	"fmt"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// ParseError reports the first ERROR or MISSING node in tree, or nil when the
// tree is clean.
//
// Formatting a source file the grammar could not parse is how a CST gap turns
// into rewritten source: the walk emits whatever it can make sense of and
// silently drops the rest, leaving only [Options.WhitespaceOnly] between a
// grammar gap and the user's file. Both entry points that format a whole file
// — the LSP's textDocument/formatting handler and the `format` subcommand —
// gate on this first, so a file the editor refuses is a file the CLI refuses.
func ParseError(tree *sitter.Tree, src []byte) error {
	root := tree.RootNode()
	if !root.HasError() {
		return nil
	}

	errNode := findErrorNode(root)
	if errNode == nil {
		return fmt.Errorf("parse error in document, cannot format")
	}

	pos := errNode.StartPosition()

	snippet := string(src[errNode.StartByte():errNode.EndByte()])
	if len(snippet) > 50 {
		snippet = snippet[:50] + "..."
	}

	return fmt.Errorf("parse error at line %d, col %d near %q", pos.Row+1, pos.Column+1, snippet)
}

// findErrorNode returns the first ERROR or MISSING node in the subtree at n.
func findErrorNode(n *sitter.Node) *sitter.Node {
	if n.IsError() || n.IsMissing() {
		return n
	}

	for i := uint(0); i < n.ChildCount(); i++ {
		if found := findErrorNode(n.Child(i)); found != nil {
			return found
		}
	}

	return nil
}
