package parser

import (
	"strings"
	"testing"

	"go.lsp.dev/uri"
)

// A reparse records the file's top-level calls afresh. It reset the
// functions, refs and scopes and appended the calls to the last parse's, so
// after an edit outside a function every top-level call was listed once more
// than before — and each copy kept that version of the document alive: on a
// 65,000-line component, ~2.8MB per edit that was never freed. (Calls inside a
// function are dropped by a reparse and read again through FuncCalls; that is
// deliberate, and not what this checks.)
func TestAReparseDoesNotKeepTheLastParsesCalls(t *testing.T) {
	src := "<cfset x = now()>\n<cffunction name=\"f\"><cfset y = dateFormat(x)></cffunction>\n<cfoutput>#ucase(x)#</cfoutput>"

	pr := ParseWithOptions(uri.URI("file:///x/page.cfm"), src, &ParseOptions{ExtractCalls: true})
	want := len(pr.Calls)

	if want != 2 {
		t.Fatalf("the fixture's top-level calls are %v, want now and ucase", pr.Calls)
	}

	for i := range 3 {
		pr.ApplyFullReplace(strings.Repeat(" ", i+1) + src)

		if got := len(pr.Calls); got != want {
			t.Fatalf("after reparse %d: %d top-level calls, want %d (%v)", i+1, got, want, pr.Calls)
		}
	}
}
