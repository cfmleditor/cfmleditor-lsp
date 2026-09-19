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

	refs := Scan(src, Config{Attributes: []string{"data-view", "data-read"}})

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

	refs := Scan(src, Config{Attributes: []string{"data-view"}})
	if len(refs) != 2 {
		t.Fatalf("want 2 refs, got %d", len(refs))
	}

	if refs[0].Line != 1 || refs[1].Line != 3 {
		t.Errorf("lines = %d and %d, want 1 and 3", refs[0].Line, refs[1].Line)
	}
}

// TestScanFindsAllThreeSyntaxes covers the forms routes are actually written in:
// an HTML attribute, a URL parameter inside an href, and a JavaScript object key
// with the name quoted or bare.
func TestScanFindsAllThreeSyntaxes(t *testing.T) {
	src := `<a href="index.cfm?do=kiosk.lms.main.learningobjects&page=2">go</a>
<div data-process="ui.web.lab.buttons.process"></div>
<script>
  var a = { view: "ui.web.user.dialog.login" };
  var b = { "read": 'tassweb.students.student.search' };
  var c = { preview: "not.a.source" };
  var d = { view: window.somethingElse };
</script>
<a href="x.cfm?domain=nope.nope">domain must not match do</a>`

	cfg := Config{
		Attributes:  []string{"data-process"},
		QueryParams: []string{"do"},
		Properties:  []string{"read", "view", "process"},
	}

	var got []string
	for _, r := range Scan(src, cfg) {
		got = append(got, string(r.Source)+":"+r.Name+"="+r.Value)
	}

	want := []string{
		"query:do=kiosk.lms.main.learningobjects",
		"attribute:data-process=ui.web.lab.buttons.process",
		"property:view=ui.web.user.dialog.login",
		"property:read=tassweb.students.student.search",
	}

	if len(got) != len(want) {
		t.Fatalf("Scan found %v,\n            want %v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ref %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestQueryParamStopsAtTheDelimiter. The value is unquoted because it sits inside
// the href's own quotes, so a scan that ran to the closing quote would swallow
// every later parameter into the route.
func TestQueryParamStopsAtTheDelimiter(t *testing.T) {
	cfg := Config{QueryParams: []string{"do"}}

	cases := map[string]string{
		`href="a.cfm?do=x.y&z=1"`:     "x.y",
		`href="a.cfm?do=x.y&amp;z=1"`: "x.y",
		`href="a.cfm?p=1&do=x.y"`:     "x.y",
		`href='a.cfm?do=x.y#frag'`:    "x.y",
		`<a href="a.cfm?do=x.y">text`: "x.y",
		`url("a.cfm?do=x.y")`:         "x.y",
	}

	for src, want := range cases {
		refs := Scan(src, cfg)
		if len(refs) != 1 {
			t.Errorf("%s: found %d refs, want 1", src, len(refs))

			continue
		}

		if refs[0].Value != want {
			t.Errorf("%s: value = %q, want %q", src, refs[0].Value, want)
		}
	}
}

// TestNestedMatchWins: an href is matched as an attribute and the do= inside it as
// a query parameter. The inner, more specific one is the route; keeping both would
// produce an unresolvable outer match for every link on the page.
func TestNestedMatchWins(t *testing.T) {
	src := `<a href="index.cfm?do=a.b.c">x</a>`

	refs := Scan(src, Config{
		Attributes:  []string{"href"},
		QueryParams: []string{"do"},
	})

	if len(refs) != 1 {
		t.Fatalf("want 1 ref, got %d: %+v", len(refs), refs)
	}

	if refs[0].Source != SourceQueryParam || refs[0].Value != "a.b.c" {
		t.Errorf("kept the outer match: %+v", refs[0])
	}
}

// TestPositionsSurviveMultipleSyntaxes. Matching happens before positions are
// assigned, so a matcher no longer has to remember to count the newlines inside a
// value it skipped — this is the test that would have caught that bookkeeping.
func TestPositionsSurviveMultipleSyntaxes(t *testing.T) {
	src := "line0\n<a href=\"x.cfm?do=a.b\">\n\n<i data-view=\"c.d\">\n<script>var q={view:\"e.f\"}</script>"

	refs := Scan(src, Config{
		Attributes:  []string{"data-view"},
		QueryParams: []string{"do"},
		Properties:  []string{"view"},
	})

	if len(refs) != 3 {
		t.Fatalf("want 3 refs, got %d: %+v", len(refs), refs)
	}

	lines := strings.Split(src, "\n")

	for _, r := range refs {
		if int(r.Line) >= len(lines) {
			t.Fatalf("%q reported line %d, past the end of the file", r.Value, r.Line)
		}

		got := lines[r.Line][r.Col : int(r.Col)+len(r.Value)]
		if got != r.Value {
			t.Errorf("%q: Line/Col point at %q", r.Value, got)
		}

		if src[r.Start:r.End] != r.Value {
			t.Errorf("%q: Start/End point at %q", r.Value, src[r.Start:r.End])
		}
	}

	if refs[0].Line != 1 || refs[1].Line != 3 || refs[2].Line != 4 {
		t.Errorf("lines = %d, %d, %d; want 1, 3, 4", refs[0].Line, refs[1].Line, refs[2].Line)
	}
}

// TestScanFunctionArgs covers routes passed to a function, in CFML and in
// JavaScript, including the query string and fragment such a route routinely
// carries after it.
func TestScanFunctionArgs(t *testing.T) {
	src := `<cfset ARGUMENTS.context.setPrint("fundraising.fundraising.event_print_action")>
<cfscript> redirect('intranet.client.detail&customercode=ABC'); </cfscript>
<script>
  redirect("intranet.ims.main.incidents#detail//");
  notARoutingCall("a.b.c");
  myRedirect("nested.name.must.not.match");
</script>`

	cfg := Config{Functions: []string{"setPrint", "redirect"}}

	var got []string
	for _, r := range Scan(src, cfg) {
		got = append(got, r.Name+"="+r.Value)
	}

	want := []string{
		"setprint=fundraising.fundraising.event_print_action",
		"redirect=intranet.client.detail",
		"redirect=intranet.ims.main.incidents",
	}

	if len(got) != len(want) {
		t.Fatalf("Scan found %v,\n            want %v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ref %d = %q, want %q", i, got[i], want[i])
		}
	}

	// The trimmed value must still be the text the offsets point at, or an editor
	// underlines the query string along with the route.
	for _, r := range Scan(src, cfg) {
		if src[r.Start:r.End] != r.Value {
			t.Errorf("%q: Start/End point at %q", r.Value, src[r.Start:r.End])
		}
	}
}

// TestFunctionNameBoundary: myRedirect must not match redirect, the same way
// domain must not match do.
func TestFunctionNameBoundary(t *testing.T) {
	for _, src := range []string{`myRedirect("a.b.c")`, `x.redirectTo("a.b.c")`} {
		if refs := Scan(src, Config{Functions: []string{"redirect"}}); len(refs) != 0 {
			t.Errorf("%s matched: %+v", src, refs)
		}
	}

	// A method call on an object is still the function, though.
	if refs := Scan(`context.redirect("a.b.c")`, Config{Functions: []string{"redirect"}}); len(refs) != 1 {
		t.Errorf("a method call did not match: %+v", refs)
	}
}

// TestSearchModeTriesEveryStartPoint. A route's segments do not say where the
// controller's name stops and the method's begins — the same shape is spelled
// studentMainStudent() on one controller and mainStudent() on another — so one
// rule enumerates the start points instead of one rule per start point.
func TestSearchModeTriesEveryStartPoint(t *testing.T) {
	segs := Split("kiosk.student.main.student")

	got := expandAll("${2+:search}", segs)
	want := []string{"studentmainstudent", "mainstudent", "student"}

	if len(got) != len(want) {
		t.Fatalf("expandAll gave %v, want %v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("candidate %d = %q, want %q", i, got[i], want[i])
		}
	}

	// Longest first matters: a trailing "read" or "view" is a common method name,
	// and taking the shortest candidate first would match it on the wrong route.
	if got[0] != "studentmainstudent" {
		t.Error("candidates are not longest-first")
	}

	// An exact mode still yields exactly one.
	if len(expandAll("${2+:concat}", segs)) != 1 {
		t.Error(":concat should not enumerate")
	}
}

// TestSearchPicksTheMethodThatExists is the pair to the above: enumerating is
// only safe because every candidate is checked against the component's real
// methods, so a wrong start point resolves to nothing rather than to an edge.
func TestSearchPicksTheMethodThatExists(t *testing.T) {
	cfg := Config{
		Attributes:  []string{"data-view"},
		Controllers: []ControllerRule{{Component: "c.${1}", Method: "${2+:search}"}},
	}

	for _, want := range []string{"studentmainstudent", "mainstudent"} {
		w := fake{components: map[string][]string{"c.kiosk": {want}}}
		r := &Resolver{Config: cfg, Lookups: w.lookups()}

		got := r.Resolve("kiosk.student.main.student")
		if len(got) != 1 || got[0].Method != want {
			t.Errorf("with only %q defined, resolved to %+v", want, got)
		}
	}
}

// TestFunctionArgsAreNameAgnostic. A route may be passed positionally, or as
// route="a.b.c", or as r="a.b.c" — the parameter name is the caller's business,
// and a scanner that had to be told it would need a config entry per function per
// codebase. Every string in the argument list is taken and Plausible rejects the
// rest.
func TestFunctionArgsAreNameAgnostic(t *testing.T) {
	cfg := Config{Functions: []string{"setRequestContext", "redirect"}}

	cases := map[string][]string{
		`setRequestContext("a.b.c")`:                    {"a.b.c"},
		`setRequestContext(route="a.b.c")`:              {"a.b.c"},
		`setRequestContext(r="a.b.c")`:                  {"a.b.c"},
		`setRequestContext(x=1, route='a.b.c', y=2)`:    {"a.b.c"},
		`setRequestContext(context=ctx, route="a.b.c")`: {"a.b.c"},
		`redirect(buildRoute("a.b.c"))`:                 {"a.b.c"},
		`redirect("a.b.c", "d.e.f")`:                    {"a.b.c", "d.e.f"},
		`setRequestContext(route="a.b.c&x=1")`:          {"a.b.c"},
	}

	for src, want := range cases {
		var got []string

		for _, r := range Scan(src, cfg) {
			got = append(got, r.Value)
		}

		if len(got) != len(want) {
			t.Errorf("%s: found %v, want %v", src, got, want)

			continue
		}

		for i := range want {
			if got[i] != want[i] {
				t.Errorf("%s: value %d = %q, want %q", src, i, got[i], want[i])
			}
		}
	}
}

// TestUnclosedCallIsNotScanned. A missing paren would otherwise walk to the end
// of the file for every call — in generated code, in a fragment inside a string,
// in a file caught mid-write.
func TestUnclosedCallIsNotScanned(t *testing.T) {
	cfg := Config{Functions: []string{"redirect"}}

	for _, src := range []string{
		`redirect("a.b.c"`,       // never closes
		`redirect('a.b.c)`,       // unclosed quote
		"redirect(\n\"a.b.c\"\n", // unclosed across lines
	} {
		if refs := Scan(src, cfg); len(refs) != 0 {
			t.Errorf("%q produced %+v", src, refs)
		}
	}

	// A call closing just inside the budget is still read.
	long := "redirect(" + strings.Repeat("x=1, ", 200) + `route="a.b.c")`
	if refs := Scan(long, cfg); len(refs) != 1 || refs[0].Value != "a.b.c" {
		t.Errorf("a long but valid argument list was not read: %+v", refs)
	}
}

// TestNonRouteArgumentsAreRejected: taking every string means Plausible is what
// stands between the scan and nonsense, so it has to hold for the shapes that
// actually appear beside a route.
func TestNonRouteArgumentsAreRejected(t *testing.T) {
	cfg := Config{Functions: []string{"setRequestContext"}}

	src := `setRequestContext(route="a.b.c", title="Some Title Here", flag="yes", n="42", expr="#var#")`

	var got []string

	for _, r := range Scan(src, cfg) {
		if Plausible(r.Value) {
			got = append(got, r.Value)
		}
	}

	if len(got) != 1 || got[0] != "a.b.c" {
		t.Errorf("plausible values = %v, want just a.b.c", got)
	}
}
