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
		Sources: []string{"data-view", "data-read"},
		Aliases: map[string][]string{"ui.web": {"tassweb", "kiosk", "parentportal"}},
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
		Sources:     []string{"data-route"},
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

	if (Config{Sources: []string{"data-view"}}).Enabled() {
		t.Error("a config with sources but no rules reports itself enabled")
	}
}
