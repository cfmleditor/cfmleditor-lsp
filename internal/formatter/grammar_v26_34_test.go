package formatter

import (
	"strings"
	"testing"
)

// Grammar v0.26.34 made a batch of cfscript constructs parse that previously
// produced an ERROR node. Anything newly visible to the CST walk is rendered by
// the formatter for the first time, and two of them were rendered wrong — the
// same failure mode v0.26.33 brought with thin-arrow lambdas and `new java:`.
//
// Neither corrupts a file in the default configuration, because the
// whitespaceOnly guard catches both. That is the point: the guard turns them
// into a silent refusal to format, so a file containing one simply stops
// responding to format-on-save with nothing reported.
func TestV2634BracelessTryBodyIsPreserved(t *testing.T) {
	t.Parallel()

	// The body is an expression_statement rather than a statement_block.
	// scriptBlock wrapped it in braces the source never had, and dropped the
	// trailing semicolon with it: `;` is an anonymous child, so the
	// named-children walk inside the block never emitted it.
	src := "<cfscript>\ntry writeOutput(1); catch (any e) {}\n</cfscript>"

	out := format(t, src)

	assertContains(t, out, "try writeOutput(1); catch (any e) {")

	if err := checkWhitespaceOnly([]byte(src), []byte(out), false, false); err != nil {
		t.Errorf("brace-less try body is not whitespace-preserving: %v", err)
	}
}

func TestV2634BracedTryStillFormatsAsABlock(t *testing.T) {
	t.Parallel()

	// The brace-less fix must not change the ordinary braced form, which is
	// still re-indented as a block.
	out := format(t, "<cfscript>\ntry { writeOutput(1); } catch (any e) {}\n</cfscript>")

	assertContains(t, out, "try {")
	assertContains(t, out, "writeOutput(1);")
	assertContains(t, out, "} catch (any e) {")
}

func TestV2634ParenthesisedComponentAttrsArePreserved(t *testing.T) {
	t.Parallel()

	// The parens are anonymous children, so the attribute walk dropped them and
	// re-emitted the header unparenthesised.
	src := "<cfscript>\ncomponent (extends=\"A\") { }\n</cfscript>"

	out := format(t, src)

	assertContains(t, out, `component (extends="A") {`)

	if err := checkWhitespaceOnly([]byte(src), []byte(out), false, false); err != nil {
		t.Errorf("parenthesised component attributes are not whitespace-preserving: %v", err)
	}
}

func TestV2634UnparenthesisedComponentAttrsUnchanged(t *testing.T) {
	t.Parallel()

	// The paren fix must not add parens to a header that never had them.
	out := format(t, "<cfscript>\ncomponent extends=\"A\" { }\n</cfscript>")

	assertContains(t, out, `component extends="A" {`)

	if strings.Contains(out, "component (") {
		t.Errorf("parens invented for an unparenthesised header:\n%s", out)
	}
}
