package formatter

import (
	"strings"
	"testing"
)

// formatWithCase formats src with lowercaseTags set either way, guard on.
func formatWithCase(t *testing.T, src string, lowercase bool) string {
	t.Helper()

	tree := parse(t, src)
	opts := testOpts()
	opts.WhitespaceOnly = true
	opts.LowercaseTags = lowercase

	out, err := Format([]byte(src), tree, opts)
	if err != nil {
		t.Fatalf("format error (lowercaseTags=%v): %v", lowercase, err)
	}

	return string(out)
}

// TestLowercaseTagsOffKeepsSourceCasing is the defect. The setting says "do not
// change my tag casing" and the formatter changed it anyway, in two different
// ways depending on how the grammar happened to parse the tag.
//
// A tag the grammar has a node kind for — most of them — was rebuilt from that
// kind, which is lowercase by construction and never consulted the setting at
// all: `<CFOUTPUT>` came back `<cfoutput>`. A tag that fell to the generic path
// did read the source, but rebuilt the name as a hardcoded "cf" plus whatever
// followed, so `<CFDUMP>` came back as the mongrel `cfDUMP`. A third group is
// written from string literals in the emitters (`cfif`, `cfelse`,
// `cfcomponent`, `cfscript`, `cfquery`) and was always lowercase.
func TestLowercaseTagsOffKeepsSourceCasing(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{
			"tag with its own node kind",
			"<CFOUTPUT>x</CFOUTPUT>\n",
			[]string{"<CFOUTPUT>", "</CFOUTPUT>"},
		},
		{
			"self-closing tag through the generic path",
			"<CFDUMP var=\"#a#\">\n",
			[]string{"<CFDUMP "},
		},
		{
			"conditional chain written from literals",
			"<CFIF a>x<CFELSEIF b>y<CFELSE>z</CFIF>\n",
			[]string{"<CFIF ", "<CFELSEIF ", "<CFELSE>", "</CFIF>"},
		},
		{
			"component",
			"<CFCOMPONENT>\n<CFSET this.a = 1>\n</CFCOMPONENT>\n",
			[]string{"<CFCOMPONENT>", "<CFSET ", "</CFCOMPONENT>"},
		},
		{
			"script block",
			"<CFSCRIPT>\nvar x = 1;\n</CFSCRIPT>\n",
			[]string{"<CFSCRIPT>", "</CFSCRIPT>"},
		},
		{
			"query",
			"<CFQUERY name=\"q\" datasource=\"d\">SELECT 1</CFQUERY>\n",
			[]string{"<CFQUERY ", "</CFQUERY>"},
		},
		{
			"mixed case is preserved as written, not normalised",
			"<CfOutput>x</CfOutput>\n",
			[]string{"<CfOutput>", "</CfOutput>"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := formatWithCase(t, tc.src, false)

			for _, want := range tc.want {
				assertContains(t, out, want)
			}

			assertReparses(t, out)
		})
	}
}

// TestLowercaseTagsOnStillLowercases is the default, and the half that must not
// move: every case above, with the setting on, comes back lowercase.
func TestLowercaseTagsOnStillLowercases(t *testing.T) {
	src := "<CFCOMPONENT>\n<CFOUTPUT><CFDUMP var=\"#a#\"><CFIF a>x<CFELSE>y</CFIF></CFOUTPUT>\n" +
		"<CFQUERY name=\"q\" datasource=\"d\">SELECT 1</CFQUERY>\n<CFSCRIPT>var x = 1;</CFSCRIPT>\n</CFCOMPONENT>\n"

	out := formatWithCase(t, src, true)

	for _, unwanted := range []string{"<CF", "</CF", "<Cf", "cfDUMP", "cfIF"} {
		assertNotContains(t, out, unwanted)
	}

	for _, want := range []string{"<cfcomponent>", "<cfoutput>", "<cfdump ", "<cfif ", "<cfelse>", "<cfquery ", "<cfscript>"} {
		assertContains(t, out, want)
	}

	assertReparses(t, out)
}

// TestOpenAndCloseCasingAreIndependent covers a shape CFML permits and the fix
// has to respect: the two halves need not agree, and "leave my casing alone"
// means leaving each as it was written rather than making them match.
func TestOpenAndCloseCasingAreIndependent(t *testing.T) {
	out := formatWithCase(t, "<CFOUTPUT>x</cfoutput>\n", false)

	assertContains(t, out, "<CFOUTPUT>")
	assertContains(t, out, "</cfoutput>")
	assertReparses(t, out)
}

// TestTagNameStaysCanonical pins the identity half of the split. tagName used
// to fold in the setting, so with it off it returned things like "cfARGUMENT",
// and firstBodyChildIsArg's bare comparison against "cfargument" stopped
// matching — a blank-line rule quietly changing with an unrelated casing
// setting. Rendering no longer depends on that string, so only a comparison
// like this one can catch it.
//
// The two formats must differ in nothing but the casing of the tag names, so
// lowercasing both outputs makes them equal.
func TestTagNameStaysCanonical(t *testing.T) {
	upper := "<CFCOMPONENT>\n<CFFUNCTION name=\"f\">\n<CFARGUMENT name=\"a\">\n<CFSET var b = 1>\n</CFFUNCTION>\n</CFCOMPONENT>\n"

	preserved := formatWithCase(t, upper, false)
	lowered := formatWithCase(t, upper, true)

	if strings.ToLower(preserved) != lowered {
		t.Errorf("lowercaseTags changed more than casing:\n--- off (lowercased) ---\n%s\n--- on ---\n%s",
			strings.ToLower(preserved), lowered)
	}

	// Guard the fixture: if this stopped parsing as a function with an
	// argument, the comparison above would pass without exercising anything.
	assertContains(t, preserved, "<CFARGUMENT ")
	assertContains(t, lowered, "<cfargument ")
}

// TestSpelledAsRejectsAMismatch pins the guard that keeps a bad offset from
// emitting a different tag. It can only ever return another casing of the name
// it was given.
func TestSpelledAsRejectsAMismatch(t *testing.T) {
	f := &Formatter{src: []byte("<CFOUTPUT>x</CFOUTPUT>")}
	f.opts.LowercaseTags = false

	if got := f.spelledAs("cfoutput", 0); got != "CFOUTPUT" {
		t.Errorf("spelledAs at the tag = %q, want CFOUTPUT", got)
	}

	for _, tc := range []struct {
		name string
		at   int
	}{
		{"offset is not a <", 3},
		{"offset past the end", 999},
		{"negative offset", -1},
	} {
		if got := f.spelledAs("cfoutput", tc.at); got != "cfoutput" {
			t.Errorf("%s: spelledAs = %q, want the canonical name back", tc.name, got)
		}
	}

	// A name that does not case-fold to the canonical one is discarded.
	other := &Formatter{src: []byte("<CFQUERY>")}
	other.opts.LowercaseTags = false

	if got := other.spelledAs("cfoutput", 0); got != "cfoutput" {
		t.Errorf("spelledAs returned a different tag: %q", got)
	}
}

// TestLowercaseTagsOffIsIdempotent is the property a formatter change most often
// breaks: formatting the output again must not move it.
func TestLowercaseTagsOffIsIdempotent(t *testing.T) {
	src := "<CFOUTPUT>\n<CFDUMP var=\"#a#\">\n<CFIF a>x<CFELSE>y</CFIF>\n</cfoutput>\n"

	once := formatWithCase(t, src, false)
	if twice := formatWithCase(t, once, false); once != twice {
		t.Errorf("not idempotent:\n--- once ---\n%s\n--- twice ---\n%s", once, twice)
	}

	if strings.Contains(once, "cfOUTPUT") || strings.Contains(once, "cfDUMP") {
		t.Errorf("mongrel casing in output:\n%s", once)
	}
}
