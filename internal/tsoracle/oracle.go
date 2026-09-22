// Package tsoracle compares what the hand-written parser extracts from a CFML
// file against what the tree-sitter grammar sees in the same file.
//
// The two are independent implementations — internal/parser is a line scanner
// and tag search, the grammar is a real parse — so where they disagree about
// something both should see, one of them is wrong. Every call-losing defect
// fixed in internal/parser so far was found by hand-probing constructs one at a
// time; this asks the question over a whole corpus instead.
package tsoracle

import (
	"sort"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/language"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

// Call is one call site, reduced to what both implementations can agree on: the
// line it sits on and the method being called. Receivers are deliberately not
// compared — the two spell a chained receiver differently (`a().b()` is a
// nested member_expression to the grammar and a base plus a Chain to the
// parser), and a difference there is a naming difference, not a missing call.
type Call struct {
	Line   uint32
	Method string
}

// GrammarCalls returns every call the tree-sitter grammars see in src,
// including the ones inside injected regions — a script component's body, a
// <cfquery> — which the outer CFML tree holds as one opaque node.
func GrammarCalls(src []byte) []Call {
	tree := language.Parse(language.CFML, src, nil)
	if tree == nil {
		return nil
	}

	defer tree.Close()

	injections := language.FindInjections(tree, src)

	opaque := make(map[uint]bool, len(injections))
	for _, m := range injections {
		opaque[m.Node.StartByte()] = true
	}

	calls := collect(tree.RootNode(), src, 0, opaque)

	for _, m := range injections {
		g, ok := grammarFor(m.Language)
		if !ok {
			continue
		}

		inner := src[m.Node.StartByte():m.Node.EndByte()]

		sub := language.Parse(g, inner, nil)
		if sub == nil {
			continue
		}

		calls = append(calls, collect(sub.RootNode(), inner, uint32(m.Node.StartPosition().Row), nil)...)

		sub.Close()
	}

	return calls
}

func grammarFor(name string) (language.Grammar, bool) {
	switch name {
	case "cfscript":
		return language.CFScript, true
	case "cfquery":
		return language.CFQuery, true
	default:
		return 0, false
	}
}

// collect walks named children only, for the reason the folding walk gives: an
// anonymous node is a grammar literal and tokens are leaves.
func collect(n *sitter.Node, src []byte, rowOffset uint32, opaque map[uint]bool) []Call {
	var out []Call

	var walk func(*sitter.Node)

	walk = func(n *sitter.Node) {
		if opaque != nil && opaque[n.StartByte()] {
			return
		}

		if isCallKind(n.Kind()) {
			if m := methodOf(n, src); m != "" {
				out = append(out, Call{Line: uint32(n.StartPosition().Row) + rowOffset, Method: m})
			}
		}

		for i := range int(n.NamedChildCount()) {
			walk(n.NamedChild(uint(i)))
		}
	}

	walk(n)

	return out
}

// isCallKind reports whether a node is a call under some name.
//
// `queryExecute(...)` is not a call_expression: the grammar gives it a node of
// its own so the SQL grammar can be injected into its first argument. Comparing
// on call_expression alone therefore reported every queryExecute as a call the
// parser had invented — the oracle's mistake, not the parser's, and the reason
// a differential check has to be calibrated against the grammar's vocabulary
// before its output means anything.
func isCallKind(kind string) bool {
	return kind == "call_expression" || kind == "query_expression"
}

// methodOf names what a call is to: the property of a member expression, the
// identifier of a bare call, or — for a query_expression — the keyword the
// grammar consumed before its own children start.
func methodOf(n *sitter.Node, src []byte) string {
	if n.Kind() == "query_expression" {
		// Its first child is the unnamed keyword itself.
		if c := n.Child(0); c != nil {
			return strings.ToLower(text(c, src))
		}

		return ""
	}

	fn := n.NamedChild(0)
	if fn == nil {
		return ""
	}

	switch fn.Kind() {
	case "identifier":
		return strings.ToLower(text(fn, src))
	case "member_expression":
		last := fn.NamedChild(fn.NamedChildCount() - 1)
		if last == nil {
			return ""
		}

		return strings.ToLower(text(last, src))
	default:
		return ""
	}
}

func text(n *sitter.Node, src []byte) string {
	return string(src[n.StartByte():n.EndByte()])
}

// grammarDecls returns the names the grammar treats as declared: a variable
// declarator, an assignment's left-hand side, and a function's name.
//
// This is the second axis of the comparison and the weaker one — see
// TestDeclarationAxisIsWeakerThanTheCallAxis for what it can and cannot settle.
func grammarDecls(src []byte) []string {
	var out []string

	visit := func(root *sitter.Node, buf []byte) {
		var walk func(*sitter.Node)

		walk = func(n *sitter.Node) {
			switch n.Kind() {
			case "variable_declarator", "function_declaration":
				if id := n.NamedChild(0); id != nil && strings.Contains(id.Kind(), "identifier") {
					out = append(out, strings.ToLower(text(id, buf)))
				}
			case "assignment_expression":
				if lhs := n.NamedChild(0); lhs != nil {
					out = append(out, strings.ToLower(text(lhs, buf)))
				}
			}

			for i := range int(n.NamedChildCount()) {
				walk(n.NamedChild(uint(i)))
			}
		}

		walk(root)
	}

	tree := language.Parse(language.CFML, src, nil)
	if tree == nil {
		return nil
	}

	defer tree.Close()

	for _, m := range language.FindInjections(tree, src) {
		g, ok := grammarFor(m.Language)
		if !ok {
			continue
		}

		inner := src[m.Node.StartByte():m.Node.EndByte()]

		sub := language.Parse(g, inner, nil)
		if sub == nil {
			continue
		}

		visit(sub.RootNode(), inner)
		sub.Close()
	}

	sort.Strings(out)

	return out
}
