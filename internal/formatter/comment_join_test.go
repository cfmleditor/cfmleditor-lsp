package formatter

import (
	"strings"
	"testing"
)

// TestLineCommentDoesNotSwallowCode covers the defect this file exists for. A
// "//" comment runs to the end of its line, so any pass that folds the next
// line up onto it turns that code into part of the comment. Because joining
// lines only removes whitespace, the damage is invisible to a character-level
// comparison of the output — it has to be prevented rather than detected.
func TestLineCommentDoesNotSwallowCode(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{
			// A condition long enough to be reflowed, with a comment on each line.
			"long condition reflowed",
			"<cfscript>\n" +
				"if ( originalInterface // do not exclude any method from this\n" +
				"\tor not StructKeyExists( curMethod.metadata, 'access' ) // no access attribute\n" +
				"\tor curMethod.metadata.access neq 'private' ) {\n" +
				"\tf();\n}\n</cfscript>",
			[]string{
				"or not StructKeyExists(curMethod.metadata, 'access')",
				"or curMethod.metadata.access neq 'private'",
			},
		},
		{
			"comment between condition and an Allman brace",
			"<cfscript>\nif (n eq 1)  // p ends with .cfc\n{\n\tp = 1;\n}\n</cfscript>",
			[]string{"// p ends with .cfc", "p = 1;"},
		},
		{
			"comment between else and its braced body",
			"<cfscript>\nif (x) {\n\tg();\n}\nelse //we have summary\n{\n\th();\n}\n</cfscript>",
			[]string{"//we have summary", "h();"},
		},
		{
			"comment between else and a single-statement body",
			"<cfscript>\nif (x) {\n\tg();\n}\nelse\n\t// 1 is in 2\n\ty = 2;\n</cfscript>",
			[]string{"// 1 is in 2", "y = 2;"},
		},
		{
			// The same shape as the first case but joined with `&&`, which the
			// grammar gives a different tree: the comments between the operands
			// are neither the left nor the right field of the binary_expression
			// holding them, so rebuilding the condition from those fields
			// dropped all but the last, and that survivor ended up in front of
			// the closing paren. tree-sitter-cfml v0.26.35 is the first release
			// to parse testbox's BaseSpec.cfc, where this was found.
			"condition joined with && keeps every comment",
			"<cfscript>\n" +
				"if (\n" +
				"\t!arguments.spec.skip && // Not skipping\n" +
				"\tisSpecFocused( arguments.suite.name ) && // Is the spec focused\n" +
				"\tcanRunLabel( labels ) // In label list\n" +
				") {\n\tdoIt();\n}\n</cfscript>",
			[]string{
				"// Not skipping",
				"// Is the spec focused",
				"// In label list",
				"doIt();",
			},
		},
		{
			"comment parked at the end of a condition",
			"<cfscript>\nif ( a EQ 1\n\tAND b EQ 2\n\t/* TODO: AND (c EQ 3) */\n) {\n\tf();\n}\n</cfscript>",
			[]string{"/* TODO: AND (c EQ 3) */", "f();"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := format(t, tc.src)

			for _, want := range tc.want {
				if !strings.Contains(out, want) {
					t.Errorf("%q missing from output — swallowed or dropped\ngot:\n%s", want, out)
				}
			}

			// Nothing may end up commented out that was not before.
			for _, line := range strings.Split(out, "\n") {
				if i := strings.Index(line, "//"); i >= 0 {
					if strings.Contains(line[i:], " or ") || strings.Contains(line[i:], "{") {
						t.Errorf("code folded into a line comment: %q", line)
					}
				}
			}
		})
	}
}

// TestConditionWithLineCommentsFormatsUnderTheGuard is the same defect seen from
// the entry point that matters: with the whitespace-only guard on, as the LSP
// runs it, a dropped comment is a refusal — so format-on-save silently did
// nothing to any file with a `&&` condition commented operand by operand.
func TestConditionWithLineCommentsFormatsUnderTheGuard(t *testing.T) {
	src := "<cfscript>\n" +
		"if (\n" +
		"\t!arguments.spec.skip && // Not skipping\n" +
		"\tisSpecFocused( arguments.suite.name ) && // Is the spec focused\n" +
		"\tcanRunLabel( labels ) // In label or no labels\n" +
		") {\n\tdoIt();\n}\n</cfscript>\n"

	out := formatGuarded(t, src)

	for _, want := range []string{"// Not skipping", "// Is the spec focused", "// In label or no labels"} {
		assertContains(t, out, want)
	}

	assertReparses(t, out)
}
