package parser

import (
	"strings"
	"testing"
)

// TestACommentBetweenTagAttributesDoesNotEndTheTag: a `>` inside a CFML comment
// between a tag's attributes is not the tag's end, so what the comment holds is
// not parsed as live tags.
func TestACommentBetweenTagAttributesDoesNotEndTheTag(t *testing.T) {
	src := `<cfcomponent>
<cffunction name="real" <!--- <cfset p = getThing()><cfset p.gone()> ---> output="false">
	<cfset keep.me()>
	<cfreturn 1>
</cffunction>
<cfset after.me()>
</cfcomponent>`

	pr := ParseWithOptions(testURI, src, &ParseOptions{ExtractCalls: true})

	var names []string
	for _, c := range pr.AllCalls() {
		names = append(names, strings.ToLower(c.FuncName))
	}

	got := strings.Join(names, ",")
	for _, bad := range []string{"gone", "getthing"} {
		if strings.Contains(got, bad) {
			t.Errorf("recorded %q from inside a comment: %s", bad, got)
		}
	}

	for _, want := range []string{"me"} {
		if !strings.Contains(got, want) {
			t.Errorf("lost %q: %s", want, got)
		}
	}

	if len(pr.Funcs) != 1 || pr.Funcs[0].Name != "real" {
		t.Errorf("funcs = %+v", pr.Funcs)
	}
}

// TestTagEndSkipsComments pins the helper itself, quotes and comments mixed.
func TestTagEndSkipsComments(t *testing.T) {
	for _, tc := range []struct{ name, in string }{
		{"comment", `<cfx a="1" <!--- > ---> b="2">rest`},
		{"comment then quote", `<cfx <!--- ">" ---> b=">">rest`},
		{"quote holding a comment opener", `<cfx a="<!---" b="2">rest`},
		{"plain", `<cfx a="1">rest`},
	} {
		got := tagEndIndex(tc.in)
		if got < 0 || tc.in[got:] != ">rest" {
			t.Errorf("%s: tagEndIndex(%q) ends at %d: %q", tc.name, tc.in, got, tc.in[max(got, 0):])
		}
	}
}
