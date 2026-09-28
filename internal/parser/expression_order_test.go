package parser

import (
	"slices"
	"testing"
)

// Overlapping keys: whichever applies first decides the component. The longer
// goes first. Map order is random on every range, so thirty parses cannot pass
// by luck.
func TestExpressionMappingsApplyLongestFirst(t *testing.T) {
	mappings := map[string]string{
		"#core#":        "pkg.core.",
		"#core#legacy.": "pkg.old.",
	}

	if got, want := ExpressionMappingOrder(mappings), []string{"#core#legacy.", "#core#"}; !slices.Equal(got, want) {
		t.Fatalf("order: got %v want %v", got, want)
	}

	src := "component {\n\tfunction f() {\n\t\tvar w = createObject( \"component\", \"#core#legacy.Widget\" );\n\t}\n}\n"

	for range 30 {
		pr := ParseWithOptions(testURI, src, &ParseOptions{ExpressionMappings: mappings})

		refs := pr.FuncComponentRefs(pr.Scopes[0].Start, pr.Scopes[0].End)
		if len(refs) != 1 || refs[0].Component != "pkg.old.Widget" {
			t.Fatalf("got %+v, want one ref to pkg.old.Widget", refs)
		}
	}
}
