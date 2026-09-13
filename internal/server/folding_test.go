package server

import (
	"fmt"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
)

// foldSet renders the folds as "start-end" strings, plus the kind when set, so
// a failure prints something readable rather than a slice of structs.
func foldSet(folds []protocol.FoldingRange) []string {
	out := make([]string, 0, len(folds))
	for _, f := range folds {
		s := fmt.Sprintf("%d-%d", f.StartLine, f.EndLine)
		if f.Kind != "" {
			s += " " + string(f.Kind)
		}

		out = append(out, s)
	}

	return out
}

func assertFolds(t *testing.T, src string, want []string) {
	t.Helper()

	got := foldSet(foldingRanges(src))
	if strings.Join(got, " | ") != strings.Join(want, " | ") {
		t.Errorf("folds:\n got %v\nwant %v", got, want)
	}
}

const tagSrc = `<!--- a comment
      spanning lines --->
<cfoutput>
	<cfif x GT 1>
		<p>
			hello
		</p>
	<cfelse>
		bye
	</cfif>
</cfoutput>
<cfscript>
	function greet( a, b ) {
		if ( a ) {
			writeOutput( b );
		}
	}
</cfscript>
`

// TestFoldingRangesInATagDocument. Every range stops one line short of its
// closing delimiter, so folding leaves `</cfif>` and `}` on screen — a fold
// that hides them reads as though the construct had been deleted.
//
// Two of these are the cases the naive rule got wrong. `<cfelse>` (7-8) has no
// closing token of its own: it ends in the whitespace before the enclosing
// `</cfif>`, which belongs to the `<cfif>`. And the cfscript function (12-15)
// keeps its `}` only because the closing token is found by descending to the
// deepest last token — it is the statement_block's child, not the function
// declaration's.
func TestFoldingRangesInATagDocument(t *testing.T) {
	assertFolds(t, tagSrc, []string{
		"0-1 comment", // the <!--- ---> block
		"2-9",         // <cfoutput>, leaving </cfoutput> shown
		"3-8",         // <cfif>, leaving </cfif> shown
		"4-5",         // <p>
		"7-8",         // the <cfelse> branch
		"11-16",       // <cfscript>, leaving </cfscript> shown
		"12-15",       // function greet, from the injected sub-parse
		"13-14",       // if ( a )
	})
}

const scriptComponentSrc = `/**
 * Docs
 */
component accessors="true" {

	property name="id";

	function init( required string a ) {
		if ( a EQ "x" ) {
			for ( var i = 1; i <= 3; i++ ) {
				writeOutput( i );
			}
		}

		return this;
	}

	private void function helper() {
		// one liner
	}
}
`

// TestFoldingRangesInAScriptComponent is the case that makes this more than a
// tree walk: the CFML grammar hands a script-syntax .cfc's whole body to the
// CFScript grammar as one opaque node, so without following the injection the
// only fold in the file would be the file itself.
func TestFoldingRangesInAScriptComponent(t *testing.T) {
	assertFolds(t, scriptComponentSrc, []string{
		"0-2 comment", // the /** */ doc block
		"3-19",        // component { }
		"7-14",        // function init
		"8-11",        // if
		"9-10",        // for
		"17-18",       // function helper
	})
}

// TestFoldingRangesSkipTheWholeFileWrapper pins the absence of a fold rather
// than its presence. The CFML grammar wraps a script .cfc in a component_file
// spanning the entire document; folding that collapses the file to one line and
// hides everything, which is not a fold anyone wants offered.
func TestFoldingRangesSkipTheWholeFileWrapper(t *testing.T) {
	lastLine := uint32(strings.Count(scriptComponentSrc, "\n"))

	for _, f := range foldingRanges(scriptComponentSrc) {
		if f.StartLine == 0 && f.EndLine >= lastLine-1 {
			t.Errorf("a fold covers the whole document: %d-%d", f.StartLine, f.EndLine)
		}
	}
}

// TestFoldingRangesIgnoreUnstructuredText: a run of prose is not a construct,
// and offering a fold arrow for every paragraph of template text is noise. A
// comment is the deliberate exception — it has no inner structure either, and
// is exactly the thing a reader collapses.
func TestFoldingRangesIgnoreUnstructuredText(t *testing.T) {
	// The nested <b> is what makes this discriminate. Without it the element
	// holds one text run covering the same lines, and the duplicate fold is
	// removed by dedupeFolds whether or not text nodes are skipped — the first
	// version of this test passed with the rule deleted. Split in two by the
	// <b>, the runs fold to 0-2 and 3-5, neither of which the element produces.
	assertFolds(t, "<div>\n\tone\n\ttwo\n\t<b>x</b>\n\tthree\n\tfour\n</div>\n", []string{"0-5"})

	assertFolds(t, "<!--- just\na comment --->\n", []string{"0-1 comment"})
}

// TestFoldingRangesNeedTwoLines: a construct written on one line has nothing to
// hide, and a client is entitled to reject a range whose end is not past its
// start.
func TestFoldingRangesNeedTwoLines(t *testing.T) {
	assertFolds(t, "<cfoutput>hello</cfoutput>\n", nil)
	assertFolds(t, "component { function f() {} }\n", nil)
}

// TestFoldingRangesOnAnUnparseableDocument: folding is decoration, so a file
// mid-edit must degrade to fewer ranges rather than an error. Unlike
// formatting, there is nothing here that could damage the source.
func TestFoldingRangesOnAnUnparseableDocument(t *testing.T) {
	if got := foldingRanges("<cfoutput>\n\t<cfif unclosed\n"); got == nil {
		t.Error("got a nil slice for an unparseable document, want an empty one")
	}
}
