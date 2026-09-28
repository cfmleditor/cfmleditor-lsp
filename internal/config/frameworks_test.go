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
	if got := UnknownFrameworks([]string{"ColdBox", "coldbx", "wheels"}); !slices.Equal(got, []string{"coldbx", "wheels"}) {
		t.Errorf("UnknownFrameworks = %v", got)
	}

	if !slices.Contains(KnownFrameworks(), "coldbox") {
		t.Errorf("KnownFrameworks = %v", KnownFrameworks())
	}
}
