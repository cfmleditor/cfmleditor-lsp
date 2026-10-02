package parser

import "testing"

// A function's refs are what its body assigns, typed as the full parse types
// them, whether they come from the parse, from the lazy body parse FuncRefs
// falls back to for a function the parse filed none under, or from that same
// parse after an edit invalidated the function. The lazy parse resolved no
// pending calls, so `variables.y = variables.y.next()` and `x = make()` added
// nothing there and the variable kept its first type.
func TestLazyFuncRefsAreTypedAsTheParseTypesThem(t *testing.T) {
	src := `component {
function make() { return new models.B(); }
function scoped() {
	variables.y = new models.A();
	variables.y = variables.y.next();
	variables.y.onlyB();
}
function bare() {
	variables.z = new models.A();
	variables.z = make();
}
}`
	lookup := func(comp, fn string) string {
		if comp == "models.A" && fn == "next" {
			return "models.B"
		}

		return ""
	}

	latest := func(refs []ComponentRef, name string) string {
		var best *ComponentRef

		for i := range refs {
			if refs[i].Variable == name && (best == nil || refs[i].Line >= best.Line) {
				best = &refs[i]
			}
		}

		if best == nil {
			return ""
		}

		return best.Component
	}

	pr := ParseWithOptions(testURI, src, &ParseOptions{FuncLookup: lookup})

	check := func(when string) {
		t.Helper()

		for _, tc := range []struct{ fn, name string }{{"scoped", "y"}, {"bare", "z"}} {
			var scope FuncScope

			for _, s := range pr.Scopes {
				if s.Name == tc.fn {
					scope = s
				}
			}

			refs, _ := pr.FuncRefs(scope.Start, scope.End)
			if got := latest(refs, tc.name); got != "models.B" {
				t.Errorf("%s: %s's latest %s ref is %q, want models.B (refs %+v)", when, tc.fn, tc.name, got, refs)
			}
		}
	}

	check("parsed")

	for _, s := range pr.Scopes {
		pr.InvalidateFunc(s.Start, s.End)
	}

	check("after an edit")
}
