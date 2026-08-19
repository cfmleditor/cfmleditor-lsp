package formatter

import (
	"strings"
	"testing"
)

// TestGuardAllowsNormalizationInsertions covers the canonicalisation the
// formatter performs deliberately: optional semicolons and braces around
// single-statement bodies. These are insertions on the output side only.
func TestGuardAllowsNormalizationInsertions(t *testing.T) {
	cases := []struct {
		name string
		src  string
		out  string
	}{
		{"semicolon added", "var x = 1", "var x = 1;"},
		{"braces added", "if (x) return 1;", "if (x) { return 1; }"},
		{"both", "if (x) return 1", "if (x) {\n\treturn 1;\n}"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkWhitespaceOnly([]byte(tc.src), []byte(tc.out), true, true); err != nil {
				t.Errorf("guard rejected a deliberate normalisation: %v", err)
			}
		})
	}
}

// TestGuardStillCatchesRealChanges pins down what the widened allowance must
// not swallow. Every one of these is a defect the guard found in real code.
func TestGuardStillCatchesRealChanges(t *testing.T) {
	cases := []struct {
		name string
		src  string
		out  string
	}{
		{"dropped return type", "public query function f() {}", "public function f() {}"},
		{"dropped catch type", "catch (any e) { x(); }", "catch (e) { x(); }"},
		{"dropped catch clause", "catch (A e) { y(); } catch (B e) { z(); }", "catch (A e) { y(); }"},
		{"interface rewritten", "interface { }", "component { }"},
		{"static access rewritten", "Widget::getData()", "Widget.getData()"},
		{"BOM dropped", "\ufeffcomponent {}", "component {}"},
		{"invented closing tag", "<cfmodule template=\"a\">", "<cfmodule template=\"a\"></cfmodule>"},
		{"comment swallowed in literal", "[\n// note\na, b]", "[// note, a, b]"},
		{"dropped semicolon", "var x = 1;", "var x = 1"},
		{"dropped brace", "if (x) { return 1; }", "if (x) return 1;"},
		{"unmatched brace added", "f()", "f() }"},
		{"identifier changed", "var total = 1", "var totl = 1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkWhitespaceOnly([]byte(tc.src), []byte(tc.out), true, true); err == nil {
				t.Errorf("guard accepted a real change: %q -> %q", tc.src, tc.out)
			}
		})
	}
}

// TestGuardBraceBalance checks the counting half of the allowance: a matched
// pair is fine, a stray brace is not.
func TestGuardBraceBalance(t *testing.T) {
	if err := checkWhitespaceOnly([]byte("if (x) f();"), []byte("if (x) { f(); }"), true, true); err != nil {
		t.Errorf("matched pair rejected: %v", err)
	}

	err := checkWhitespaceOnly([]byte("if (x) f();"), []byte("if (x) { f();"), true, true)
	if err == nil {
		t.Error("unmatched opening brace accepted")
	} else if !strings.Contains(err.Error(), "unmatched") {
		t.Errorf("expected an unmatched-brace error, got %v", err)
	}
}

// TestGuardCatchesCommentSwallowingCode covers the defect the comment handling
// exists for: a "//" comment runs to the end of its line, so deleting that
// newline pulls the code that followed into the comment. Joining lines removes
// only whitespace, which is exactly why a plain character walk cannot see it.
func TestGuardCatchesCommentSwallowingCode(t *testing.T) {
	src := "<cfscript>\nif ( a // one\n\tor b ) { f(); }\n</cfscript>"
	out := "<cfscript>\nif ( a // one or b ) { f(); }\n</cfscript>"

	if err := checkWhitespaceOnly([]byte(src), []byte(out), true, true); err == nil {
		t.Error("guard accepted a line comment that swallowed the rest of the condition")
	}
}

// TestGuardCatchesDroppedComment pins the other half: a comment the formatter
// removed outright is a silent loss of what the code says about itself.
func TestGuardCatchesDroppedComment(t *testing.T) {
	src := `<cfmail to="a" <!--- server="x" ---> from="b">`
	out := `<cfmail to="a" from="b">`

	if err := checkWhitespaceOnly([]byte(src), []byte(out), true, true); err == nil {
		t.Error("guard accepted a dropped comment")
	}
}

// TestGuardAllowsCommentMovement covers what the formatter legitimately does to
// comments: reindenting them, rewrapping them, and lifting a trailing comment
// onto its own line. None of these change what the comment says or what the
// code does.
func TestGuardAllowsCommentMovement(t *testing.T) {
	cases := []struct {
		name string
		src  string
		out  string
	}{
		{
			"trailing line comment lifted onto its own line",
			"<cfscript>\nif ( a ) { // note\n\tf();\n}\n</cfscript>",
			"<cfscript>\nif ( a ) {\n\t// note\n\tf();\n}\n</cfscript>",
		},
		{
			"tag comment reindented",
			"<cfif x>\n<!---  spaced   out  --->\n</cfif>",
			"<cfif x>\n\t<!--- spaced out --->\n</cfif>",
		},
		{
			"commented-out markup gains a self-closing slash",
			`<cfif x><!--- <cfargument name="a"> ---></cfif>`,
			`<cfif x><!--- <cfargument name="a" /> ---></cfif>`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkWhitespaceOnly([]byte(tc.src), []byte(tc.out), true, true); err != nil {
				t.Errorf("guard rejected a legitimate comment move: %v", err)
			}
		})
	}
}

// TestGuardTreatsSlashesInMarkupAsContent covers the false positives that a
// naive "//" rule produces. Outside script these are ordinary characters, and
// reading one as a comment would swallow its line and reject a good reformat.
func TestGuardTreatsSlashesInMarkupAsContent(t *testing.T) {
	cases := []struct {
		name string
		src  string
		out  string
	}{
		{
			"doctype in a string literal",
			`<cfset x = '<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0//EN">'><cfset y = 1>`,
			"<cfset x = '<!DOCTYPE html PUBLIC \"-//W3C//DTD XHTML 1.0//EN\">' />\n<cfset y = 1 />",
		},
		{
			"slashes inside an HTML comment",
			`<!-- // end-of-template --><cfset y = 1>`,
			"<!-- // end-of-template -->\n<cfset y = 1 />",
		},
		{
			"url in markup",
			`<a href="http://example.com/a">x</a><cfset y = 1>`,
			"<a href=\"http://example.com/a\">x</a>\n<cfset y = 1 />",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkWhitespaceOnly([]byte(tc.src), []byte(tc.out), true, true); err != nil {
				t.Errorf("guard rejected markup containing slashes: %v", err)
			}
		})
	}
}

// TestScriptSyntaxComponentDetected checks that a script-syntax component,
// which has no <cfscript> tag to key off, still counts as script throughout —
// including when a doc block precedes the keyword.
func TestScriptSyntaxComponentDetected(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want bool
	}{
		{"plain", "component { }", true},
		{"after doc block", "/**\n * docs\n */\ncomponent { }", true},
		{"interface", "interface { }", true},
		{"modifier", "final component { }", true},
		{"tag based", "<cfcomponent></cfcomponent>", false},
		{"not a keyword prefix", "componentry = 1", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isScriptSyntaxComponent([]byte(tc.src)); got != tc.want {
				t.Errorf("isScriptSyntaxComponent(%q) = %v, want %v", tc.src, got, tc.want)
			}
		})
	}
}

// TestGuardRejectsDroppedQuotes covers the hole recorded as 3.2 in
// FORMATTER-ISSUES.md. The re-quoting allowance was written as "any mismatched
// quote on either side", which also excused a quote the formatter *dropped* —
// so with the default options the guard could not see the formatter stripping
// the quotes off a CFML string or a SQL literal. Neither is a shape
// normaliseAttrValue produces, and neither appears anywhere in the 5,620-file
// corpus, so the allowance was pure blind spot.
func TestGuardRejectsDroppedQuotes(t *testing.T) {
	cases := []struct {
		name string
		src  string
		out  string
	}{
		{
			name: "cfset string literal loses its quotes",
			src:  `<cfset msg = "hello world">`,
			out:  `<cfset msg = hello world>`,
		},
		{
			name: "sql string literal loses its quotes",
			src:  `<cfquery>SELECT 'a' FROM t</cfquery>`,
			out:  `<cfquery>SELECT a FROM t</cfquery>`,
		},
		{
			name: "attribute value loses its quotes",
			src:  `<cfinclude template="foo.cfm">`,
			out:  `<cfinclude template=foo.cfm>`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkWhitespaceOnly([]byte(tc.src), []byte(tc.out), true, true); err == nil {
				t.Errorf("guard accepted a dropped quote:\n src: %s\n out: %s", tc.src, tc.out)
			}
		})
	}
}

// TestGuardAllowsAttributeRequoting is the other half: the two shapes
// normaliseAttrValue really does produce must still pass, or the formatter
// would be in conflict with its own defaults.
func TestGuardAllowsAttributeRequoting(t *testing.T) {
	cases := []struct {
		name string
		src  string
		out  string
	}{
		{
			name: "unquoted value gains quotes",
			src:  `<cfinclude template=foo.cfm>`,
			out:  `<cfinclude template="foo.cfm">`,
		},
		{
			name: "single quoted value upgraded to double",
			src:  `<cfquery name='q' datasource='ds'>x</cfquery>`,
			out:  `<cfquery name="q" datasource="ds">x</cfquery>`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkWhitespaceOnly([]byte(tc.src), []byte(tc.out), true, true); err != nil {
				t.Errorf("guard rejected a deliberate re-quoting: %v\n src: %s\n out: %s", err, tc.src, tc.out)
			}
		})
	}
}

// TestGuardRequoteGatedOnItsOwnOption pins the knob. The allowance used to ride
// on selfCloseTags, which has nothing to do with quoting: turning self-closing
// tags on disabled quote checking for the whole file. It is gated on
// doubleQuoteAttributes now — the option that actually performs the re-quoting.
func TestGuardRequoteGatedOnItsOwnOption(t *testing.T) {
	src := `<cfinclude template=foo.cfm>`
	out := `<cfinclude template="foo.cfm">`

	if err := checkWhitespaceOnly([]byte(src), []byte(out), false, true); err != nil {
		t.Errorf("re-quoting should be allowed with selfCloseTags off: %v", err)
	}

	if err := checkWhitespaceOnly([]byte(src), []byte(out), true, false); err == nil {
		t.Error("re-quoting should be reported with doubleQuoteAttributes off")
	}
}
