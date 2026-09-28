package server

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
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

	got := foldSet(foldingRanges(parser.Parse("file:///doc.cfm", src), src))
	if strings.Join(got, " | ") != strings.Join(want, " | ") {
		t.Errorf("folds:\n got %v\nwant %v", got, want)
	}
}

const tagSrc = `<!--- a comment
      spanning lines --->
<cfcomponent>
	<!--- one line --->
	<cffunction name="get">
		<cfif x GT 1>
			<cfreturn 1>
		</cfif>
	</cffunction>
	<!-- an html
	     comment -->
	<cfscript>
		function greet( a, b ) {
			/* a script
			   comment */
			writeOutput( b );
		}
	</cfscript>
</cfcomponent>
`

// TestFoldingRangesInATagDocument: every construct folds to the line before
// its closing one, so `</cffunction>`, `</cfif>` and `}` stay on screen — a
// fold that hides them reads as though the construct had been deleted. A
// comment folds to its last line, which has content of its own, and a comment
// on one line has nothing to fold. The <cfscript> block folds as a tag, and
// the function in it from the script pass.
func TestFoldingRangesInATagDocument(t *testing.T) {
	assertFolds(t, tagSrc, []string{
		"0-1 comment",  // <!--- --->
		"2-17",         // <cfcomponent>
		"4-7",          // <cffunction>, leaving </cffunction> shown
		"5-6",          // <cfif>
		"9-10 comment", // <!-- -->
		"11-16",        // <cfscript>
		"12-15",        // function greet, in the <cfscript> block
		"13-14 comment",
	})
}

const scriptComponentSrc = `/**
 * Docs
 */
component accessors="true" {

	property name="id";

	function init( required string a ) {
		var q = "'#replace( a, "'", "''", "all" )#'";

		/*
		 * after the string
		 */
		return this;
	}

	private void function helper() {
		// one liner
	}

	function oneLine() { return 1; }
}
`

// TestFoldingRangesInAScriptComponent. The string in init is what a plain
// quote-to-quote scan gets wrong: the `#...#` inside it holds strings of its
// own, and pairing the quotes naively leaves the scan inside a string from
// there on, so the comment below it is never found. The comment scan reads
// script with the parse's own scanner for that reason.
func TestFoldingRangesInAScriptComponent(t *testing.T) {
	assertFolds(t, scriptComponentSrc, []string{
		"0-2 comment",   // the /** */ doc block
		"3-20",          // component { }
		"7-13",          // function init
		"10-12 comment", // the comment after the string
		"16-17",         // function helper
	})
}

// TestFoldingRangesNeedTwoLines: a construct written on one line has nothing to
// hide, and a client is entitled to reject a range whose end is not past its
// start. A function on two lines has none either, once its closing line stays
// visible; the four-line component around it does.
func TestFoldingRangesNeedTwoLines(t *testing.T) {
	assertFolds(t, "component { function f() {} }\n", []string{})
	assertFolds(t, "component {\n\tfunction f() {\n\t}\n}\n", []string{"0-2"})
}

// TestFoldingRangesOnAnUnfinishedDocument: folding is decoration, so a file
// mid-edit must degrade to fewer ranges rather than an error, and an empty
// answer is `[]`, not null.
func TestFoldingRangesOnAnUnfinishedDocument(t *testing.T) {
	src := "<cfoutput>\n\t<cfif unclosed\n<!--- open\n"
	if got := foldingRanges(parser.Parse("file:///doc.cfm", src), src); got == nil {
		t.Error("got a nil slice, want an empty one")
	}
}

// TestFoldingFollowsEdits: the functions come from the document's cached
// parse, not a fresh one, so the answer is only right if that parse is kept
// current. A line added above a function at component level is the edit whose
// reparse is deferred; the handler must take the document's lock the way that
// pays it, or it folds the function one line too high.
func TestFoldingFollowsEdits(t *testing.T) {
	srv := newTestServer()
	srv.Features.Folding = true
	docURI := uri.URI("file:///fold.cfc")
	openDoc(t, srv, docURI, "component {\n\tfunction a() {\n\t\tvar x = 1;\n\t\treturn x;\n\t}\n}\n")

	editDoc(t, srv, docURI, protocol.Position{Line: 1}, "\tproperty name=\"p\";\n")

	req := makeCall(t, protocol.MethodTextDocumentFoldingRange, protocol.FoldingRangeParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
	})

	res, err := srv.handleFoldingRange(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	folds, _ := res.([]protocol.FoldingRange)
	// 0-5 is the component; 2-4 the function, one line lower than before
	// the edit. A stale parse adds its scope's 1-3 beside the brace pass's
	// 2-4, which reads the current text.
	if got, want := strings.Join(foldSet(folds), " | "), "0-5 | 2-4"; got != want {
		t.Errorf("folds after the edit: got %q, want %q", got, want)
	}
}
