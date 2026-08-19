package formatter

import "testing"

// TestGuardTreatsBreakingUnicodeSpacesAsWhitespace covers source indented with
// something other than tabs and spaces. Text pasted from a word processor or a
// browser arrives carrying U+2003 EM SPACE and friends; re-indenting such a
// line is a whitespace change, but measuring whitespace one byte at a time made
// it look like the formatter had deleted content and the file was refused.
func TestGuardTreatsBreakingUnicodeSpacesAsWhitespace(t *testing.T) {
	spaces := map[string]string{
		"en quad":                   " ",
		"em space":                  " ",
		"hair space":                " ",
		"ideographic space":         "　",
		"ogham space mark":          " ",
		"medium mathematical space": " ",
		"line separator":            " ",
	}

	for name, sp := range spaces {
		t.Run(name, func(t *testing.T) {
			src := "<cfset a = 1>" + sp + sp + "<cfset b = 2>"
			out := "<cfset a = 1 />\n\t<cfset b = 2 />"

			if err := checkWhitespaceOnly([]byte(src), []byte(out), true); err != nil {
				t.Errorf("guard rejected re-indentation of a breaking space: %v", err)
			}
		})
	}
}

// TestGuardKeepsNonBreakingSpacesAsContent pins the other side of the line.
// U+00A0 and U+202F render differently from an ordinary space and are
// load-bearing in markup, so dropping one is a real change and must be
// reported rather than waved through as whitespace.
func TestGuardKeepsNonBreakingSpacesAsContent(t *testing.T) {
	for name, sp := range map[string]string{
		"no-break space":        " ",
		"narrow no-break space": " ",
	} {
		t.Run(name, func(t *testing.T) {
			src := `<cfset x = "a` + sp + `b" />`
			out := `<cfset x = "ab" />`

			if err := checkWhitespaceOnly([]byte(src), []byte(out), true); err == nil {
				t.Error("guard accepted the loss of a non-breaking space")
			}
		})
	}
}
