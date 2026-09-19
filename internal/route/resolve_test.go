package route

import (
	"strings"
	"testing"
)

// fake is a stand-in workspace, so these tests state the convention rather than
// depend on a fixture tree.
type fake struct {
	components map[string][]string // dot-path → method names
	dirs       map[string]bool
	files      map[string]bool
}

func (f fake) lookups() Lookups {
	return Lookups{
		ComponentPath: func(c string) string {
			if _, ok := f.components[c]; ok {
				return "/w/" + strings.ReplaceAll(c, ".", "/") + ".cfc"
			}

			return ""
		},
		HasMethod: func(path, method string) bool {
			c := strings.TrimSuffix(strings.TrimPrefix(path, "/w/"), ".cfc")
			for _, m := range f.components[strings.ReplaceAll(c, "/", ".")] {
				if strings.EqualFold(m, method) {
					return true
				}
			}

			return false
		},
		DirExists: func(rel string) bool { return f.dirs[rel] },
		FindFile: func(rel string) string {
			if f.files[rel] {
				return "/w/" + rel
			}

			return ""
		},
	}
}

// tassConfig is the convention measured against a real workspace: three
// controller rules and a longest-directory view rule.
func tassConfig() Config {
	return Config{
		Attributes: []string{"data-view", "data-read"},
		Aliases:    map[string][]string{"ui.web": {"tassweb", "kiosk", "parentportal"}},
		Controllers: []ControllerRule{
			{Component: "packages.tass.${1}-${2}", Method: "${3+:concat}"},
			{Component: "packages.tass.${2}", Method: "${3+:concat}"},
			{Component: "packages.tass.${1}", Method: "${2+:concat}"},
		},
		Views: []ViewRule{{LongestDir: true, Ext: []string{".cfm"}}},
	}
}

func TestTassConvention(t *testing.T) {
	w := fake{
		components: map[string][]string{
			"packages.tass.kiosk-customrollcall": {"dialogCustomRoll"},
			"packages.tass.tassweb":              {"adminChangeLogsGridViewRead"},
			"packages.tass.assessment":           {"dialogObjectiveGroupSetup"},
			"packages.tass.kiosk-calendar":       {"feedURLRead"},
		},
		dirs:  map[string]bool{"ui": true, "ui/web": true, "ui/web/general": true},
		files: map[string]bool{"ui/web/general/popup.lookup.filter.cfm": true},
	}
	r := &Resolver{Config: tassConfig(), Lookups: w.lookups()}

	cases := []struct {
		route  string
		kind   Kind
		expect string
	}{
		// Product-module controller: two segments make the file name.
		{"kiosk.customrollcall.dialog.custom.roll", KindController, "dialogcustomroll"},
		// Product prefix dropped: the controller is the second segment.
		{"tassweb.assessment.dialog.objectivegroup.setup", KindController, "dialogobjectivegroupsetup"},
		// Plain controller: everything after the first segment is the method.
		{"tassweb.admin.changelogsgridview.read", KindController, "adminchangelogsgridviewread"},
		// Longest existing directory, remainder is a dotted file name.
		{"ui.web.general.popup.lookup.filter", KindView, "ui/web/general/popup.lookup.filter.cfm"},
	}

	for _, c := range cases {
		got := r.Resolve(c.route)
		if len(got) == 0 {
			t.Errorf("%s: resolved to nothing", c.route)

			continue
		}

		first := got[0]
		if first.Kind != c.kind {
			t.Errorf("%s: kind = %s, want %s", c.route, first.Kind, c.kind)

			continue
		}

		actual := strings.ToLower(first.Method)
		if c.kind == KindView {
			actual = strings.TrimPrefix(first.Path, "/w/")
		}

		if actual != c.expect {
			t.Errorf("%s: got %q, want %q", c.route, actual, c.expect)
		}
	}
}

// TestAliasReachesEveryProduct. "ui.web" means the product serving the page, and
// which that is cannot be known statically — a shared view legitimately reaches
// more than one controller, and reporting only the first would be a guess
// presented as a fact.
func TestAliasReachesEveryProduct(t *testing.T) {
	w := fake{components: map[string][]string{
		"packages.tass.kiosk-calendar":   {"feedURLRead"},
		"packages.tass.tassweb-calendar": {"feedURLRead"},
	}}
	r := &Resolver{Config: tassConfig(), Lookups: w.lookups()}

	got := r.Resolve("ui.web.calendar.feedurl.read")
	if len(got) != 2 {
		t.Fatalf("want both products, got %d: %+v", len(got), got)
	}

	seen := map[string]bool{}
	for _, tgt := range got {
		seen[tgt.Component] = true

		if tgt.Alias == "" {
			t.Errorf("%s did not record which alias produced it", tgt.Component)
		}
	}

	for _, want := range []string{"packages.tass.tassweb-calendar", "packages.tass.kiosk-calendar"} {
		if !seen[want] {
			t.Errorf("%s missing from %v", want, seen)
		}
	}
}

// TestMethodMustExist is the check that makes rule order safe. A component
// template built from the first segment matches an enormous number of routes —
// every "tassweb.*" route resolves tassweb.cfc — so a rule that claimed a route
// without finding the method would shadow every rule below it.
func TestMethodMustExist(t *testing.T) {
	w := fake{components: map[string][]string{
		"packages.tass.tassweb":    {}, // exists, declares nothing
		"packages.tass.assessment": {"dialogThing"},
	}}
	r := &Resolver{Config: tassConfig(), Lookups: w.lookups()}

	got := r.Resolve("tassweb.assessment.dialog.thing")
	if len(got) != 1 {
		t.Fatalf("want exactly the assessment controller, got %+v", got)
	}

	if got[0].Component != "packages.tass.assessment" {
		t.Errorf("resolved to %q; the empty tassweb controller shadowed the real rule", got[0].Component)
	}
}

// TestFw1Convention: the same machinery, a different framework. If this needs a
// Go change, the config grammar is not general enough.
func TestFw1Convention(t *testing.T) {
	cfg := Config{
		Attributes:  []string{"data-route"},
		Controllers: []ControllerRule{{Component: "controllers.${1}", Method: "${2}"}},
		Views:       []ViewRule{{Path: "views/${1}/${2}", Ext: []string{".cfm"}}},
	}
	w := fake{
		components: map[string][]string{"controllers.section": {"item"}},
		files:      map[string]bool{"views/section/item.cfm": true},
	}
	r := &Resolver{Config: cfg, Lookups: w.lookups()}

	got := r.Resolve("section.item")
	if len(got) != 2 {
		t.Fatalf("want the controller method and the view, got %+v", got)
	}

	if got[0].Kind != KindController || got[0].Method != "item" {
		t.Errorf("first target = %+v, want controllers.section.item()", got[0])
	}

	if got[1].Kind != KindView || got[1].Path != "/w/views/section/item.cfm" {
		t.Errorf("second target = %+v, want views/section/item.cfm", got[1])
	}
}

// TestLongestDirectoryWins. The file name carries dots of its own, so the
// leftmost directory that exists is almost never the split that was meant.
func TestLongestDirectoryWins(t *testing.T) {
	w := fake{
		dirs: map[string]bool{"ui": true, "ui/web": true, "ui/web/general": true},
		files: map[string]bool{
			"ui/web/general/popup.lookup.filter.cfm": true,
			"ui/web.general.popup.lookup.filter.cfm": true, // a decoy at a shallower split
		},
	}
	r := &Resolver{Config: tassConfig(), Lookups: w.lookups()}

	got := r.Resolve("ui.web.general.popup.lookup.filter")
	if len(got) == 0 {
		t.Fatal("resolved to nothing")
	}

	if got[0].Path != "/w/ui/web/general/popup.lookup.filter.cfm" {
		t.Errorf("took the shallower split: %s", got[0].Path)
	}
}

func TestDisabledConfigResolvesNothing(t *testing.T) {
	r := &Resolver{Config: Config{}, Lookups: fake{}.lookups()}
	if got := r.Resolve("a.b.c"); got != nil {
		t.Errorf("an empty config resolved %+v", got)
	}

	if (Config{Attributes: []string{"data-view"}}).Enabled() {
		t.Error("a config with sources but no rules reports itself enabled")
	}
}

// TestViewPathTemplateKeepsDottedNames. The view template controls its own
// separators: a literal "/" between directories and whatever join the segment
// reference asks for inside a name. Rewriting every dot to a slash — which this
// did — made a dotted file name impossible to express, and that is exactly what
// "webroot/${2}/${3+}" needs: a directory called assessment holding
// dialog.objectivegroup.setup.cfm, not a directory called dialog.
func TestViewPathTemplateKeepsDottedNames(t *testing.T) {
	cfg := Config{
		Attributes: []string{"data-view"},
		Views:      []ViewRule{{Path: "webroot/${2}/${3+}", Ext: []string{".cfm"}}},
	}
	w := fake{files: map[string]bool{
		"webroot/assessment/dialog.objectivegroup.setup.cfm": true,
	}}
	r := &Resolver{Config: cfg, Lookups: w.lookups()}

	got := r.Resolve("tassweb.assessment.dialog.objectivegroup.setup")
	if len(got) != 1 {
		t.Fatalf("resolved to %+v", got)
	}

	if got[0].Path != "/w/webroot/assessment/dialog.objectivegroup.setup.cfm" {
		t.Errorf("path = %q; the dots in the file name were rewritten", got[0].Path)
	}

	// A template with only literal slashes — the FW/1 shape — must still work.
	fw1 := &Resolver{
		Config:  Config{Attributes: []string{"x"}, Views: []ViewRule{{Path: "views/${1}/${2}", Ext: []string{".cfm"}}}},
		Lookups: fake{files: map[string]bool{"views/section/item.cfm": true}}.lookups(),
	}

	if got := fw1.Resolve("section.item"); len(got) != 1 {
		t.Errorf("the slash-only template broke: %+v", got)
	}
}

// TestLongerAliasWins covers a module abbreviation sitting under a broader alias:
// "ui.web" means the product, but "ui.web.extracurric" names the extracurricular
// controller outright, and the specific key has to be tried first or it never
// gets a chance.
func TestLongerAliasWins(t *testing.T) {
	cfg := Config{
		Attributes: []string{"data-view"},
		Aliases: map[string][]string{
			"ui.web":             {"tassweb"},
			"ui.web.extracurric": {"extracurricular"},
		},
		Controllers: []ControllerRule{{Component: "c.${1}", Method: "${2+:search}"}},
	}
	w := fake{components: map[string][]string{
		"c.extracurricular": {"dialogExtracurricularActivityCloneRead"},
		"c.tassweb":         {},
	}}
	r := &Resolver{Config: cfg, Lookups: w.lookups()}

	got := r.Resolve("ui.web.extracurric.dialog.extracurricular.activity.clone.read")
	if len(got) != 1 {
		t.Fatalf("resolved to %+v", got)
	}

	if got[0].Component != "c.extracurricular" {
		t.Errorf("component = %q; the broader ui.web alias shadowed the specific one", got[0].Component)
	}
}

// TestSuffixIsStrippedNotResolved. A trailing segment can say how a target is
// presented rather than what it is: TASS writes "kiosk.lms.lms_grades.iframe" for
// the route kiosk.lms.lms_grades loaded inside an iframe container. Treating the
// suffix as part of the route leaves it resolving to nothing.
func TestSuffixIsStrippedNotResolved(t *testing.T) {
	cfg := Config{
		Attributes: []string{"data-view"},
		Suffixes:   []string{"iframe"},
		Views:      []ViewRule{{LongestDir: true, Ext: []string{".cfm"}}},
	}
	w := fake{
		dirs:  map[string]bool{"kiosk": true, "kiosk/lms": true},
		files: map[string]bool{"kiosk/lms/lms_grades.cfm": true},
	}
	r := &Resolver{Config: cfg, Lookups: w.lookups()}

	got := r.Resolve("kiosk.lms.lms_grades.iframe")
	if len(got) != 1 {
		t.Fatalf("resolved to %+v", got)
	}

	if got[0].Path != "/w/kiosk/lms/lms_grades.cfm" {
		t.Errorf("path = %q", got[0].Path)
	}

	// Without the suffix configured it must not resolve, or the test above proves
	// nothing about the stripping.
	plain := &Resolver{Config: Config{
		Attributes: cfg.Attributes, Views: cfg.Views,
	}, Lookups: w.lookups()}

	if got := plain.Resolve("kiosk.lms.lms_grades.iframe"); len(got) != 0 {
		t.Errorf("resolved without the suffix configured: %+v", got)
	}
}

// TestRouteAsWrittenBeatsTheStrippedReading. A configured suffix may also be a
// genuine trailing segment somewhere, so the unstripped route is tried first and
// keeps resolving the way it did before the suffix was configured.
func TestRouteAsWrittenBeatsTheStrippedReading(t *testing.T) {
	cfg := Config{
		Attributes:  []string{"data-view"},
		Suffixes:    []string{"read"},
		Controllers: []ControllerRule{{Component: "c.${1}", Method: "${2+:concat}"}},
	}
	w := fake{components: map[string][]string{"c.app": {"thingRead", "thing"}}}
	r := &Resolver{Config: cfg, Lookups: w.lookups()}

	got := r.Resolve("app.thing.read")
	if len(got) == 0 {
		t.Fatal("resolved to nothing")
	}

	// Method carries the template's expansion, which is case-folded like the rest
	// of route matching, so compare it that way.
	if !strings.EqualFold(got[0].Method, "thingRead") {
		t.Errorf("first target = %q, want thingRead: the stripped reading won", got[0].Method)
	}
}

// TestSuffixStripsBeforeAliasing. A suffix says how a target is presented and an
// alias says where it lives; stripping after aliasing would mean writing every
// alias twice, once per presentation.
func TestSuffixStripsBeforeAliasing(t *testing.T) {
	cfg := Config{
		Attributes:  []string{"data-view"},
		Suffixes:    []string{"iframe"},
		Aliases:     map[string][]string{"ui.web": {"kiosk"}},
		Controllers: []ControllerRule{{Component: "c.${1}-${2}", Method: "${3+:concat}"}},
	}
	w := fake{components: map[string][]string{"c.kiosk-lms": {"grades"}}}
	r := &Resolver{Config: cfg, Lookups: w.lookups()}

	got := r.Resolve("ui.web.lms.grades.iframe")
	if len(got) != 1 {
		t.Fatalf("resolved to %+v", got)
	}

	if got[0].Component != "c.kiosk-lms" || got[0].Method != "grades" {
		t.Errorf("target = %+v", got[0])
	}
}
