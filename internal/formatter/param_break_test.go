package formatter

import (
	"strings"
	"testing"
)

func formatWithParamBreak(t *testing.T, src string, threshold int) string {
	t.Helper()

	opts := testOpts()
	opts.WhitespaceOnly = true
	opts.ParamBreakThreshold = threshold

	out, err := Format([]byte(src), parse(t, src), opts)
	if err != nil {
		t.Fatalf("format error (paramBreakThreshold=%d): %v", threshold, err)
	}

	return string(out)
}

func decl(sig string) string {
	return "<cfscript>\ncomponent {\n\t" + sig + " {\n\t\tx = 1;\n\t}\n}\n</cfscript>\n"
}

// TestParamBreakUnsetBreaksEveryList is the constraint. The threshold's default
// of zero has to reproduce what the formatter has always done — break a
// declaration's parameters onto separate lines however few and however short
// they are — or adding the setting reformats every signature in every project.
func TestParamBreakUnsetBreaksEveryList(t *testing.T) {
	t.Parallel()

	out := formatWithParamBreak(t, decl("function short(a)"), 0)

	assertContains(t, out, "function short(\n")
	assertNotContains(t, out, "function short(a)")
}

// TestParamBreakEmptyListStaysInline is the boundary at the other end: "()" has
// nothing to break and must not gain a line of its own at any threshold.
func TestParamBreakEmptyListStaysInline(t *testing.T) {
	t.Parallel()

	for _, threshold := range []int{0, 3} {
		out := formatWithParamBreak(t, decl("function none()"), threshold)
		assertContains(t, out, "function none() {")
	}
}

// TestParamBreakKeepsShortListsInline is what the setting is for.
func TestParamBreakKeepsShortListsInline(t *testing.T) {
	t.Parallel()

	out := formatWithParamBreak(t, decl("function three(required numeric id, boolean flag, string name)"), 3)

	assertContains(t, out, "function three(required numeric id, boolean flag, string name) {")
}

// TestParamBreakBreaksAboveTheThreshold is the other half of the same rule.
func TestParamBreakBreaksAboveTheThreshold(t *testing.T) {
	t.Parallel()

	out := formatWithParamBreak(t, decl("function four(a, b, c, d)"), 3)

	assertContains(t, out, "function four(\n")
	assertNotContains(t, out, "function four(a, b, c, d)")
}

// TestParamBreakStillBreaksPastLineWidth stops a generous threshold from
// producing signatures that run off the screen. The count says a list may stay
// on one line; the width says whether it fits.
func TestParamBreakStillBreaksPastLineWidth(t *testing.T) {
	t.Parallel()

	long := "function wide(required string alphaBetaGammaDelta, required string epsilonZetaEtaTheta, required string iotaKappaLambdaMu)"

	out := formatWithParamBreak(t, decl(long), 99)

	assertContains(t, out, "function wide(\n")
}

// TestParamBreakRefusesCommentsAndTrailingCommas covers the two shapes that
// cannot be folded onto one line whatever the threshold says: a `//` comment
// would swallow the rest of the signature including the brace, and a trailing
// comma the source wrote would have to be either dropped — a non-whitespace
// change the guard would reject — or emitted as `(a, b,)`.
func TestParamBreakRefusesCommentsAndTrailingCommas(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"comment among the parameters": "<cfscript>\ncomponent {\n\tfunction f(\n\t\ta,\n\t\t// why\n\t\tb\n\t) {\n\t\tx = 1;\n\t}\n}\n</cfscript>\n",
		"trailing comma":               "<cfscript>\ncomponent {\n\tfunction f(a, b,) {\n\t\tx = 1;\n\t}\n}\n</cfscript>\n",
	}

	for name, src := range cases {
		out := formatWithParamBreak(t, src, 99)
		if !strings.Contains(out, "function f(\n") {
			t.Errorf("%s: parameters were folded onto one line:\n%s", name, out)
		}
	}
}

// TestParamBreakIsIdempotent — a threshold that keeps a list inline on the
// first pass has to keep it inline on the second, when the source it reads is
// the one-line form it just wrote.
func TestParamBreakIsIdempotent(t *testing.T) {
	t.Parallel()

	src := decl("function three(required numeric id, boolean flag, string name)")

	for _, threshold := range []int{0, 3, 99} {
		once := formatWithParamBreak(t, src, threshold)
		if twice := formatWithParamBreak(t, once, threshold); twice != once {
			t.Errorf("threshold %d not idempotent:\n--- pass 1\n%s\n--- pass 2\n%s", threshold, once, twice)
		}
	}
}
