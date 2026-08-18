package formatter

import (
	"strings"
	"testing"
)

// indentOf returns the leading whitespace of the first line containing sub.
func indentOf(t *testing.T, out, sub string) string {
	t.Helper()

	for line := range strings.SplitSeq(out, "\n") {
		if strings.Contains(line, sub) {
			return line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		}
	}

	t.Fatalf("substring %q not found in output:\n%s", sub, out)

	return ""
}

// TestCFCatchAlignsWithCFTry checks that <cfcatch>/<cffinally> are emitted at
// the indentation of their enclosing <cftry> rather than inside its body.
func TestCFCatchAlignsWithCFTry(t *testing.T) {
	src := "<cfif a>\n<cftry>\n<cfdump var=\"#a#\" />\n<cfcatch type=\"any\">\n<cfrethrow />\n</cfcatch>\n<cffinally>\n<cfset x = 1 />\n</cffinally>\n</cftry>\n</cfif>\n"

	out := format(t, src)

	tryIndent := indentOf(t, out, "<cftry>")
	for _, tag := range []string{"<cfcatch", "</cfcatch>", "<cffinally>", "</cffinally>", "</cftry>"} {
		if got := indentOf(t, out, tag); got != tryIndent {
			t.Errorf("%s indent = %q, want %q (aligned with <cftry>)\n%s", tag, got, tryIndent, out)
		}
	}

	// The catch body still indents one level in from the catch tag itself.
	if got, want := indentOf(t, out, "<cfrethrow"), tryIndent+"    "; got != want {
		t.Errorf("cfcatch body indent = %q, want %q\n%s", got, want, out)
	}
}

// TestEmptyCFBlockHasNoBlankLines covers an empty block emitting the body
// padding blank line from both ends, leaving two blank lines around nothing.
func TestEmptyCFBlockHasNoBlankLines(t *testing.T) {
	src := "<cftry>\n<cfdump var=\"#a#\" />\n<cfcatch type=\"any\">\n</cfcatch>\n</cftry>\n"

	out := format(t, src)
	if !strings.Contains(out, "<cfcatch type=\"any\">\n</cfcatch>") {
		t.Errorf("empty cfcatch should have no blank lines in its body\n%s", out)
	}
}

// TestCFTryIdempotent guards against the dedent shifting further on each pass.
func TestCFTryIdempotent(t *testing.T) {
	src := "<cftry>\n<cfdump var=\"#a#\" />\n<cfcatch type=\"any\">\n<cfrethrow />\n</cfcatch>\n</cftry>\n"

	first := format(t, src)
	if second := format(t, first); first != second {
		t.Errorf("formatting not idempotent\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}
