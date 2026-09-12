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

// TestScriptComponentDetectedThroughAnImport is the same defect as the BOM
// above, with a different token in the way: `import a.b.C;` is legal between
// the doc block and the component, and the probe stepped over `abstract` and
// `final` but not over that.
//
// It surfaced as a guard rejection in exactly one place — under
// `commaPosition: "before"`, where a comma legitimately moves across a `//`
// comment. With comments recognised the guard skips them and sees no change to
// the code stream; with comment recognition off the comment is code, and a
// comma crossing it reads as a reordering. Five corpus files have an import
// before their component, and on all five the guard ran weakened in every mode.
func TestScriptComponentDetectedThroughAnImport(t *testing.T) {
	t.Parallel()

	body := "component {\n\tfunction f() {\n\t\tg(\n\t\t\t// key\n\t\t\tk,\n\t\t\t// value\n\t\t\tv\n\t\t);\n\t}\n}\n"

	for _, tc := range []struct {
		name string
		src  string
	}{
		{"no import", body},
		{"one import", "import a.b.C;\n\n" + body},
		{"several imports", "import a.b.C;\nimport d.e.F;\n\n" + body},
		{"import with no semicolon", "import a.b.C\n\n" + body},
		{"doc block, then import", "/**\n * why\n */\nimport a.b.C;\n\n" + body},
		{"BOM, doc block, then import", "\ufeff/**\n * why\n */\nimport a.b.C;\n\n" + body},
		{"import then final", "import a.b.C;\n\nfinal " + body},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if !isScriptSyntaxComponent([]byte(tc.src)) {
				t.Fatal("script-syntax component not recognised")
			}

			if n := len(scriptRegionsOf([]byte(tc.src))); n == 0 {
				t.Fatal("file has no script region, so `//` is not recognised anywhere in it")
			}

			// Leading commas move a comma across each `//` comment, which is
			// what the weakened guard read as a reordering.
			opts := testOpts()
			opts.WhitespaceOnly = true
			opts.CommaPosition = "before"

			if _, err := Format([]byte(tc.src), parse(t, tc.src), opts); err != nil {
				t.Errorf("guard refused a correct format: %v", err)
			}
		})
	}
}

// TestImportWithoutAComponentIsNotScript is the boundary. The loop steps over
// imports to find what follows them; a file of imports and nothing else is not
// a script component, and must not be claimed as one.
func TestImportWithoutAComponentIsNotScript(t *testing.T) {
	t.Parallel()

	for _, src := range []string{
		"import a.b.C;\n",
		"import a.b.C;\nimport d.e.F;\n",
		"import a.b.C;\n<cfoutput>hi</cfoutput>\n",
		// Never terminates: no semicolon, no newline, end of file. importEnd
		// reports that rather than guessing, and the probe declines.
		"import a.b.C",
	} {
		if isScriptSyntaxComponent([]byte(src)) {
			t.Errorf("claimed as a script component: %q", src)
		}
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
