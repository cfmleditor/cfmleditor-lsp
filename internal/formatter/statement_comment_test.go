package formatter

import "testing"

// TestCommentInsideStatementIsKept covers a comment that the grammar places
// inside an expression statement or a return, beside the expression rather
// than in it. Both renderers wrote only the expression and the `;`, so the
// comment was deleted and the guard refused the file: lucee-docs'
// Application.cfc (`variables.assetBundleVersion = 45 // must match …`, with no
// semicolon) and Slatwall's VendorOrder.cfc (`return getSubtotal() /*+ …*/;`).
// The comment now follows the statement on a line of its own, as a
// declaration's does, and the output formats to itself.
func TestCommentInsideStatementIsKept(t *testing.T) {
	tests := []struct {
		name  string
		src   string
		wants []string
	}{
		{
			name:  "line comment after a statement with no semicolon",
			src:   "<cfscript>\ncomponent {\n\tvariables.v = 45 // must match a\\b\n\n\tthis.cwd = 1\n}\n</cfscript>\n",
			wants: []string{"variables.v = 45;\n", "// must match a\\b\n", "this.cwd = 1;\n"},
		},
		{
			name:  "block comment before a return's semicolon",
			src:   "<cfscript>\ncomponent {\n\tpublic numeric function getTotal() {\n\t\treturn getSubtotal() /*+ getTaxTotal()*/;\n\t}\n}\n</cfscript>\n",
			wants: []string{"return getSubtotal();\n", "/*+ getTaxTotal()*/\n"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := formatGuarded(t, tt.src)

			for _, want := range tt.wants {
				assertContains(t, out, want)
			}

			assertReparses(t, out)

			if again := formatGuarded(t, out); again != out {
				t.Errorf("not idempotent:\nfirst:\n%s\nsecond:\n%s", out, again)
			}
		})
	}
}
