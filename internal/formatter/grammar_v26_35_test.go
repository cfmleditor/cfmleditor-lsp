package formatter

import (
	"strings"
	"testing"
)

// Grammar v0.26.35 made another batch of cfscript constructs parse that
// previously produced an ERROR node, and as with v0.26.33 and v0.26.34 the
// release turns each one from a safe refusal into a formatter defect: a
// construct the grammar starts parsing is one the formatter starts rendering,
// whether or not it knows how.
//
// Each case below deleted something. None of them corrupts a file in the
// default configuration, because the whitespaceOnly guard catches every one —
// which is exactly the failure mode worth naming, since the user sees
// format-on-save do nothing at all rather than an error.

// TestV2635StaticSubscriptKeepsItsAccessor covers `Test::["f"]()`, the
// subscripted form of `Test::f()` (#79). The grammar reports the `::` as a
// named static_chain field on the subscript_expression, exactly as it does on a
// member_expression, and the renderer built the node from object and index
// alone — so the accessor was dropped and a static call became an instance call
// on a value that has no such member.
func TestV2635StaticSubscriptKeepsItsAccessor(t *testing.T) {
	t.Parallel()

	src := "<cfscript>\nm = \"f\";\nb = Test::[\"f\"]();\nc = Test::[m]();\n</cfscript>\n"

	out := formatGuarded(t, src)

	assertContains(t, out, `Test::["f"]()`)
	assertContains(t, out, "Test::[m]()")
	assertNotContains(t, out, `Test["f"]()`)
	assertReparses(t, out)
}

// TestV2635DotStaticAccessStillRenders is the other direction: the accessor is
// added because the source had one, not because every subscript now gets one.
func TestV2635DotStaticAccessStillRenders(t *testing.T) {
	t.Parallel()

	out := formatGuarded(t, "<cfscript>\nb = arr[\"f\"];\nc = Test::f();\n</cfscript>\n")

	assertContains(t, out, `arr["f"]`)
	assertContains(t, out, "Test::f()")
	assertNotContains(t, out, `arr::["f"]`)
}

// TestV2635TagFormThrowKeepsEveryAttribute covers the tag form of throw written
// in script, whose attributes are space-separated rather than a comma-separated
// argument list. Each is its own parameter_attribute child, and every rendering
// path read NamedChild(0) alone — so a throw with more than one attribute lost
// all but the first, deleting the `type` a catch block dispatches on.
func TestV2635TagFormThrowKeepsEveryAttribute(t *testing.T) {
	t.Parallel()

	src := "<cfscript>\ntry {\n\tthrow message=\"Access Denied\" type=\"MyCustomError\";\n} catch (any e) {\n\twriteoutput(e.message);\n}\n</cfscript>\n"

	out := formatGuarded(t, src)

	assertContains(t, out, `message="Access Denied"`)
	assertContains(t, out, `type="MyCustomError"`)
	assertReparses(t, out)
}

// TestV2635ThrowCallFormIsUnchanged pins the call form against the fix above,
// which must not reroute it: `throw(...)` is an ordinary argument list and is
// still spaced like every other call's named arguments.
func TestV2635ThrowCallFormIsUnchanged(t *testing.T) {
	t.Parallel()

	out := format(t, "<cfscript>\nthrow(type=\"x\", message=\"y\");\n</cfscript>\n")

	assertContains(t, out, `throw(type = "x", message = "y");`)
}

// TestV2635ParenthesisedComponentAttributesKeepCommas covers
// `component( output=false, javasettings={...} )`. The parenthesised form
// separates its attributes with commas while the bare form separates them with
// spaces; the commas are anonymous children, so joining everything with a space
// dropped them.
func TestV2635ParenthesisedComponentAttributesKeepCommas(t *testing.T) {
	t.Parallel()

	src := "<cfscript>\ncomponent( output=false, javasettings = { maven = [\"a:b:1.0\"] } ) {\n\tpublic function test() {\n\t\treturn \"x\";\n\t}\n}\n</cfscript>\n"

	out := formatGuarded(t, src)

	assertContains(t, out, "output=false,")
	assertContains(t, out, `maven = ["a:b:1.0"]`)
	assertReparses(t, out)
}

// TestV2635BareComponentAttributesStaySpaceSeparated is the boundary: the comma
// is reproduced because the source wrote one, so the bare form must not gain
// separators it never had.
func TestV2635BareComponentAttributesStaySpaceSeparated(t *testing.T) {
	t.Parallel()

	out := formatGuarded(t, "<cfscript>\ncomponent output=false extends=\"Base\" {\n\tfunction f() {}\n}\n</cfscript>\n")

	assertContains(t, out, `output=false extends="Base"`)
	assertNotContains(t, out, "output=false,")
}

// TestV2635FunctionExpressionKeepsAnnotations covers
// `describe("x", function() labels="query" { … })` — how TestBox and Lucee's own
// suite label a spec. The annotation is a child with no field name, and the
// function-expression renderer built its output from name, parameters and body
// alone, so the annotation was deleted. The declaration path already collected
// these, which is why the same annotation survived on a declaration and vanished
// inside an argument list.
func TestV2635FunctionExpressionKeepsAnnotations(t *testing.T) {
	t.Parallel()

	src := "<cfscript>\ncomponent {\n\tfunction run() {\n\t\tdescribe(\"x\", function() labels=\"query\" {\n\t\t\ty = 1;\n\t\t});\n\t}\n}\n</cfscript>\n"

	out := formatGuarded(t, src)

	assertContains(t, out, `function() labels="query"`)
	assertReparses(t, out)
}

// TestV2635UnannotatedFunctionExpressionGainsNothing is the boundary for the
// case above — an ordinary function expression must not pick up a stray space
// where the annotations would go.
func TestV2635UnannotatedFunctionExpressionGainsNothing(t *testing.T) {
	t.Parallel()

	out := formatGuarded(t, "<cfscript>\nx = describe(\"d\", function() {\n\ty = 1;\n});\n</cfscript>\n")

	assertContains(t, out, "function() {")
	assertNotContains(t, out, "function()  {")
}

// TestV2635TernaryKeepsALineComment covers a `//` comment parked before the `:`
// of a ternary — a common way to say why the alternative is what it is. It
// belongs to none of the condition, consequence or alternative fields the
// renderer reads, so it was dropped outright. This is the same root cause as the
// `&&` condition case in comment_join_test.go, and shares its fix: the rendering
// is checked against the source's own comments and the expression reproduced as
// written when any went missing.
func TestV2635TernaryKeepsALineComment(t *testing.T) {
	t.Parallel()

	src := "<cfscript>\nresults = (1 == 1)\n\t? function (input){\n\t\treturn true;\n\t}\n\t// by setting to an empty string, the standard formatting is applied\n\t: \"\";\n</cfscript>\n"

	out := formatGuarded(t, src)

	assertContains(t, out, "// by setting to an empty string, the standard formatting is applied")
	assertReparses(t, out)
}

// TestV2635OrdinaryTernaryStillCollapses is the boundary: only a ternary that
// would lose a comment is reproduced verbatim; every other one is still
// rendered and re-spaced normally.
func TestV2635OrdinaryTernaryStillCollapses(t *testing.T) {
	t.Parallel()

	out := format(t, "<cfscript>\nx = ( a==1 )\n\t? \"yes\"\n\t: \"no\";\n</cfscript>\n")

	assertContains(t, out, `x = ( a == 1 ) ? "yes" : "no";`)
}

// TestSignatureAnnotationCommentsAreNotFolded covers a function declaration
// whose annotations carry `//` comments — how ColdBox's own test handlers say
// what each cache setting is for. The annotations were joined with a space, so
// the first comment swallowed every annotation after it *and* the brace that
// opens the body, leaving code that no longer parses. The guard rejected it, so
// nothing was written; the file simply stopped responding to format-on-save.
func TestSignatureAnnotationCommentsAreNotFolded(t *testing.T) {
	t.Parallel()

	src := "<cfscript>\ncomponent {\n" +
		"\tfunction withAllFilters( event, rc, prc )\n" +
		"\t\tcache        =\"true\"\n" +
		"\t\tcacheFilter  =\"filterMutateParams\" // mutate params\n" +
		"\t\tcacheInclude =\"slug,id\" // include slug and id\n" +
		"\t{\n\t\tparam rc.slug = \"\";\n\t}\n}\n</cfscript>\n"

	out := formatGuarded(t, src)

	assertContains(t, out, "// mutate params")
	assertContains(t, out, `cacheInclude ="slug,id"`)
	assertContains(t, out, "// include slug and id")
	assertReparses(t, out)

	// Nothing may sit after a line comment on its own line — not the next
	// annotation, and not the brace that opens the body.
	for _, line := range strings.Split(out, "\n") {
		if at := strings.Index(line, "//"); at >= 0 {
			if rest := strings.TrimSpace(line[at:]); strings.HasSuffix(rest, "{") {
				t.Errorf("the body's opening brace was folded into a comment: %q", line)
			}
		}
	}
}

// TestSignatureAnnotationsWithoutCommentsStayOnOneLine is the boundary: only a
// comment forces the break, so an ordinary annotated signature is unchanged.
func TestSignatureAnnotationsWithoutCommentsStayOnOneLine(t *testing.T) {
	t.Parallel()

	out := formatGuarded(t, "<cfscript>\ncomponent {\n\tfunction f() cache=\"true\" cacheTimeout=\"10\" {\n\t\tx = 1;\n\t}\n}\n</cfscript>\n")

	assertContains(t, out, `cache="true" cacheTimeout="10" {`)
}

// TestConditionOperandNotFoldedOntoItsComment is the other half of the check
// that keeps the two cases above honest. A comment can survive a rendering and
// still be broken by it: the operand that followed it in the source gets folded
// up onto its line, where the comment swallows it. Only whitespace changed, so
// a character-level comparison cannot see it — which is why the rendering is
// checked against the source's comments for position as well as presence.
// TestBox's MockBox.cfc, whose nested parenthesised operand folded where a flat
// one did not.
func TestConditionOperandNotFoldedOntoItsComment(t *testing.T) {
	t.Parallel()

	src := "<cfscript>\n" +
		"if (\n" +
		"\tisObject( tree[ arg ] ) and\n" +
		"\t// Find out if object, sometimes of course, on Adobe, is instance does not work\n" +
		"\t(\n" +
		"\t\tisInstanceOf( tree[ arg ], \"Component\" ) OR structKeyExists( getMetadata( tree[ arg ] ), \"extends\" )\n" +
		"\t)\n" +
		") {\n\tf();\n}\n</cfscript>\n"

	out := formatGuarded(t, src)

	for _, line := range strings.Split(out, "\n") {
		at := strings.Index(line, "// Find out if object")
		if at < 0 {
			continue
		}

		if rest := strings.TrimSpace(line[at:]); !strings.HasSuffix(rest, "work") {
			t.Errorf("an operand was folded onto the comment's line: %q", line)
		}
	}

	assertReparses(t, out)
}
