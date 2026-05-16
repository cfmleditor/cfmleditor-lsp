package formatter

import (
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

type preformatEdit struct {
	start, end  uint
	replacement string
}

// preformat rewrites the source to convert elements with implicit_end_tag
// into self-closing tags (e.g. <br> → <br />), then re-parses.
// Body content after the start_tag is moved outside the self-closing tag.
func preformat(src []byte, tree *sitter.Tree, parse func([]byte) *sitter.Tree) ([]byte, *sitter.Tree) {
	var edits []preformatEdit
	collectEdits(tree.RootNode(), src, &edits)
	if len(edits) == 0 {
		return src, tree
	}

	// Apply edits in reverse order to preserve byte offsets.
	result := string(src)
	for i := len(edits) - 1; i >= 0; i-- {
		e := edits[i]
		result = result[:e.start] + e.replacement + result[e.end:]
	}

	newSrc := []byte(result)
	newTree := parse(newSrc)
	return newSrc, newTree
}

func collectEdits(n *sitter.Node, src []byte, edits *[]preformatEdit) {
	if n.Kind() == "element" {
		if tryConvertToSelfClosing(n, src, edits) {
			return
		}
	}
	for i := uint(0); i < n.ChildCount(); i++ {
		collectEdits(n.Child(i), src, edits)
	}
}

func tryConvertToSelfClosing(n *sitter.Node, src []byte, edits *[]preformatEdit) bool {
	var startTag *sitter.Node
	hasEndTag := false
	hasImplicitEnd := false
	var bodyStart, bodyEnd uint

	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		switch c.Kind() {
		case "start_tag":
			startTag = c
		case "end_tag":
			hasEndTag = true
		case "implicit_end_tag":
			hasImplicitEnd = true
		}
	}

	if startTag == nil || hasEndTag || !hasImplicitEnd {
		return false
	}

	// Collect body content (everything between start_tag and implicit_end_tag).
	bodyStart = startTag.EndByte()
	bodyEnd = bodyStart
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		if c.Kind() != "start_tag" && c.Kind() != "implicit_end_tag" {
			if c.EndByte() > bodyEnd {
				bodyEnd = c.EndByte()
			}
		}
	}

	// Build the self-closing tag: replace ">" with " />"
	startText := string(src[startTag.StartByte():startTag.EndByte()])
	if !strings.HasSuffix(startText, ">") {
		return false
	}
	selfClosing := startText[:len(startText)-1] + " />"

	// The replacement for the entire element: self-closing tag + body content after it.
	body := string(src[bodyStart:bodyEnd])
	*edits = append(*edits, preformatEdit{
		start:       uint(n.StartByte()),
		end:         uint(n.EndByte()),
		replacement: selfClosing + body,
	})
	return true
}
