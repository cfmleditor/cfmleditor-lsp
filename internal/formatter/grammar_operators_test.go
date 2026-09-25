package formatter

import (
	"testing"
)

// The tree-sitter-cfml release after v0.26.36 changes how CFML's operators
// appear in the tree, in three ways the formatter reads:
//
//   - `!` and `NOT` are a not_expression, with a not_operator, rather than a
//     unary_expression. They bind looser than the comparisons in CFML, so they
//     needed a precedence of their own.
//   - `IS NOT` is two tokens, `is` and `not`, both under the operator field. It
//     was one, so that `a is nothing` stopped reading as `a IS NOT hing`.
//   - every word operator — `AND`, `EQ`, `CONTAINS`, `DOES NOT CONTAIN` — is a
//     visible token. They were hidden, so a binary_expression holding one had
//     an empty operator field, and gapOperator was the only path they took.
//
// Each test here passes on v0.26.36 as well, so they can land ahead of the bump.

// TestNotExpressionIsFormattedLikeAUnary covers the node rename. Unknown to
// expr, a not_expression fell through to its default and was emitted verbatim,
// so its operand was never formatted.
func TestNotExpressionIsFormattedLikeAUnary(t *testing.T) {
	t.Parallel()

	src := "<cfscript>\nx = ! foo( a,b );\ny = NOT foo( a,b );\n</cfscript>\n"

	out := formatGuarded(t, src)

	assertContains(t, out, "x = !foo(a, b);")
	assertContains(t, out, "y = NOT foo(a, b);")
	assertReparses(t, out)
}

// TestMixedCaseNotKeepsItsSpace covers isWordOp, which matched `not` and `NOT`
// only. `Not c` was joined into `Notc`, another identifier, and the guard could
// not see it: the two differ only in whitespace. This was wrong on v0.26.36 too
// — the tag form is from Slatwall's admin views, three of which v0.3.5 rewrote
// this way, along with two Mura components — and the grammar now accepts every
// casing of the keyword.
func TestMixedCaseNotKeepsItsSpace(t *testing.T) {
	t.Parallel()

	src := "<cfscript>\ny = Not c;\nz = nOT d;\n</cfscript>\n" +
		"<cfif Not rc.addressZone.isNew()>\n</cfif>\n"

	out := formatGuarded(t, src)

	assertContains(t, out, "y = Not c;")
	assertContains(t, out, "z = nOT d;")
	assertContains(t, out, "<cfif Not rc.addressZone.isNew()>")
	assertNotContains(t, out, "Notc")
	assertNotContains(t, out, "Notrc")
	assertReparses(t, out)
}

// TestIsNotKeepsBothWords covers the two-token `IS NOT`. ChildByFieldName
// returns only the first child in a field, which was `is` — so `a IS NOT b`
// came out as `a IS b` and the guard refused the file.
func TestIsNotKeepsBothWords(t *testing.T) {
	t.Parallel()

	src := "<cfscript>\nx = a IS NOT b;\n</cfscript>\n<cfif a IS NOT b>\n</cfif>\n"

	out := formatGuarded(t, src)

	assertContains(t, out, "x = a IS NOT b;")
	assertContains(t, out, "<cfif a IS NOT b>")
	assertReparses(t, out)
}

// TestWordOperatorKeepsACommentBeforeIt covers the change of path. A visible
// word operator would take the one symbolic operators take, which puts a block
// comment after the operator wherever it was written.
func TestWordOperatorKeepsACommentBeforeIt(t *testing.T) {
	t.Parallel()

	src := "<cfscript>\nx = a /* why */ AND b;\n</cfscript>\n"

	out := formatGuarded(t, src)

	assertContains(t, out, "x = a /* why */ AND b;")
	assertReparses(t, out)
}
