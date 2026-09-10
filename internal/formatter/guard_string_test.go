package formatter

import "testing"

// The guard steps over comments rather than comparing them character by
// character, because a comment's extent is what carries meaning
// (skipWSAndComments). That makes "where does a comment start" load-bearing: a
// string literal may contain the characters that open one, and CFML is full of
// globs that do — `"/#dir#/**"`, `"#target#/*.zip"`.
//
// Reading such a glob as a comment open does not blind the guard. The swallowed
// code is still compared, as comment text, and that comparison is the stricter
// of the two: it folds whitespace and case just as the main loop does, but has
// none of the main loop's allowances for the canonicalisation the formatter
// performs on purpose. So the failure is a refusal of correct output, reported
// against a comment far from the string that caused it.

// TestGuardAcceptsNormalizationAfterAGlobString is the defect. `.run()` gaining
// its optional semicolon is a deliberate insertion the guard allows everywhere
// else; after a glob string it landed inside a supposed comment body, where no
// such allowance exists, and the format was refused.
func TestGuardAcceptsNormalizationAfterAGlobString(t *testing.T) {
	t.Parallel()

	src := "<cfscript>\n" +
		"zip( path = \"/#libBuildDir#/**\" );\n" +
		"command( \"run\" )\n" +
		"\t.run()\n" +
		"/* trailing block comment */\n" +
		"</cfscript>\n"

	out := "<cfscript>\n" +
		"\tzip( path = \"/#libBuildDir#/**\" );\n" +
		"\tcommand( \"run\" ).run();\n" +
		"\t/* trailing block comment */\n" +
		"</cfscript>\n"

	if err := checkWhitespaceOnly([]byte(src), []byte(out), true, true); err != nil {
		t.Errorf("guard refused a deliberate semicolon insertion because an earlier glob string was read as a comment open: %v", err)
	}
}

// TestGuardStillRejectsRealChangesAfterAGlobString is the other direction, and
// the one that matters most: making the guard string-aware must not let a real
// change through in the region it used to swallow.
func TestGuardStillRejectsRealChangesAfterAGlobString(t *testing.T) {
	t.Parallel()

	src := "<cfscript>\n" +
		"zip( path = \"/#libBuildDir#/**\" );\n" +
		"doSomethingImportant();\n" +
		"/* trailing block comment */\n" +
		"</cfscript>\n"

	cases := []struct {
		name string
		out  string
	}{
		{
			"statement deleted",
			"<cfscript>\nzip( path = \"/#libBuildDir#/**\" );\n/* trailing block comment */\n</cfscript>\n",
		},
		{
			"statement renamed",
			"<cfscript>\nzip( path = \"/#libBuildDir#/**\" );\ndoSomethingElse();\n/* trailing block comment */\n</cfscript>\n",
		},
		{
			"the glob itself rewritten",
			"<cfscript>\nzip( path = \"/#libBuildDir#/*\" );\ndoSomethingImportant();\n/* trailing block comment */\n</cfscript>\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := checkWhitespaceOnly([]byte(src), []byte(tc.out), true, true); err == nil {
				t.Error("guard accepted a non-whitespace change after a glob string")
			}
		})
	}
}

// TestGuardStillSkipsRealComments pins the thing the fix must not break: a
// comment that genuinely is one is still stepped over, so the formatter may
// move and re-indent it.
func TestGuardStillSkipsRealComments(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		src  string
		out  string
	}{
		{
			"block comment reindented",
			"<cfscript>\n/* note */\nvar x = 1;\n</cfscript>",
			"<cfscript>\n\t/* note */\n\tvar x = 1;\n</cfscript>",
		},
		{
			"line comment reindented",
			"<cfscript>\n// note\nvar x = 1;\n</cfscript>",
			"<cfscript>\n\t\t// note\n\t\tvar x = 1;\n</cfscript>",
		},
		{
			"comment following an ordinary string",
			"<cfscript>\nvar p = \"plain\";\n/* note */\nvar x = 1;\n</cfscript>",
			"<cfscript>\n\tvar p = \"plain\";\n\t/* note */\n\tvar x = 1;\n</cfscript>",
		},
		{
			"comment following a glob string",
			"<cfscript>\nvar p = \"/#d#/**\";\n/* note */\nvar x = 1;\n</cfscript>",
			"<cfscript>\n\tvar p = \"/#d#/**\";\n\t/* note */\n\tvar x = 1;\n</cfscript>",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := checkWhitespaceOnly([]byte(tc.src), []byte(tc.out), true, true); err != nil {
				t.Errorf("guard rejected a whitespace-only change: %v", err)
			}
		})
	}
}

// TestGuardStillSeesRewrittenComments keeps 3.1's protection intact through the
// change: a comment following a glob string is still compared, not skipped.
func TestGuardStillSeesRewrittenComments(t *testing.T) {
	t.Parallel()

	src := "<cfscript>\nvar p = \"#target#/*.zip\";\n// the original note\nrun();\n</cfscript>\n"
	out := "<cfscript>\nvar p = \"#target#/*.zip\";\n// a completely different note\nrun();\n</cfscript>\n"

	if err := checkWhitespaceOnly([]byte(src), []byte(out), true, true); err == nil {
		t.Error("guard accepted rewritten comment text following a glob string")
	}
}

// TestGuardHandlesQuotesInsideStrings covers the two ways CFML puts a quote in
// a string literal. Ending the literal early would leave its remainder looking
// like code — and a `/*` in that remainder would reopen the defect above.
func TestGuardHandlesQuotesInsideStrings(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		src  string
		out  string
	}{
		{
			"doubled quote",
			"<cfscript>\nvar s = \"say \"\"hi\"\" /**\";\nvar n = 1\n/* end */\n</cfscript>",
			"<cfscript>\n\tvar s = \"say \"\"hi\"\" /**\";\n\tvar n = 1;\n\t/* end */\n</cfscript>",
		},
		{
			"single-quoted glob",
			"<cfscript>\nvar s = '/#dir#/**';\nvar n = 1\n/* end */\n</cfscript>",
			"<cfscript>\n\tvar s = '/#dir#/**';\n\tvar n = 1;\n\t/* end */\n</cfscript>",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := checkWhitespaceOnly([]byte(tc.src), []byte(tc.out), true, true); err != nil {
				t.Errorf("guard refused a semicolon insertion after a %s: %v", tc.name, err)
			}
		})
	}
}

// TestStringSpansIgnoreQuotesInComments is the ordering constraint between the
// two: an apostrophe inside a comment is prose, not the start of a literal.
// Treating it as one would open a "string" running to the next quote anywhere
// in the file and suppress comment detection for everything in between.
func TestStringSpansIgnoreQuotesInComments(t *testing.T) {
	t.Parallel()

	src := "<cfscript>\n// it's fine\n/* don't worry */\nvar x = 1;\n</cfscript>"
	out := "<cfscript>\n\t// it's fine\n\t/* don't worry */\n\tvar x = 1;\n</cfscript>"

	if err := checkWhitespaceOnly([]byte(src), []byte(out), true, true); err != nil {
		t.Errorf("guard rejected a whitespace-only change around apostrophes in comments: %v", err)
	}

	damaged := "<cfscript>\n\t// it's fine\n\t/* don't worry */\n</cfscript>"
	if err := checkWhitespaceOnly([]byte(src), []byte(damaged), true, true); err == nil {
		t.Error("guard accepted a deleted statement after comments containing apostrophes")
	}
}

// TestEndOfStringHandlesCFMLLiterals pins the scanner behind the string spans
// directly. The two defects below only reproduce end-to-end in files of several
// thousand lines — where the runaway span eventually swallows a comment the
// formatter moves — so they are pinned at the level where the mistake is made.
func TestEndOfStringHandlesCFMLLiterals(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			// A quote is escaped by doubling it, and nothing else.
			"plain literal",
			`"abc" rest`,
			`"abc"`,
		},
		{
			"doubled quote",
			`"say ""hi""" rest`,
			`"say ""hi"""`,
		},
		{
			// CFML has no backslash escape: `"\"` is a string holding one
			// backslash. Reading `\"` as an escape ran the literal past its own
			// closing quote and on to the next one — ContentBox's
			// `replace( inPath, "\", "/", "all" )`, and every Windows path
			// written `"C:\dir\"`.
			"backslash is an ordinary character",
			`"\", "/" rest`,
			`"\"`,
		},
		{
			"windows path ending in a separator",
			`"C:\dir\" rest`,
			`"C:\dir\"`,
		},
		{
			// An interpolation may hold strings of its own, in either style.
			// Lucee's admin nests three double-quoted strings one level down.
			"interpolation holding nested strings",
			`"timezone:'#replace(ds.tz,"'","''","all")#' // default" rest`,
			`"timezone:'#replace(ds.tz,"'","''","all")#' // default"`,
		},
		{
			// "##" is a literal hash, not an empty interpolation; reading it as
			// one leaves the scan hunting for a close through the whole file.
			"doubled hash",
			`"a ## b" rest`,
			`"a ## b"`,
		},
		{
			"unterminated literal stops at end of input",
			`"no close`,
			`"no close`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := tc.src[:endOfString([]byte(tc.src), 0)]
			if got != tc.want {
				t.Errorf("endOfString(%q) = %q, want %q", tc.src, got, tc.want)
			}
		})
	}
}
