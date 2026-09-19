package route

import (
	"reflect"
	"strings"
	"testing"
)

func TestExpandTemplates(t *testing.T) {
	segs := []string{"kiosk", "customrollcall", "dialog", "custom", "roll"}

	cases := []struct {
		tmpl string
		want string
		ok   bool
	}{
		{"packages.tass.${1}-${2}", "packages.tass.kiosk-customrollcall", true},
		{"packages.tass.${1}", "packages.tass.kiosk", true},
		{"${3+:concat}", "dialogcustomroll", true},
		{"${3+}", "dialog.custom.roll", true},
		{"${3+:slash}", "dialog/custom/roll", true},
		{"${2-3}", "customrollcall.dialog", true},
		{"${1:upper}", "KIOSK", true},
		{"literal", "literal", true},

		// A template that reaches past the route declines, rather than producing a
		// short component path that resolves to something else entirely.
		{"${9}", "", false},
		{"${6+}", "", false},
		{"${0}", "", false},
	}

	for _, c := range cases {
		got, ok := expand(c.tmpl, segs)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("expand(%q) = (%q, %v), want (%q, %v)", c.tmpl, got, ok, c.want, c.ok)
		}
	}
}

// TestAliasExpansionIsDeterministic is the guard on a defect that only shows up
// between runs: ranging over the alias map directly made the winning expansion
// depend on Go's randomised map order, so the same route resolved to different
// components on different builds of the same code.
func TestAliasExpansionIsDeterministic(t *testing.T) {
	c := Config{Aliases: map[string][]string{
		"ui.web":     {"tassweb", "kiosk", "parentportal"},
		"ui.web.lab": {"labapp"},
		"a.b":        {"x"},
		"c":          {"y"},
	}}

	segs := Split("ui.web.lab.thing.read")

	first := c.expansions(segs)
	for range 50 {
		if !reflect.DeepEqual(c.expansions(segs), first) {
			t.Fatal("expansions() is not stable across calls; alias order is leaking map iteration order")
		}
	}

	// Longest key first: ui.web.lab must be tried before ui.web, or the specific
	// alias never gets a chance.
	if len(first) < 3 {
		t.Fatalf("expected the route plus both aliases, got %d expansions", len(first))
	}

	if first[0].alias != "" {
		t.Error("the route as written must be tried first")
	}

	if !strings.HasPrefix(first[1].alias, "ui.web.lab") {
		t.Errorf("the longer alias should be tried first, got %q", first[1].alias)
	}
}

func TestPlausible(t *testing.T) {
	cases := map[string]bool{
		"tassweb.admin.changelogsgridview.read": true,
		"ui.web.general.popup.lookup.filter":    true,
		"kiosk-listings.read":                   true,

		// Not routes, and every one of these appears in the corpus: panel ids, CSS
		// hooks, and values with a runtime expression in them. Running them through
		// resolution produces confident nonsense rather than nothing.
		"calendar-recur":                      false,
		"leaveapp-step1":                      false,
		"ui.web.calendar.popup<cfif x>.event": false,
		"#variables.route#":                   false,
		"has space.here":                      false,
		"":                                    false,
		".":                                   false,
		"trailing.":                           false,
	}

	for in, want := range cases {
		if got := Plausible(in); got != want {
			t.Errorf("Plausible(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestSplitIsCaseFolded(t *testing.T) {
	got := Split("UI.Web.General")
	want := []string{"ui", "web", "general"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("Split = %v, want %v", got, want)
	}

	if Split("a..b") != nil {
		t.Error("an empty segment should make the whole route unusable, not silently vanish")
	}
}

// TestScanFindsAttributesInRealisticMarkup covers the shapes these attributes
// actually appear in — inside CFML tags, with single quotes, with whitespace
// around the equals, and next to attributes with similar names.
func TestScanFindsAttributesInRealisticMarkup(t *testing.T) {
	src := `<cfoutput>
<div data-view="ui.web.general.popup.lookup.filter" class="x">
  <span data-read = 'tassweb.admin.changelogsgridview.read'></span>
  <b x-data-view="should.not.match"></b>
  <i data-viewport="also.not.this"></i>
  <em data-view="#variables.dynamic#"></em>
</div>
</cfoutput>`

	refs := Scan(src, []string{"data-view", "data-read"})

	var values []string
	for _, r := range refs {
		values = append(values, r.Value)
	}

	want := []string{
		"ui.web.general.popup.lookup.filter",
		"tassweb.admin.changelogsgridview.read",
		"#variables.dynamic#",
	}

	if !reflect.DeepEqual(values, want) {
		t.Fatalf("Scan found %v, want %v", values, want)
	}

	// x-data-view must not match: an attribute name that merely ends with one of
	// ours is a different attribute.
	for _, r := range refs {
		if strings.Contains(r.Value, "should.not.match") || strings.Contains(r.Value, "also.not.this") {
			t.Errorf("Scan matched a longer attribute name: %q", r.Value)
		}
	}

	// Positions must land on the value, so an editor puts the link on the route
	// and not on the quotes or the attribute name.
	first := refs[0]
	if first.Line != 1 {
		t.Errorf("first ref line = %d, want 1", first.Line)
	}

	lines := strings.Split(src, "\n")
	if got := lines[first.Line][first.Col : int(first.Col)+len(first.Value)]; got != first.Value {
		t.Errorf("Col/Value disagree: line slice is %q, value is %q", got, first.Value)
	}

	if src[first.Start:first.End] != first.Value {
		t.Errorf("Start/End disagree with Value: %q", src[first.Start:first.End])
	}
}

// TestScanCountsLinesAcrossValues: the scanner skips over a matched value, so it
// has to count any newlines inside it or every later reference is on the wrong
// line.
func TestScanCountsLinesAcrossValues(t *testing.T) {
	src := "a\n<i data-view=\"one.two\">\n\n<i data-view=\"three.four\">"

	refs := Scan(src, []string{"data-view"})
	if len(refs) != 2 {
		t.Fatalf("want 2 refs, got %d", len(refs))
	}

	if refs[0].Line != 1 || refs[1].Line != 3 {
		t.Errorf("lines = %d and %d, want 1 and 3", refs[0].Line, refs[1].Line)
	}
}
