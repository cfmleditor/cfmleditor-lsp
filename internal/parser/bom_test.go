package parser

import (
	"slices"
	"testing"
)

// bom is the UTF-8 byte order mark, spelled in bytes because Go source may not
// contain one anywhere but its own first three bytes.
const bom = "\xef\xbb\xbf"

// A BOM is an encoding marker, not a token, and it is invisible in an editor —
// 561 of the 5,629 corpus files carry one. A scanner that stopped at it made
// ClassifyRegions answer "the first token is not `component`", so a script
// `.cfc` went to the tag splitter. That is harmless until the file mentions
// `<script>`, which `isScriptFile` reads as evidence of an HTML page. ColdBox's
// HTMLHelperSpec.cfc only mentions it inside string literals — the strings the
// spec asserts against — and came apart into eight regions, each parsed from the
// middle of an expression: no function found in 800 lines, and 481 call sites
// lost across the corpus.
func TestALeadingBOMIsNotAToken(t *testing.T) {
	const src = "// a comment\n" +
		"component extends=\"Base\" {\n" +
		"	function beforeAll() {\n" +
		"		assertEquals( \"<script src=\"\"/js/t.js\"\"></script>\", render() );\n" +
		"	}\n" +
		"}\n"

	for _, c := range []struct {
		name string
		src  string
	}{
		{"without a BOM", src},
		{"with a BOM", bom + src},
	} {
		t.Run(c.name, func(t *testing.T) {
			regions := ClassifyRegions(c.src)
			if len(regions) != 1 || regions[0].Kind != RegionScript {
				t.Errorf("got %d regions, want one script region", len(regions))

				for i, r := range regions {
					t.Logf("  region %d kind=%v %q", i, r.Kind, r.Text)
				}
			}

			pr := ParseWithOptions(testURI, c.src, &ParseOptions{ExtractCalls: true})
			if len(pr.Funcs) != 1 || pr.Funcs[0].Name != "beforeAll" {
				t.Errorf("funcs: got %v, want [beforeAll]", pr.Funcs)
			}

			var calls []string
			for _, call := range pr.AllCalls() {
				calls = append(calls, call.FuncName)
			}

			slices.Sort(calls)

			if want := []string{"assertEquals", "render"}; !slices.Equal(calls, want) {
				t.Errorf("calls: got %v want %v", calls, want)
			}
		})
	}

	// The BOM must not become part of the first token either, which is what
	// makes the classification answer above right for the right reason.
	if tok := NewScanner(bom + "component {}").NextSkipComments(); tok.Value != "component" {
		t.Errorf("first token is %q, want %q", tok.Value, "component")
	}
}
