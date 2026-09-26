package formatter

import (
	"strings"
	"testing"
)

// TestEqualsStructLiteralFormatted covers the `=` spelling of a struct literal,
// `{ a = 1 }`. The grammar shares its rule with JavaScript destructuring, so the
// literal is an object_pattern, a bare-name entry is an object_assignment_pattern
// and any other key is a cf_pair. None of the three had a renderer, so every
// `=` struct was written back exactly as it came in — 19,178 of them across
// 2,788 corpus files — while the same struct spelled with `:` was laid out.
func TestEqualsStructLiteralFormatted(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "bare, quoted, path and nested keys",
			src:  "<cfscript>\nx = {a=1, b.c=2, \"d\"=3, 'e'={f=4}};\n</cfscript>\n",
			want: "x = { a = 1, b.c = 2, \"d\" = 3, 'e' = { f = 4 } };",
		},
		{
			name: "multi-line source collapses when it fits",
			src:  "<cfscript>\nx = {'a'=1,\n 'b'={\n 'c'=2\n }};\n</cfscript>\n",
			want: "x = { 'a' = 1, 'b' = { 'c' = 2 } };",
		},
		{
			name: "tag context",
			src:  "<cfset s = {a=1,b=\"x\",c={d=2}}>\n",
			want: "<cfset s = { a = 1, b = \"x\", c = { d = 2 } } />",
		},
		{
			name: "mixed with colon entries",
			src:  "<cfscript>\nm = {a=1, b:2};\n</cfscript>\n",
			want: "m = { a = 1, b: 2 };",
		},
		{
			name: "trailing comma kept",
			src:  "<cfscript>\nt = {a=1,};\n</cfscript>\n",
			want: "t = { a = 1, };",
		},
		{
			name: "function argument",
			src:  "<cfscript>\nf({a=1});\n</cfscript>\n",
			want: "f({ a = 1 });",
		},
		{
			name: "hash key",
			src:  "<cfscript>\nx = {#k#=1};\n</cfscript>\n",
			want: "x = { #k# = 1 };",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := formatGuarded(t, tt.src)
			assertContains(t, out, tt.want)
			assertReparses(t, out)

			if again := formatGuarded(t, out); again != out {
				t.Errorf("not idempotent:\nfirst:\n%s\nsecond:\n%s", out, again)
			}
		})
	}
}

// TestEqualsStructLineComment checks the `=` literal goes through the same
// comment handling as the `:` one: a `//` comment forces one entry per line and
// never takes a comma.
func TestEqualsStructLineComment(t *testing.T) {
	src := "<cfscript>\nw = {\"a\"=1, // note\n b=2};\n</cfscript>\n"
	out := formatGuarded(t, src)

	allIn(t, out, "w = {\n", "\"a\" = 1,\n", "// note\n", "b = 2\n")
	assertNotContains(t, out, "// note,")
	assertReparses(t, out)
}

// TestEqualsStructLeftVerbatim pins the shapes isStructPattern refuses. An
// empty slot between commas parses, but re-joining the entries would drop it;
// a rest entry is destructuring, not a struct literal. Both keep their source
// text rather than being refused by the guard.
func TestEqualsStructLeftVerbatim(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"empty slot", "<cfscript>\nh = {a=1,,b=2};\n</cfscript>\n", "h = {a=1,,b=2};"},
		{"leading comma", "<cfscript>\nh = {,a=1};\n</cfscript>\n", "h = {,a=1};"},
		{"rest entry", "<cfscript>\nr = {...q, a=1};\n</cfscript>\n", "r = {...q, a=1};"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertContains(t, formatGuarded(t, tt.src), tt.want)
		})
	}
}

// TestEqualsStructKeyCase checks a bare `=` key keeps its case under scopeCase.
// It names a key, not a scope: `{ url = x }` is a key called url, and
// upper-casing it changes the key wherever key case is preserved.
func TestEqualsStructKeyCase(t *testing.T) {
	src := "<cfscript>\nx = {url=variables.u, \"form\"=1};\n</cfscript>\n"
	tree := parse(t, src)
	opts := testOpts()
	opts.ScopeCase = "upper"

	out, err := Format([]byte(src), tree, opts)
	if err != nil {
		t.Fatalf("format error: %v", err)
	}

	assertContains(t, string(out), "x = { url = VARIABLES.u, \"form\" = 1 };")
}

// TestNestedMultilineCollectionIndent covers a literal that spans lines inside
// another one. The items were rendered at the outer literal's level, so the
// inner entries came out level with their own key and the inner closing bracket
// level with the outer entries. It predates the `=` renderer — the `:` struct
// and the array had it too — but the `=` renderer made it common.
func TestNestedMultilineCollectionIndent(t *testing.T) {
	long := "firstName%sarguments.firstName, lastName%sarguments.lastName, emailAddress%sarguments.emailAddress"

	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "equals struct in struct",
			src:  "x = {outer={" + strings.ReplaceAll(long, "%s", "=") + "}};",
			want: "    x = {\n" +
				"        outer = {\n" +
				"            firstName = arguments.firstName,\n" +
				"            lastName = arguments.lastName,\n" +
				"            emailAddress = arguments.emailAddress\n" +
				"        }\n" +
				"    };",
		},
		{
			name: "colon struct in struct",
			src:  "x = {outer: {" + strings.ReplaceAll(long, "%s", ": ") + "}};",
			want: "    x = {\n" +
				"        outer: {\n" +
				"            firstName: arguments.firstName,\n" +
				"            lastName: arguments.lastName,\n" +
				"            emailAddress: arguments.emailAddress\n" +
				"        }\n" +
				"    };",
		},
		{
			name: "struct in array",
			src:  "x = [{" + strings.ReplaceAll(long, "%s", ": ") + ", x: 1}];",
			want: "    x = [\n" +
				"        {\n" +
				"            firstName: arguments.firstName,\n" +
				"            lastName: arguments.lastName,\n" +
				"            emailAddress: arguments.emailAddress,\n" +
				"            x: 1\n" +
				"        }\n" +
				"    ];",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := formatGuarded(t, wrap(tt.src))
			assertContains(t, out, tt.want)
			assertReparses(t, out)

			if again := formatGuarded(t, out); again != out {
				t.Errorf("not idempotent:\nfirst:\n%s\nsecond:\n%s", out, again)
			}
		})
	}
}

// TestClosureLastInTagStructStable covers Lucee's test/tags/query/inc.cfm: a
// struct of closures in a <cfset>, spanning lines, whose last closure is
// followed by a newline and the struct's `}`. The cfml grammar ends that
// closure's body after the newline, and the verbatim copy of it added a blank
// line before the `}` on every pass. The `:` spelling had it already; the `=`
// renderer brought the file in reach.
func TestClosureLastInTagStructStable(t *testing.T) {
	for _, sep := range []string{"=", ": "} {
		src := "<cfset local.l={\n\t\tbefore" + sep + "function (a) {\n\t\t\treturn a;\n\t\t}\n\t\t,after" + sep +
			"function (b) {\n\t\t\t//return b;\n\t\t}\n\t}>\n"

		out := formatGuarded(t, src)
		assertNotContains(t, out, "\n\n} />")
		assertReparses(t, out)

		if again := formatGuarded(t, out); again != out {
			t.Errorf("not idempotent (%q):\nfirst:\n%s\nsecond:\n%s", sep, out, again)
		}
	}
}
