package formatter

import (
	"strings"
	"testing"
)

// Everything the guard does with a `//` comment depends on knowing which parts
// of the file are script — `//` is a comment there and ordinary content in
// markup. A script-syntax .cfc has no <cfscript> tag to key off, so
// scriptRegionsOf asks isScriptSyntaxComponent instead, and that answer decides
// comment handling for the whole file. Two ways of writing a perfectly ordinary
// .cfc defeated it.

// TestScriptComponentDetectedThroughABOM covers a leading UTF-8 BOM. It is not
// whitespace, so the probe stopped on it and the keyword check then failed on a
// file that is plainly a script component. With no script region the guard
// stopped recognising `//` anywhere in the file and compared comment text as
// though it were code — which is how a leading-comma struct with a
// commented-out entry came back as a non-whitespace change, reported against
// the entry before it. 554 files in the corpus carry a BOM.
func TestScriptComponentDetectedThroughABOM(t *testing.T) {
	t.Parallel()

	body := "component {\n\tthis.cache = {\n\t\tclass: 'a'\n\t\t, bundleName: 'b'\n\t\t//, bundleVersion: '3.2.2'\n\t\t, storage: true\n\t};\n}\n"

	for _, tc := range []struct {
		name string
		src  string
	}{
		{"without a BOM", body},
		{"with a BOM", "\ufeff" + body},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if !isScriptSyntaxComponent([]byte(tc.src)) {
				t.Fatal("script-syntax component not recognised")
			}

			out := formatGuarded(t, tc.src)

			assertContains(t, out, "//, bundleVersion: '3.2.2'")
			assertReparses(t, out)
		})
	}
}

// TestBOMSurvivesFormatting keeps 2.3's fix intact through the change above:
// the BOM is stepped over when deciding what the file is, not dropped from it.
func TestBOMSurvivesFormatting(t *testing.T) {
	t.Parallel()

	out := formatGuarded(t, "\ufeffcomponent {\n\tfunction f() {}\n}\n")

	if !strings.HasPrefix(out, "\ufeff") {
		t.Errorf("BOM lost: %q", out[:min(12, len(out))])
	}
}

// TestLineCommentEndsAtACarriageReturn covers classic Mac line endings — a bare
// "\r" with no "\n" anywhere. The line-comment scan looked for "\n" alone, so
// the first `//` ran to end of file and collected every remaining line as its
// body. TestBox ships a fixture written this way, and it only surfaced once the
// BOM fix above gave the file a script region to be scanned in.
func TestLineCommentEndsAtACarriageReturn(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		nl   string
	}{
		{"CR only", "\r"},
		{"CRLF", "\r\n"},
		{"LF", "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			lines := []string{"component {", "\t// a note", "\tvar x = 1;", "\tvar y = 2;", "}", ""}
			src := strings.Join(lines, tc.nl)

			var body []byte

			script := scriptSpans{{0, len(src)}}
			// Start the walk at the comment itself.
			at := strings.Index(src, "//")
			skipWSAndComments([]byte(src), at, &body, script, nil)

			if got := string(body); got != "anote" {
				t.Errorf("line comment body ran past its line: got %q, want %q", got, "anote")
			}
		})
	}
}

// TestCarriageReturnFileFormatsCleanly is the same defect from the entry point
// that matters: with the comment running to end of file, everything after it
// was compared as comment text and the format was refused.
func TestCarriageReturnFileFormatsCleanly(t *testing.T) {
	t.Parallel()

	src := strings.Join([]string{
		"component {",
		"\t// a note",
		"\tvar x = 1;",
		"\tvar y = 2;",
		"}",
		"",
	}, "\r")

	out := formatGuarded(t, src)

	assertContains(t, out, "// a note")
	assertContains(t, out, "var y = 2;")
}
