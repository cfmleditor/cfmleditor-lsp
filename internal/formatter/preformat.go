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
// The returned owned tree, when non-nil, is the caller's to Close; the tree
// passed in stays the caller's own either way.
func preformat(src []byte, tree *sitter.Tree, parse func([]byte) *sitter.Tree) (outSrc []byte, outTree *sitter.Tree, owned *sitter.Tree) {
	// Converting an element replaces it whole, so collectEdits cannot descend
	// into its body on the same pass and any void element nested there is left
	// alone. Repeat until the source stops changing — otherwise the first
	// format leaves conversions that a second format performs, and an
	// unchanged file keeps producing a new diff on every save.
	const maxPasses = 10

	for range maxPasses {
		var edits []preformatEdit

		collectEdits(tree.RootNode(), src, &edits)

		if len(edits) == 0 {
			break
		}

		// Apply edits in reverse order to preserve byte offsets.
		result := string(src)

		for i := len(edits) - 1; i >= 0; i-- {
			e := edits[i]
			result = result[:e.start] + e.replacement + result[e.end:]
		}

		src = []byte(result)

		if owned != nil {
			owned.Close()
		}

		owned = parse(src)
		tree = owned
	}

	return src, tree, owned
}

func collectEdits(n *sitter.Node, src []byte, edits *[]preformatEdit) {
	if n.Kind() == "element" {
		tryConvertToSelfClosing(n, src, edits)
	}

	// Descend even into an element that was just converted. Its edit touches
	// only its own start tag, so an edit collected inside its body cannot
	// overlap. Unclosed markup nests each following element inside the last —
	// a run of unclosed <tr>/<td> is twenty levels deep — and stopping here
	// converted one level per pass. Deeper nesting than maxPasses was left half
	// converted, finished by the *next* run of the formatter, so an unchanged
	// file kept producing a fresh diff.
	for i := uint(0); i < n.ChildCount(); i++ {
		collectEdits(n.Child(i), src, edits)
	}
}

func tryConvertToSelfClosing(n *sitter.Node, src []byte, edits *[]preformatEdit) bool {
	var startTag *sitter.Node

	hasEndTag := false
	hasImplicitEnd := false

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

	// Build the self-closing tag: replace ">" with " />"
	startText := string(src[startTag.StartByte():startTag.EndByte()])
	if !strings.HasSuffix(startText, ">") {
		return false
	}

	selfClosing := startText[:len(startText)-1] + " />"

	// Rewrite the start tag alone and leave the body where it is. Replacing the
	// whole element and re-appending its body came to the same text, but it
	// claimed the body's byte range, so no edit could be collected inside it on
	// the same pass. Keeping the edit narrow is what lets the walk carry on
	// through nested elements.
	*edits = append(*edits, preformatEdit{
		start:       startTag.StartByte(),
		end:         startTag.EndByte(),
		replacement: selfClosing,
	})

	return true
}
