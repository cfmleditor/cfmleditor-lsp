package config

import (
	"slices"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
)

// TestColdBoxImplicitBases: a file that extends nothing is given the base its
// place in a ColdBox app implies — a directory anywhere in its path, the
// deepest winning, and the right extension; a file rule only for that file.
func TestColdBoxImplicitBases(t *testing.T) {
	implicit := ImplicitExtends([]string{"coldbox"})

	for path, want := range map[string]string{
		"/app/handlers/Main.cfc":                         "coldbox.system.EventHandler",
		"/app/modules_app/blog/handlers/admin/Posts.cfc": "coldbox.system.EventHandler",
		"/app/interceptors/Security.cfc":                 "coldbox.system.Interceptor",
		"/app/views/main/index.cfm":                      "coldbox.system.web.Renderer",
		"/app/layouts/Main.cfm":                          "coldbox.system.web.Renderer",
		"/app/views/handlers/list.cfm":                   "coldbox.system.web.Renderer", // a .cfc rule is not a .cfm's
		"/app/interceptors/handlers/Audit.cfc":           "coldbox.system.EventHandler", // deepest directory wins
		"/app/config/Router.cfc":                         "coldbox.system.web.routing.Router",
		"/app/config/Scheduler.cfc":                      "coldbox.system.web.tasks.ColdBoxScheduler",
		"/app/config/Coldbox.cfc":                        "", // no base for the config itself
		"/app/handlers/helper.cfm":                       "", // a template in handlers is not a handler
		"/app/models/UserService.cfc":                    "",
		"/app/Handlers/Main.cfc":                         "coldbox.system.EventHandler", // any case
	} {
		if got := implicit(path); got != want {
			t.Errorf("%s: %q, want %q", path, got, want)
		}
	}

	if ImplicitExtends(nil) != nil || ImplicitExtends([]string{"nosuch"}) != nil {
		t.Error("no framework, no rule: the resolver should pay nothing")
	}
}

// TestColdBoxResolversMatchWholeNames: `event` is the request context in every
// handler, view and interceptor, and so are its scoped spellings — but not
// `oEvent`, `prc.event` or a call on it. A method's return type answers the
// chain on it, however it was reached, and not a longer name or a further hop.
func TestColdBoxResolversMatchWholeNames(t *testing.T) {
	var rs []parser.Resolver
	for _, r := range FrameworkResolvers([]string{"coldbox"}) {
		rs = append(rs, parser.Resolver{Match: r.Match, Resolve: r.Resolve, Prefix: r.Prefix, Anchored: r.Anchored, DynamicIfMissing: r.DynamicIfMissing})
	}

	const rc = "coldbox.system.web.context.RequestContext"

	for expr, want := range map[string]string{
		"event":               rc,
		"arguments.event":     rc,
		"VARIABLES.Event":     rc,
		"oEvent":              "",
		"prc.event":           "",
		"eventManager":        "",
		"wirebox":             "coldbox.system.ioc.Injector",
		"variables.injector":  "coldbox.system.ioc.Injector",
		"cbController":        "coldbox.system.web.Controller",
		"getRequestContext()": rc,
		"variables.controller.getRequestContext()": rc,
		"getRequestContextFor()":                   "",
		"getRequestContext().getValue()":           "",
	} {
		if got, _ := parser.ResolveFromCallFull(expr, rs); got != want {
			t.Errorf("%s: %q, want %q", expr, got, want)
		}
	}

	for _, r := range FrameworkResolvers([]string{"coldbox"}) {
		if !r.DynamicIfMissing {
			t.Errorf("%s: a preset resolver must be dynamicIfMissing, or an app without the framework checked out reports it missing", r.Match)
		}
	}
}

// TestFrameworksMergeAndOrder: a child config adds to its parent's frameworks,
// each name once; the presets' resolvers follow the config's own, so a
// project's rule for a name wins over the preset's.
func TestFrameworksMergeAndOrder(t *testing.T) {
	got := Merge(&JSON{Frameworks: []string{"coldbox"}}, &JSON{Frameworks: []string{"ColdBox", "wheels"}})
	if !slices.Equal(got.Frameworks, []string{"ColdBox", "wheels"}) {
		t.Errorf("merged frameworks = %v, want the child's with the parent's duplicate dropped", got.Frameworks)
	}

	if got := Merge(&JSON{Frameworks: []string{"coldbox"}}, &JSON{}).Frameworks; !slices.Equal(got, []string{"coldbox"}) {
		t.Errorf("a child naming none keeps the parent's: %v", got)
	}

	own := Resolver{Match: "event", Resolve: "my.Event", Prefix: "event"}
	r := Resolve(&JSON{Frameworks: []string{"coldbox"}, ComponentResolvers: []Resolver{own}}, "/w")

	if len(r.ComponentResolvers) < 2 || r.ComponentResolvers[0] != own {
		t.Errorf("the config's own resolver must come first: %v", r.ComponentResolvers)
	}

	if !slices.Equal(r.Frameworks, []string{"coldbox"}) {
		t.Errorf("Resolved.Frameworks = %v", r.Frameworks)
	}
}

func TestUnknownFrameworks(t *testing.T) {
	if got := UnknownFrameworks([]string{"ColdBox", "coldbx", "rails"}); !slices.Equal(got, []string{"coldbx", "rails"}) {
		t.Errorf("UnknownFrameworks = %v", got)
	}

	if !slices.Contains(KnownFrameworks(), "coldbox") {
		t.Errorf("KnownFrameworks = %v", KnownFrameworks())
	}
}

// TestEveryPresetResolverMatchesItsOwnNames: a name is taken literally — the
// $ in TestBox's $assert is a character, not an end-of-line anchor, and the
// dots in prc.oCurrentAuthor are dots — so each preset's variables resolve to
// what it says, and a near miss resolves to nothing.
func TestEveryPresetResolverMatchesItsOwnNames(t *testing.T) {
	for _, tc := range []struct{ framework, expr, want string }{
		{"testbox", "$assert", "testbox.system.Assertion"},
		{"testbox", "variables.assert", "testbox.system.Assertion"},
		{"testbox", "getMockBox()", "testbox.system.MockBox"},
		{"commandbox", "print", "commandbox.system.util.PrintBuffer"},
		{"commandbox", "command()", "commandbox.system.util.CommandDSL"},
		{"commandbox", "task()", ""}, // a ColdBox scheduler's task() is not CommandBox's
		{"coldbox", "task()", "coldbox.system.web.tasks.ColdBoxScheduledTask"},
		{"contentbox", "prc.oContent", "contentbox.models.content.BaseContent|contentbox.models.content.Entry|contentbox.models.content.Page|contentbox.models.content.ContentStore"},
		{"cfmigrations", "table", "qb.models.Schema.Blueprint"},
		{"cfmigrations", "arguments.qb", "qb.models.Query.QueryBuilder"},
		{"contentbox", "prc.oCurrentAuthor", "contentbox.models.security.Author"},
		{"contentbox", "prcXoCurrentAuthor", ""},
		{"wheels", "application.wo", "wheels.Global"},
		{"fw1", "variables.fw", "framework.one"},
	} {
		var rs []parser.Resolver
		for _, r := range FrameworkResolvers([]string{tc.framework}) {
			rs = append(rs, parser.Resolver{Match: r.Match, Resolve: r.Resolve, Prefix: r.Prefix, Anchored: r.Anchored})
		}

		if got, _ := parser.ResolveFromCallFull(tc.expr, rs); got != tc.want {
			t.Errorf("%s %s: %q, want %q", tc.framework, tc.expr, got, tc.want)
		}
	}

	implicit := ImplicitExtends([]string{"commandbox", "fw1"})
	for path, want := range map[string]string{
		"/p/task.cfc":                     "commandbox.system.BaseTask",
		"/p/deep/in/tree/Task.cfc":        "commandbox.system.BaseTask",
		"/p/commands/wheels/Generate.cfc": "commandbox.system.BaseCommand",
		"/p/views/main/default.cfm":       "framework.one",
		"/p/models/task.cfm":              "",
	} {
		if got := implicit(path); got != want {
			t.Errorf("%s: %q, want %q", path, got, want)
		}
	}
}

// TestSuggestFrameworksFromBoxJSON: a project finds out a preset exists from
// its own box.json — a dependency, a dev dependency or the package's own slug
// (coldbox-platform is the package named coldbox) — minus the presets the
// config already names. A dependency that says nothing about the framework
// the code is written against suggests nothing.
func TestSuggestFrameworksFromBoxJSON(t *testing.T) {
	for _, tc := range []struct {
		box  string
		have []string
		want []string
	}{
		{`{"dependencies":{"coldbox":"^7"},"devDependencies":{"testbox":"*","commandbox-migrations":"*"}}`, nil, []string{"cfmigrations", "coldbox", "testbox"}},
		{`{"slug":"coldbox","devDependencies":{"testbox":"*"}}`, []string{"ColdBox"}, []string{"testbox"}},
		{`{"dependencies":{"wheels-core":"^3"}}`, nil, []string{"wheels"}},
		{`{"slug":"fw1"}`, nil, []string{"fw1"}},
		{`{"devDependencies":{"commandbox-cfformat":"*","cbproxies":"*"}}`, nil, nil},
		{`not json`, nil, nil},
	} {
		if got := SuggestFrameworks([]byte(tc.box), tc.have); !slices.Equal(got, tc.want) {
			t.Errorf("%s with %v: %v, want %v", tc.box, tc.have, got, tc.want)
		}
	}
}

// TestColdBoxHelperScope: ColdBox mixes its helpers into handlers,
// interceptors, views and layouts, at any depth, and into nothing else; a
// preset with no helpers gives no scope, so the resolver pays nothing.
func TestColdBoxHelperScope(t *testing.T) {
	scope := HelperScope([]string{"coldbox"})
	for path, want := range map[string]bool{
		"/app/handlers/Main.cfc":                true,
		"/app/modules/blog/views/post/show.cfm": true,
		"/app/layouts/Main.cfm":                 true,
		"/app/interceptors/Security.cfc":        true,
		"/app/models/User.cfc":                  false,
		"/app/config/Router.cfc":                false,
	} {
		if got := scope(path); got != want {
			t.Errorf("%s: %v, want %v", path, got, want)
		}
	}

	if HelperScope([]string{"testbox"}) != nil {
		t.Error("a preset with no helper directories should give no scope")
	}
}
