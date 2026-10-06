package parser

import (
	"strings"
	"testing"

	"go.lsp.dev/uri"
)

// A call in the arguments of a scoped assignment is recorded once. With a
// componentResolver configured, the top-level path first looks ahead for a
// chain to extend, and that look ahead walked the argument list — recording
// Now() — before rewinding the scanner and walking it again. Inside a function,
// or with no resolvers, it was already recorded once.
func TestALookAheadThatRewindsRecordsNothing(t *testing.T) {
	resolvers := []Resolver{{Match: "unrelated.$1", Resolve: "x.$1", Prefix: "unrelated"}}

	for name, src := range map[string]string{
		"script component level": "component {\n variables.today = DateFormat(Now(), \"y\");\n}",
		"script in a function":   "component {\n function f() { variables.today = DateFormat(Now(), \"y\"); }\n}",
		"cfscript in a page":     "<cfscript>\nvariables.today = DateFormat(Now(), \"y\");\n</cfscript>",
		"cfset":                  "<cfset variables.today = DateFormat(Now(), \"y\")>",
	} {
		pr := ParseWithOptions(uri.URI("file:///x/a.cfc"), src, &ParseOptions{ExtractCalls: true, Resolvers: resolvers})

		count := map[string]int{}
		for _, c := range pr.AllCalls() {
			count[strings.ToLower(c.FuncName)]++
		}

		if count["now"] != 1 || count["dateformat"] != 1 {
			t.Errorf("%s: calls %v, want DateFormat and Now once each", name, count)
		}
	}
}
