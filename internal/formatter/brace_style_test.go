package formatter

import (
	"strings"
	"testing"
)

func formatWithBraces(t *testing.T, src, style string) string {
	t.Helper()

	opts := testOpts()
	opts.WhitespaceOnly = true
	opts.BraceStyle = style

	out, err := Format([]byte(src), parse(t, src), opts)
	if err != nil {
		t.Fatalf("format error (braceStyle=%q): %v", style, err)
	}

	return string(out)
}

const braceSrc = "<cfscript>\n" +
	"component extends=\"Base\" {\n" +
	"\tpublic function go(required numeric id) {\n" +
	"\t\tif (id > 1) {\n" +
	"\t\t\ta();\n" +
	"\t\t} else if (id == 0) {\n" +
	"\t\t\tb();\n" +
	"\t\t} else {\n" +
	"\t\t\tc();\n" +
	"\t\t}\n" +
	"\t\ttry {\n" +
	"\t\t\tdoIt();\n" +
	"\t\t} catch (any e) {\n" +
	"\t\t\trethrow;\n" +
	"\t\t} finally {\n" +
	"\t\t\tcleanup();\n" +
	"\t\t}\n" +
	"\t\tswitch (id) {\n" +
	"\t\t\tcase 1:\n" +
	"\t\t\t\tbreak;\n" +
	"\t\t}\n" +
	"\t\tfor (var i = 1; i <= 3; i++) d(i);\n" +
	"\t\twhile (false) { e(); }\n" +
	"\t}\n" +
	"}\n" +
	"</cfscript>\n"

// TestBraceStyleUnsetIsSameLine is the constraint the setting has to satisfy
// before it is worth having: a project that never names braceStyle keeps the
// K&R output the formatter has always emitted, byte for byte, and nothing it
// has already formatted moves.
func TestBraceStyleUnsetIsSameLine(t *testing.T) {
	t.Parallel()

	out := formatWithBraces(t, braceSrc, "")

	for _, want := range []string{
		"component extends=\"Base\" {",
		") {",
		"} else if ( id == 0 ) {",
		"} else {",
		"} catch (any e) {",
		"} finally {",
		"switch ( id ) {",
		"while ( false ) {",
	} {
		assertContains(t, out, want)
	}
}

// TestBraceStyleSameLineMatchesUnset pins the explicit spelling to the default,
// so a project can write the setting down without its files changing.
func TestBraceStyleSameLineMatchesUnset(t *testing.T) {
	t.Parallel()

	if got, want := formatWithBraces(t, braceSrc, "same-line"), formatWithBraces(t, braceSrc, ""); got != want {
		t.Errorf("same-line differs from unset:\n--- unset\n%s\n--- same-line\n%s", want, got)
	}
}

// TestBraceStyleNextLineMovesTheBrace covers every construct whose brace the
// setting reaches, and the clause keywords that have to move with it: leaving
// `else` attached to the closing brace would put the next brace after
// `} else`, which is neither style.
func TestBraceStyleNextLineMovesTheBrace(t *testing.T) {
	t.Parallel()

	out := formatWithBraces(t, braceSrc, "next-line")

	for _, unwanted := range []string{
		"extends=\"Base\" {",
		") {",
		"} else",
		"} catch",
		"} finally",
		"switch ( id ) {",
		"while ( false ) {",
	} {
		assertNotContains(t, out, unwanted)
	}

	// Every brace-opening line is the brace and nothing else, at the indent of
	// the header above it.
	for _, want := range []string{
		"\n    component extends=\"Base\"\n    {",
		"\n        )\n        {",
		"\n            }\n            else if ( id == 0 )\n            {",
		"\n            }\n            else\n            {",
		"\n            try\n            {",
		"\n            }\n            catch (any e)\n            {",
		"\n            }\n            finally\n            {",
		"\n            switch ( id )\n            {",
		"\n            while ( false )\n            {",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("next-line output missing %q:\n%s", want, out)
		}
	}

	assertReparses(t, out)
}

// TestBraceStyleIsIdempotent is the failure mode this kind of change actually
// has: the first pass moves a brace, the second sees the new line break in the
// source and pads it again, and an untouched file produces a fresh diff on
// every save.
func TestBraceStyleIsIdempotent(t *testing.T) {
	t.Parallel()

	for _, style := range []string{"", "same-line", "next-line"} {
		once := formatWithBraces(t, braceSrc, style)
		if twice := formatWithBraces(t, once, style); twice != once {
			t.Errorf("braceStyle=%q not idempotent:\n--- pass 1\n%s\n--- pass 2\n%s", style, once, twice)
		}
	}
}

// TestBraceStyleLeavesABareBlockAlone covers Lucee's one-word `elseif`, as
// spelled in its own Query.cfc. The grammar has no such keyword, so it reads
// `elseif ( … )` as an ordinary call and the block that follows is left
// standing as a statement of its own, attached to nothing.
//
// braceStyle must not reach a block like that: "next-line" means "under the
// header", and there is no header. Moving it anyway made formatting
// non-idempotent — the newline the brace gained on the first pass became a
// real source line break, which the second pass padded again.
func TestBraceStyleLeavesABareBlockAlone(t *testing.T) {
	t.Parallel()

	src := "<cfscript>\ncomponent {\n\tfunction f() {\n\t\tif ( a )\n\t\t{\n\t\t\tx();\n\t\t}\n\t\t// why\n\t\telseif ( b )\n\t\t{\n\t\t\ty();\n\t\t}\n\t}\n}\n</cfscript>\n"

	for _, style := range []string{"", "next-line"} {
		once := formatWithBraces(t, src, style)
		if twice := formatWithBraces(t, once, style); twice != once {
			t.Errorf("bare block, braceStyle=%q, not idempotent:\n--- pass 1\n%s\n--- pass 2\n%s", style, once, twice)
		}
	}
}

// TestBraceStyleNextLineKeepsAnAnnotatedSignatureBrace is the other half of
// that boundary. A signature ending in a `//` annotation comment already puts
// the brace on a line of its own, because a comment would otherwise swallow it.
// next-line has nothing left to do there, and adding its newline anyway would
// leave a blank line between the signature and the brace.
func TestBraceStyleNextLineKeepsAnAnnotatedSignatureBrace(t *testing.T) {
	t.Parallel()

	src := "<cfscript>\ncomponent {\n" +
		"\tfunction withFilters( event )\n" +
		"\t\tcache =\"true\" // cache it\n" +
		"\t{\n\t\tparam rc.slug = \"\";\n\t}\n}\n</cfscript>\n"

	out := formatWithBraces(t, src, "next-line")

	assertContains(t, out, "// cache it\n        {")
	assertNotContains(t, out, "// cache it\n\n")
	assertReparses(t, out)
}
