package formatter

import (
	"strings"
	"testing"
)

// TestCollectionCommentGetsNoComma covers a comment interleaved among the
// entries of a struct or array literal. Commas separate elements, and a comment
// is not one — giving it a comma turns it into a list entry. CFML comments
// reach these literals from tag context (`<cfset x = { ... }>`), where they are
// the only comment form available, and they were the case left unhandled.
func TestCollectionCommentGetsNoComma(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		wantNot string
		want    string
	}{
		{
			"struct literal, laid out one per line",
			"<cfset r = {\n\t\"a\": q.a\n\t, \"b\": q.b\n\t<!--- why c --->\n\t, \"c\": q.c\n\t, \"d\": q.d\n}>",
			"<!--- why c --->,",
			"<!--- why c --->",
		},
		{
			"array literal, short enough to stay inline",
			"<cfset a = [\n\t1\n\t, 2\n\t<!--- why --->\n\t, 3\n]>",
			"<!--- why --->,",
			"<!--- why --->",
		},
		{
			"block comment among entries",
			"<cfscript>\nr = {\n\t\"a\": 1\n\t/* why b */\n\t, \"b\": 2\n};\n</cfscript>",
			"*/,",
			"/* why b */",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := format(t, tc.src)

			if !strings.Contains(out, tc.want) {
				t.Fatalf("comment missing from output\ngot:\n%s", out)
			}

			if strings.Contains(out, tc.wantNot) {
				t.Errorf("comment was given a comma, making it a list entry\ngot:\n%s", out)
			}
		})
	}
}

// TestCollectionCommasUnaffectedWithoutComments checks the separator logic is
// unchanged for ordinary literals.
func TestCollectionCommasUnaffectedWithoutComments(t *testing.T) {
	cases := map[string]string{
		`<cfset a = { "x": 1, "y": 2 }>`: `{ "x": 1, "y": 2 }`,
		`<cfset a = [1, 2, 3]>`:          `[1, 2, 3]`,
	}

	for src, want := range cases {
		out := format(t, src)
		if !strings.Contains(out, want) {
			t.Errorf("literal changed\nwant to contain: %s\ngot:\n%s", want, out)
		}
	}
}
