package parser

import (
	"strings"
	"testing"

	"go.lsp.dev/uri"
)

// A <cfset> that assigns a member and makes calls is read by one sub-parse,
// which supplies both the member assignment and the calls. Each call on the
// line is recorded as often as it is written, and none names the wrapper the
// sub-parse reads it in as its caller.
func TestACfsetMemberAssignmentRecordsEachCallOnce(t *testing.T) {
	src := `<cfcomponent>
<cffunction name="report">
	<cfset VARIABLES.today = DateFormat(Now(), "yyyy-mm-dd")>
	<cfset VARIABLES.announce = Replace(VARIABLES.announce, Chr(13), " ", "ALL")>
</cffunction>
</cfcomponent>`

	pr := ParseWithOptions(uri.URI("file:///x/Report.cfc"), src, &ParseOptions{
		ExtractCalls: true,
		FuncLookup:   func(string, string) string { return "" },
		Resolvers:    []Resolver{{Match: "unrelated.$1", Resolve: "x.$1", Prefix: "unrelated"}},
	})

	count := map[string]int{}

	for _, c := range pr.AllCalls() {
		count[strings.ToLower(c.FuncName)]++

		if c.Caller != "report" {
			t.Errorf("%s on line %d: caller %q, want report", c.FuncName, c.Line, c.Caller)
		}
	}

	for _, name := range []string{"dateformat", "now", "replace", "chr"} {
		if count[name] != 1 {
			t.Errorf("%s recorded %d times, want 1 (all: %v)", name, count[name], count)
		}
	}
}
