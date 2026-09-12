package formatter

import (
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/language"
)

// TestMalformedShapeCatchesFoldedBraces feeds malformedShape the exact output
// the braced-`case` defect produced. That defect was whitespace-only and
// idempotent, so the guard passed it and the corpus counted the file clean
// across every run recorded in FORMATTER-ISSUES.md — this check is the only
// thing in the harness that can see it, and this is the case it exists for.
func TestMalformedShapeCatchesFoldedBraces(t *testing.T) {
	t.Parallel()

	before := "component {\n\tfunction sw(k) {\n\t\tswitch ( k ) {\n\t\tcase 1:\n {\n\n\t\t\t\ta();\n\n\t\t\t}}\n\n\t}\n}\n"

	got := malformedShape([]byte(before), corpusOptions())
	if got == "" {
		t.Fatal("malformedShape passed the output the braced-case defect produced")
	}

	if !strings.Contains(got, "closing braces share a line") {
		t.Errorf("reported the wrong defect: %q", got)
	}
}

// TestMalformedShapeCatchesALostIndent is the second rule, on the shape that
// found it: everything after a multi-line string argument came out in column
// one, the opening brace of a struct literal included.
func TestMalformedShapeCatchesALostIndent(t *testing.T) {
	t.Parallel()

	src := "component {\n\tfunction f() {\n\t\tq = queryExecute(\n\" select 1\nfrom t\",\n{\na: 1\n}\n);\n\t}\n}\n"

	got := malformedShape([]byte(src), corpusOptions())
	if !strings.Contains(got, "column one") {
		t.Errorf("a block brace in column one was not reported: %q", got)
	}
}

// TestMalformedShapeAcceptsHealthyOutput is the half that decides whether
// anyone reads the report. Both rules were picked by measuring candidates over
// the corpus and keeping only those that accused no healthy file; these are the
// shapes that made four other candidates unusable.
func TestMalformedShapeAcceptsHealthyOutput(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"formatted switch with a braced case": mustFormat(t,
			"<cfscript>\ncomponent {\n\tfunction sw(k) {\n\t\tswitch (k) {\n\t\t\tcase 1: {\n\t\t\t\ta();\n\t\t\t}\n\t\t}\n\t}\n}\n</cfscript>\n"),
		"nested struct literal closing":  "component {\n\tfunction f() {\n\t\tx = { a: { b: 1 } };\n\t}\n}\n",
		"braces inside a string literal": "component {\n\tfunction f() {\n\t\ts = \"a\n}}\nb\";\n\t}\n}\n",
		"lone brace inside a string":     "component {\n\tfunction f() {\n\t\ts = \"a\n{\nb\";\n\t}\n}\n",
		"brace on the very first line":   "{\n\tx = 1;\n}\n",
	}

	for name, src := range cases {
		if got := malformedShape([]byte(src), corpusOptions()); got != "" {
			t.Errorf("%s: healthy output reported as malformed: %s\n%s", name, got, src)
		}
	}
}

// TestMalformedShapeAllowsNextLineBraces is the boundary the column-one rule
// needs, and it was not obvious: Allman puts every opening brace on a line of
// its own, so a top-level `component` has one in column zero by design. The
// rule accused 3,563 of the corpus's 5,624 files under braceStyle "next-line"
// before it was scoped — found by the first sweep CFML_CORPUS_OPTS made cheap
// enough to run.
//
// The closing-brace rule holds in both styles and is still checked here.
func TestMalformedShapeAllowsNextLineBraces(t *testing.T) {
	t.Parallel()

	opts := corpusOptions()
	opts.BraceStyle = "next-line"

	src := mustFormatWith(t, "<cfscript>\ncomponent {\n\tfunction f() {\n\t\tx = 1;\n\t}\n}\n</cfscript>\n", opts)

	if got := malformedShape([]byte(src), opts); got != "" {
		t.Errorf("next-line output reported as malformed: %s\n%s", got, src)
	}

	if got := malformedShape([]byte("component\n{\n\tf();\n\t}}\n"), opts); got == "" {
		t.Error("the closing-brace rule stopped applying under next-line braces")
	}
}

func mustFormat(t *testing.T, src string) string {
	t.Helper()

	return mustFormatWith(t, src, corpusOptions())
}

func mustFormatWith(t *testing.T, src string, opts Options) string {
	t.Helper()

	tree := language.Parse(language.CFML, []byte(src), nil)
	defer tree.Close()

	out, err := Format([]byte(src), tree, opts)
	if err != nil {
		t.Fatalf("format error: %v", err)
	}

	return string(out)
}
