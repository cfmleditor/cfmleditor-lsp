package parser

import (
	tree_sitter_cfml "github.com/cfmleditor/tree-sitter-cfml/bindings/go"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

// Grammar identifies which tree-sitter grammar to use.
type Grammar int

const (
	CFML Grammar = iota
	CFScript
	CFQuery
)

// Languages returns the tree-sitter Language for the given grammar.
func Language(g Grammar) *sitter.Language {
	switch g {
	case CFML:
		return sitter.NewLanguage(tree_sitter_cfml.LanguageCfml())
	case CFScript:
		return sitter.NewLanguage(tree_sitter_cfml.LanguageCfscript())
	case CFQuery:
		return sitter.NewLanguage(tree_sitter_cfml.LanguageCfquery())
	default:
		return nil
	}
}

// NewParser creates a tree-sitter parser configured for the given grammar.
// The caller must call Close() on the returned parser when done.
func NewParser(g Grammar) *sitter.Parser {
	p := sitter.NewParser()
	p.SetLanguage(Language(g))
	return p
}

// Parse parses source code using the given grammar and returns the tree.
// The caller must call Close() on the returned tree when done.
func Parse(g Grammar, src []byte, oldTree *sitter.Tree) *sitter.Tree {
	p := NewParser(g)
	defer p.Close()
	return p.Parse(src, oldTree)
}

