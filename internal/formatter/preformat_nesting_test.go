package formatter

import (
	"strings"
	"testing"
)

// TestDeeplyNestedVoidElementsConvertInOnePass covers unclosed markup, which the
// grammar nests one element inside the last — a run of unclosed <tr>/<td> ends up
// twenty levels deep. Converting an element used to claim its whole byte range,
// so nothing inside it could be converted on the same pass and each pass
// unwrapped a single level. Past maxPasses the file was left half converted and
// the *next* run of the formatter finished the job, so an unchanged file kept
// producing a fresh diff.
func TestDeeplyNestedVoidElementsConvertInOnePass(t *testing.T) {
	var b strings.Builder
	for i := range 25 {
		b.WriteString("<tr class=\"r\"><td>cell ")
		b.WriteString(string(rune('a' + i%26)))
		b.WriteString("<BR>\n")
	}

	out := format(t, b.String())

	if n := strings.Count(out, "<BR>"); n != 0 {
		t.Errorf("%d <BR> left unconverted after one format", n)
	}

	if next := format(t, out); next != out {
		t.Errorf("not idempotent\n--- first ---\n%s\n--- second ---\n%s", out, next)
	}
}

// TestMalformedTableIsIdempotent is the shape reduced from a real help page:
// unclosed rows and cells, a stray end tag, and void elements at several depths.
func TestMalformedTableIsIdempotent(t *testing.T) {
	src := "\t<tr class=\"tRow4\"><th width=\"80\"><B>Operator</B></a><BR><th>Description<BR>\n" +
		"\t<tr class=\"colData\"><td>and</a><BR><td>Finds records containing all of your search words.<br>\n" +
		"\t<tr class=\"colData\"><td>or</a><BR><td>Finds records containing either word. <br><br>\n" +
		"\t<tr class=\"colData\"><td>not<BR><td>Finds all records except those. <br><br>\n"

	once := format(t, src)

	twice := format(t, once)
	if once != twice {
		t.Errorf("not idempotent\n--- first ---\n%s\n--- second ---\n%s", once, twice)
	}
}

// TestSelfCloseConversionPreservesContent checks the narrower edit does not
// disturb the body it no longer rewrites.
func TestSelfCloseConversionPreservesContent(t *testing.T) {
	src := "<td>before<BR>after<BR>tail\n"

	out := format(t, src)
	for _, want := range []string{"before", "after", "tail"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q lost\ngot:\n%s", want, out)
		}
	}

	if strings.Contains(out, "<BR>") {
		t.Errorf("void element not converted\ngot:\n%s", out)
	}
}
