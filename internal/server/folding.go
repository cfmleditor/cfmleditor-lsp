package server

import (
	"context"
	"strings"

	"encoding/json/v2"

	"github.com/cfmleditor/cfmleditor-lsp/internal/language"
	sitter "github.com/tree-sitter/go-tree-sitter"
	"go.lsp.dev/protocol"
)

// handleFoldingRange answers textDocument/foldingRange, so the editor folds on
// CFML structure instead of falling back to indentation.
//
// The ranges come from the tree-sitter CST, which is the only thing that knows
// where a `<cfif>` ends. Indentation guessing gets tags wrong constantly:
// a `<cfelse>` is written at the same indent as its `<cfif>`, and a tag whose
// attributes wrap looks like a block of its own.
//
// A script-syntax `.cfc` is the case that makes this more than a tree walk. The
// CFML grammar hands its whole body to the CFScript grammar as one opaque
// `cf_component_content` — the same injection the formatter uses — so walking
// only the outer tree yields exactly one fold for the entire file. Each
// injected region is therefore parsed with its own grammar and walked too, its
// rows offset by where the region starts.
func (s *Server) handleFoldingRange(_ context.Context, rawParams []byte) (any, error) {
	if !s.Features.Folding {
		return nil, nil
	}

	var params protocol.FoldingRangeParams
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return nil, err
	}

	content, ok := s.getDocument(params.TextDocument.URI)
	if !ok {
		return nil, nil
	}

	return foldingRanges(content), nil
}

// injectionGrammars maps an injections.scm language name onto its grammar.
var injectionGrammars = map[string]language.Grammar{
	"cfscript": language.CFScript,
	"cfquery":  language.CFQuery,
}

func foldingRanges(content string) []protocol.FoldingRange {
	src := []byte(content)

	tree := language.Parse(language.CFML, src, nil)
	if tree == nil {
		return []protocol.FoldingRange{}
	}

	// The tree owns C memory the Go GC does not account for, the same reason
	// formatDocument closes its own.
	defer tree.Close()

	// An injected region's content node is skipped in the outer walk: its
	// inside is opaque there, and the sub-parse below covers the same span with
	// the structure actually in it.
	injections := language.FindInjections(tree, src)
	opaque := make(map[uintptr]bool, len(injections))

	for _, m := range injections {
		opaque[m.Node.Id()] = true
	}

	var folds []protocol.FoldingRange

	collectFolds(tree.RootNode(), src, 0, opaque, &folds, 0)

	for _, m := range injections {
		g, known := injectionGrammars[m.Language]
		if !known {
			continue
		}

		region := src[m.Node.StartByte():m.Node.EndByte()]

		sub := language.Parse(g, region, nil)
		if sub == nil {
			continue
		}

		// Walked with the region's own bytes, since the sub-tree's positions
		// are relative to them; row 0 of the region is the row it starts on.
		collectFolds(sub.RootNode(), region, uint32(m.Node.StartPosition().Row), opaque, &folds, 0)
		sub.Close()
	}

	return dedupeFolds(folds)
}

// collectFolds walks a tree pre-order, so an enclosing construct is recorded
// before the constructs inside it, which is what makes dedupeFolds keep the
// outer one of two nodes covering the same lines.
// Every accessor on a tree-sitter node is a cgo call into the C grammar, and
// this walk visits every node in the file: it measured 71% runtime.cgocall.
// What the body below is arranged around is therefore the *number of accessor
// calls per node*, not the work between them.
//
// Three things follow from that. depth replaces a Parent() call that existed
// only to recognise the root. Range() reads both positions and both byte
// offsets in one call where StartPosition/EndPosition/EndByte were three. And
// the cheap rejection — a node that begins and ends on one line cannot fold —
// runs before anything else, because most nodes in a file are single-line and
// each one that stops here stops after a single accessor.
func collectFolds(n *sitter.Node, src []byte, rowOffset uint32, opaque map[uintptr]bool, out *[]protocol.FoldingRange, depth int) {
	// Id() is itself a call, so it is only worth asking when something can be
	// opaque — that is, when the file has injected regions at all.
	if len(opaque) > 0 && opaque[n.Id()] {
		return
	}

	if depth > 0 {
		if f, ok := foldFor(n, src, rowOffset); ok && !wrapsOnlyOpaque(n, opaque) {
			*out = append(*out, f)
		}
	}

	// Named children only. An anonymous node is a literal token from the
	// grammar and tokens are leaves, so no named node hides under one — and a
	// node that is not named can never fold. Skipping them drops roughly half
	// the tree, and with it half the accessor calls this walk is made of.
	// TestFoldingWalksEveryFoldableNode pins the leaf assumption.
	for i := range n.NamedChildCount() {
		collectFolds(n.NamedChild(i), src, rowOffset, opaque, out, depth+1)
	}
}

// wrapsOnlyOpaque reports whether n exists solely to hold an injected region
// that covers exactly the same lines.
//
// Such a node has no tokens of its own to fold to, and its range is the whole
// region — for a script-syntax `.cfc` the CFML grammar wraps the body in a
// component_file spanning the entire document, which folds the file down to
// nothing. Its useful structure comes from the sub-parse instead. A
// `<cfscript>` block is the case this must *not* catch: its content node starts
// on the line after the opening tag, so the extents differ and the block keeps
// its fold.
func wrapsOnlyOpaque(n *sitter.Node, opaque map[uintptr]bool) bool {
	// Nothing is opaque in a file with no injected regions, which is every
	// tag-syntax document, and walking its named children to discover that is
	// a cgo call per child.
	if len(opaque) == 0 {
		return false
	}

	for i := range n.NamedChildCount() {
		c := n.NamedChild(i)
		if !opaque[c.Id()] {
			continue
		}

		if c.StartPosition().Row == n.StartPosition().Row && c.EndPosition().Row == n.EndPosition().Row {
			return true
		}
	}

	return false
}

// foldFor turns one node into a folding range, or reports that it is not worth
// folding.
func foldFor(n *sitter.Node, src []byte, rowOffset uint32) (protocol.FoldingRange, bool) {
	// One call for both positions and both byte offsets, and the first thing
	// asked of the node: a single-line node cannot fold, and most nodes are
	// single-line.
	r := n.Range()

	start := uint32(r.StartPoint.Row)
	end := uint32(r.EndPoint.Row)

	if end <= start {
		return protocol.FoldingRange{}, false
	}

	comment := strings.Contains(n.Kind(), "comment")

	// A node with no named children is a run of text — html_text, a raw body —
	// and folding text nobody structured is noise in the gutter. A comment is
	// the exception: it has no inner structure and is exactly the thing a
	// reader wants collapsed.
	if !comment && n.NamedChildCount() == 0 {
		return protocol.FoldingRange{}, false
	}

	// Keep the closing line visible. Folding through it hides the delimiter and
	// the fold reads as if the construct had been deleted. Two shapes need
	// catching, and each is the only one that catches its cases:
	//
	//   - The node's last line is nothing but its own closing token, so the
	//     fold stops the line before. The token has to be found by descending
	//     to the node's deepest last token rather than reading its immediate
	//     last child: a function_declaration's last child is the
	//     statement_block, and the `}` is the *block's* last child, so the
	//     shallow check silently missed every wrapper node.
	//   - The node ends inside the leading whitespace of its last line, which
	//     means the line is its parent's. `<cfelse>`'s branch ends at the tab
	//     before `</cfif>`; that `</cfif>` belongs to the enclosing `<cfif>`,
	//     so the branch has no closing token of its own to find.
	//
	// Comments hit neither and fold to their last line, which is right: their
	// last line carries content.
	switch {
	case endsInLeadingWhitespace(r, src):
		end--
	default:
		if last := deepestLastToken(n); last != nil {
			if closing := uint32(last.StartPosition().Row); closing == end && closing > start {
				end = closing - 1
			}
		}
	}

	if end <= start {
		return protocol.FoldingRange{}, false
	}

	f := protocol.FoldingRange{StartLine: start + rowOffset, EndLine: end + rowOffset}
	if comment {
		f.Kind = protocol.FoldingRangeKindComment
	}

	return f, true
}

// endsInLeadingWhitespace reports whether everything on the node's last line,
// up to where the node ends, is whitespace — so the node contributes nothing to
// that line.
// It takes the range its caller already read rather than re-reading the node,
// which was two more cgo calls for a node that had got this far.
func endsInLeadingWhitespace(r sitter.Range, src []byte) bool {
	endByte := r.EndByte
	col := r.EndPoint.Column

	if endByte > uint(len(src)) || col > endByte {
		return false
	}

	return strings.TrimSpace(string(src[endByte-col:endByte])) == ""
}

// deepestLastToken returns the last leaf under n — the token that closes it,
// when it has one.
func deepestLastToken(n *sitter.Node) *sitter.Node {
	for n != nil && n.ChildCount() > 0 {
		n = n.Child(n.ChildCount() - 1)
	}

	return n
}

// dedupeFolds drops ranges covering lines an earlier range already covers.
//
// The CST nests wrappers that add no lines of their own — a function_declaration
// around its statement_block, an element around its html_text, a component
// around its component_body — and each would otherwise contribute an identical
// fold. Pre-order collection means the survivor is the outermost, which is the
// one whose kind a client would want to show.
func dedupeFolds(folds []protocol.FoldingRange) []protocol.FoldingRange {
	out := make([]protocol.FoldingRange, 0, len(folds))
	seen := make(map[[2]uint32]bool, len(folds))

	for _, f := range folds {
		key := [2]uint32{f.StartLine, f.EndLine}
		if seen[key] {
			continue
		}

		seen[key] = true

		out = append(out, f)
	}

	return out
}
