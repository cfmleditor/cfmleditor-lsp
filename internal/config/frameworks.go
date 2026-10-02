package config

import (
	"encoding/json"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
)

// A framework preset is what naming a framework in `frameworks` adds to a
// config: resolvers for the values the framework hands every file, and the
// component a file of a given kind extends when it names none.
//
// A preset is only resolvers and bases, the machinery a project would
// otherwise write by hand, so anything a preset says can be overridden:
// the config's own componentResolvers come first. Every resolver is
// dynamicIfMissing — with the framework's source in the workspace a call is
// checked against its real methods, and without it the call is dynamic rather
// than reported against a component that does not exist.
type frameworkPreset struct {
	resolvers []Resolver
	bases     []implicitBase
	// helperDirs are the directories whose files the framework mixes its
	// helper templates into; the resolver finds the templates (helpers.go).
	helperDirs []string
}

// implicitBase is the component a file extends when it names none: a
// ColdBox handler with no extends attribute is an EventHandler all the same,
// and a view is rendered from inside the Renderer, so a bare view() or
// getInstance() in either is the framework's.
type implicitBase struct {
	dir       string // a directory of the file's path, at any depth; "" for any
	file      string // the file's name, when the rule is for one file
	ext       string // ".cfc" or ".cfm"
	component string
}

// variableResolver types a variable and its scoped spellings by name alone.
// Anchored and whole-name, so `oEvent` is not `event`.
func variableResolver(component string, names ...string) Resolver {
	alts := make([]string, 0, len(names))
	prefixes := make([]string, 0, len(names)*3)

	for _, n := range names {
		alts = append(alts, regexp.QuoteMeta(n))
		prefixes = append(prefixes, n, "variables."+n, "arguments."+n)
	}

	return Resolver{
		Match:            `^(?:variables\.|arguments\.)?(?:` + strings.Join(alts, "|") + `)$`,
		Resolve:          component,
		Prefix:           strings.Join(prefixes, "|"),
		Anchored:         true,
		DynamicIfMissing: true,
		NameOnly:         true,
	}
}

// returnResolver types what a method returns, for a call chained on it: the
// resolver is tried against `name()` when the method declares no component.
// Unanchored, so `x = variables.controller.getRequestContext()` types x too;
// the match is still the whole remainder, so a longer name or a further hop
// is not claimed.
func returnResolver(component string, names ...string) []Resolver {
	out := make([]Resolver, 0, len(names))
	for _, n := range names {
		out = append(out, Resolver{Match: n + "()", Resolve: component, Prefix: n, DynamicIfMissing: true})
	}

	return out
}

// idResolver types what a factory call returns by the id it is handed:
// `getInstance( "UserService@users" )` is UserService as the users module
// registers it, and a bare id the component found by path and, failing that,
// by file name — nearest the calling file first — as an injected property's id
// is (parser.injectedComponent, resolve.wireboxID). The id is a
// literal, optionally named and optionally followed by further arguments;
// a computed one is left alone, and so is a call chained on the result,
// since the match runs to the end. It may start after a dot rather than at
// the prefix, which is found first inside getBeanFactory().getBean( "x" ).
func idResolver(fn string) Resolver {
	return Resolver{
		Match:            `(?i)(?:^|\.)` + fn + `\(\s*(?:\w+\s*[=:]\s*)?["'](?:id:|model:)?([A-Za-z_][\w.]*(?:@[\w.-]+)?)@?["']\s*(?:,[^()]*)?\)$`,
		Resolve:          "$1",
		Prefix:           fn,
		DynamicIfMissing: true,
	}
}

// dslResolvers type `getInstance( "wirebox:populator" )` and the rest of the
// WireBox DSL a getInstance() may be handed in place of an id, as an
// injection is (parser.InjectionDSL). They go before idResolver, which would
// read `getInstance( "logbox" )` as a component called logbox.
func dslResolvers() []Resolver {
	dsl := parser.InjectionDSL()
	out := make([]Resolver, 0, len(dsl))

	for _, k := range slices.Sorted(maps.Keys(dsl)) {
		out = append(out, Resolver{
			Match:            `(?i)^getInstance\(\s*(?:\w+\s*[=:]\s*)?["']` + regexp.QuoteMeta(k) + `["']\s*\)$`,
			Resolve:          dsl[k],
			Prefix:           "getInstance",
			DynamicIfMissing: true,
		})
	}

	return out
}

const (
	coldboxSystem    = "coldbox.system."
	testboxSystem    = "testbox.system."
	commandboxSystem = "commandbox.system."
	qbModels         = "qb.models."
	contentboxModels = "contentbox.models."
)

// wheelsView is what a Wheels view runs inside: the controller rendering it,
// which integrates every component of wheels.view and wheels.controller when
// it starts. No one component declares linkTo(), so the base is the
// controller and the mixins a view calls, tried in turn.
var wheelsView = strings.Join([]string{
	"wheels.Controller",
	"wheels.view.links", "wheels.view.forms", "wheels.view.formsobject",
	"wheels.view.formsplain", "wheels.view.formsassociation", "wheels.view.formsdate",
	"wheels.view.formsdateobject", "wheels.view.formsdateplain", "wheels.view.miscellaneous",
	"wheels.view.assets", "wheels.view.csrf", "wheels.view.errors", "wheels.view.pagination",
	"wheels.view.sanitize",
	"wheels.controller.rendering", "wheels.controller.flash", "wheels.controller.miscellaneous",
}, "|")

var frameworkPresets = map[string]frameworkPreset{
	// ColdBox: what FrameworkSupertype injects (controller, log, wirebox,
	// cachebox, flash, logbox), the request context every handler, view and
	// interceptor is handed, and the HTML helper views use. Handlers,
	// interceptors, the router and the scheduler may omit their base; views and
	// layouts run inside the Renderer.
	"coldbox": {
		resolvers: slices.Concat(
			[]Resolver{
				variableResolver(coldboxSystem+"web.context.RequestContext", "event"),
				variableResolver(coldboxSystem+"modules.HTMLHelper.models.HTMLHelper", "html"),
				variableResolver(coldboxSystem+"web.Controller", "controller", "cbController", "coldbox"),
				variableResolver(coldboxSystem+"logging.Logger", "log"),
				variableResolver(coldboxSystem+"logging.LogBox", "logbox"),
				variableResolver(coldboxSystem+"ioc.Injector", "wirebox", "injector"),
				variableResolver(coldboxSystem+"ioc.config.Binder", "binder"),
				variableResolver(coldboxSystem+"cache.CacheFactory", "cachebox"),
				variableResolver(coldboxSystem+"web.flash.AbstractFlashScope", "flash"),
			},
			returnResolver(coldboxSystem+"web.context.RequestContext", "getRequestContext"),
			returnResolver(coldboxSystem+"web.Controller", "getController"),
			returnResolver(coldboxSystem+"logging.Logger", "getLogger"),
			returnResolver(coldboxSystem+"logging.LogBox", "getLogBox"),
			returnResolver(coldboxSystem+"ioc.Injector", "getWireBox", "getInjector"),
			returnResolver(coldboxSystem+"ioc.config.Binder", "getBinder"),
			returnResolver(coldboxSystem+"cache.CacheFactory", "getCacheBox"),
			returnResolver(coldboxSystem+"web.flash.AbstractFlashScope", "getFlash"),
			returnResolver(coldboxSystem+"web.services.InterceptorService", "getInterceptorService"),
			returnResolver(coldboxSystem+"web.Renderer", "getRenderer"),
			returnResolver(coldboxSystem+"web.services.RequestService", "getRequestService"),
			returnResolver(coldboxSystem+"web.services.RoutingService", "getRoutingService"),
			returnResolver(coldboxSystem+"web.services.HandlerService", "getHandlerService"),
			returnResolver(coldboxSystem+"web.services.ModuleService", "getModuleService"),
			returnResolver(coldboxSystem+"web.services.LoaderService", "getLoaderService"),
			returnResolver(coldboxSystem+"web.services.SchedulerService", "getSchedulerService"),
			returnResolver(coldboxSystem+"async.AsyncManager", "getAsyncManager"),
			returnResolver(coldboxSystem+"core.conversion.DataMarshaller", "getDataMarshaller"),
			returnResolver(coldboxSystem+"web.context.Response", "getResponse"),
			returnResolver(coldboxSystem+"core.util.Util", "getUtil"),
			// Declared, but as a bare word, which the resolver does not take
			// for a component: ColdBoxScheduledTask function task( name ).
			returnResolver(coldboxSystem+"web.tasks.ColdBoxScheduledTask", "task"),
			// WireBox builds what an id names, from a handler, a model, a
			// test or the injector itself.
			dslResolvers(),
			[]Resolver{idResolver("getInstance")},
		),
		bases: []implicitBase{
			{dir: "handlers", ext: ".cfc", component: coldboxSystem + "EventHandler"},
			{dir: "interceptors", ext: ".cfc", component: coldboxSystem + "Interceptor"},
			{dir: "config", file: "Router.cfc", ext: ".cfc", component: coldboxSystem + "web.routing.Router"},
			{dir: "config", file: "Scheduler.cfc", ext: ".cfc", component: coldboxSystem + "web.tasks.ColdBoxScheduler"},
			{dir: "views", ext: ".cfm", component: coldboxSystem + "web.Renderer"},
			{dir: "layouts", ext: ".cfm", component: coldboxSystem + "web.Renderer"},
		},
		// ColdBox's application helpers — its own config, each module's
		// ModuleConfig, and a view's own helpers — reach these.
		helperDirs: []string{"handlers", "interceptors", "views", "layouts"},
	},

	// TestBox: the assertion and MockBox objects a spec is handed, a
	// reporter's results and runner, and what the spec DSL returns. Specs
	// extend BaseSpec themselves, so there is no implied base.
	"testbox": {
		resolvers: slices.Concat(
			[]Resolver{
				variableResolver(testboxSystem+"Assertion", "assert", "$assert", "assertions"),
				variableResolver(testboxSystem+"MockBox", "mockbox", "$mockbox"),
				variableResolver(testboxSystem+"TestBox", "testbox"),
				variableResolver(testboxSystem+"TestResult", "results", "testResults"),
			},
			returnResolver(testboxSystem+"MockBox", "getMockBox"),
			returnResolver(testboxSystem+"Expectation", "expect"),
			returnResolver(testboxSystem+"CollectionExpectation", "expectAll"),
		),
	},

	// CommandBox: the print buffer every command and task writes to, and the
	// DSL command() returns — not task()'s: a ColdBox scheduler's task() is a
	// different thing by the same name. A .cfc under commands/ is a command, and
	// task.cfc, or a .cfc under build/, a task runner.
	"commandbox": {
		resolvers: slices.Concat(
			[]Resolver{
				variableResolver(commandboxSystem+"util.PrintBuffer", "print"),
			},
			returnResolver(commandboxSystem+"util.CommandDSL", "command"),
			returnResolver(commandboxSystem+"util.PrintBuffer", "getPrint"),
		),
		bases: []implicitBase{
			{dir: "commands", ext: ".cfc", component: commandboxSystem + "BaseCommand"},
			{file: "task.cfc", ext: ".cfc", component: commandboxSystem + "BaseTask"},
			{dir: "build", ext: ".cfc", component: commandboxSystem + "BaseTask"},
		},
	},

	// cfmigrations, and the qb it builds on: a migration's up( schema, qb ) —
	// or ( schema, query ) — and the table a schema.create() callback is
	// handed. These are common names, which is why they are a preset and not a
	// default: name it only in a project that uses cfmigrations.
	"cfmigrations": {
		resolvers: []Resolver{
			variableResolver(qbModels+"Schema.SchemaBuilder", "schema"),
			variableResolver(qbModels+"Schema.Blueprint", "table"),
			variableResolver(qbModels+"Query.QueryBuilder", "qb", "query"),
		},
	},

	// ContentBox, on top of the coldbox preset: the CB helper its themes and
	// views call, and what its request interceptors put in prc.
	"contentbox": {
		resolvers: []Resolver{
			variableResolver(contentboxModels+"system.CBHelper", "cb"),
			variableResolver(contentboxModels+"security.Author", "prc.oCurrentAuthor", "prc.oAuthor"),
			variableResolver(contentboxModels+"system.Site", "prc.oCurrentSite", "prc.oSite"),
			// Whichever kind of content the request is about: a method only a
			// page has, hasParent(), is not missing from an entry's content.
			variableResolver(contentboxModels+"content.BaseContent|"+contentboxModels+"content.Entry|"+
				contentboxModels+"content.Page|"+contentboxModels+"content.ContentStore", "prc.oContent"),
		},
	},

	// Wheels 3: the global object in application scope, and a view rendered by
	// its controller, mixins included.
	"wheels": {
		resolvers: []Resolver{
			variableResolver("wheels.Global", "application.wo"),
			// model( "User" ) is the class in the application's models
			// directory: by file name, nearest the calling file, which is how
			// a test's own models are found before the application's.
			idResolver("model"),
		},
		bases: []implicitBase{
			{dir: "views", ext: ".cfm", component: wheelsView},
			{dir: "layouts", ext: ".cfm", component: wheelsView},
		},
		// The application's global/functions.cfm reaches every controller,
		// model and view (resolve.wheelsGlobals).
		helperDirs: []string{"controllers", "models", "views", "layouts"},
	},

	// FW/1: the framework object controllers are handed, its bean factory, and
	// views and layouts, which framework.one includes.
	"fw1": {
		resolvers: []Resolver{
			variableResolver("framework.one", "fw", "framework"),
			variableResolver("framework.ioc", "beanFactory"),
			// DI/1's getBean( "userService" ): a bean by the name it
			// registers, file name or file name and folder
			// (resolve.componentPathUncached, cfpath.BuildBeanMap).
			idResolver("getBean"),
		},
		// A view or layout runs inside the framework object, and an
		// Application.cfc that extends the framework is that object, so its
		// own functions are callable from the view. Mura's admin embeds FW/1
		// 1.x as admin/framework.cfc and declares rbKey() in the Application.
		bases: []implicitBase{
			{dir: "views", ext: ".cfm", component: parser.ApplicationBase + "framework.one"},
			{dir: "layouts", ext: ".cfm", component: parser.ApplicationBase + "framework.one"},
		},
	},
}

// KnownFrameworks lists the names `frameworks` accepts, sorted.
func KnownFrameworks() []string {
	names := make([]string, 0, len(frameworkPresets))
	for n := range frameworkPresets {
		names = append(names, n)
	}

	slices.Sort(names)

	return names
}

// UnknownFrameworks returns the names in frameworks that no preset answers to,
// so a misspelling is reported rather than silently doing nothing.
func UnknownFrameworks(frameworks []string) []string {
	var out []string

	for _, f := range frameworks {
		if _, ok := frameworkPresets[strings.ToLower(f)]; !ok {
			out = append(out, f)
		}
	}

	return out
}

// FrameworkResolvers returns the component resolvers the named frameworks add,
// in the order the frameworks are named. They go after a config's own.
func FrameworkResolvers(frameworks []string) []Resolver {
	lists := make([][]Resolver, 0, len(frameworks))
	for _, f := range frameworks {
		lists = append(lists, frameworkPresets[strings.ToLower(f)].resolvers)
	}

	return slices.Concat(lists...)
}

// HelperScope returns whether the named frameworks mix their helper templates
// into the file at path, or nil when none does: a file under one of a
// preset's helper directories, at any depth.
func HelperScope(frameworks []string) func(path string) bool {
	lists := make([][]string, 0, len(frameworks))
	for _, f := range frameworks {
		lists = append(lists, frameworkPresets[strings.ToLower(f)].helperDirs)
	}

	dirs := slices.Concat(lists...)
	if len(dirs) == 0 {
		return nil
	}

	return func(path string) bool {
		for d := range strings.SplitSeq(filepath.ToSlash(filepath.Dir(path)), "/") {
			if slices.ContainsFunc(dirs, func(h string) bool { return strings.EqualFold(h, d) }) {
				return true
			}
		}

		return false
	}
}

// PresetComponents lists the components a preset names — what its
// resolvers resolve to and the bases it implies — each once, in the order
// written. The stub generator (cmd/cfstubgen) starts from these, so a
// framework whose source is not in the workspace can still be checked.
func PresetComponents(name string) []string {
	p, ok := frameworkPresets[strings.ToLower(name)]
	if !ok {
		return nil
	}

	var out []string

	seen := map[string]bool{}
	add := func(list string) {
		for c := range strings.SplitSeq(list, "|") {
			if c != "" && !strings.Contains(c, "$") && !seen[strings.ToLower(c)] {
				seen[strings.ToLower(c)] = true
				out = append(out, c)
			}
		}
	}

	for i := range p.resolvers {
		add(p.resolvers[i].Resolve)
	}

	for _, b := range p.bases {
		add(b.component)
	}

	return out
}

// ImplicitExtends returns the rule the named frameworks set for a file that
// extends nothing: the component it behaves as if it extended, or "". It is
// nil when no framework sets one, so a resolver without presets pays nothing.
//
// A rule matches a directory anywhere in the path — a module's own
// modules/blog/handlers is as much a handlers directory as the application's
// — and the deepest matching directory wins, so views/handlers/x.cfm is a view.
func ImplicitExtends(frameworks []string) func(path string) string {
	lists := make([][]implicitBase, 0, len(frameworks))
	for _, f := range frameworks {
		lists = append(lists, frameworkPresets[strings.ToLower(f)].bases)
	}

	bases := slices.Concat(lists...)
	if len(bases) == 0 {
		return nil
	}

	return func(path string) string {
		ext := strings.ToLower(filepath.Ext(path))
		name := filepath.Base(path)
		dirs := strings.Split(filepath.ToSlash(filepath.Dir(path)), "/")

		for _, dir := range slices.Backward(dirs) {
			for _, b := range bases {
				if b.ext == ext && (b.dir == "" || strings.EqualFold(b.dir, dir)) && (b.file == "" || strings.EqualFold(b.file, name)) {
					return b.component
				}
			}
		}

		return ""
	}
}

// boxDependencyPresets maps a box.json dependency, or a package's own slug,
// to the preset it implies. Only names that say which framework the code is
// written against: a commandbox-cfformat dev dependency says nothing about
// whether the project has commands or task runners, so commandbox is not here.
var boxDependencyPresets = map[string]string{
	"coldbox":               "coldbox",
	"testbox":               "testbox",
	"cfmigrations":          "cfmigrations",
	"commandbox-migrations": "cfmigrations",
	"contentbox":            "contentbox",
	"wheels-core":           "wheels",
	"cfwheels":              "wheels",
	"wheels":                "wheels",
	"fw1":                   "fw1",
}

// SuggestFrameworks names the presets a box.json implies that frameworks
// does not already list, sorted: what `unresolved` suggests and the server
// logs, so a project that would benefit from a preset hears about it. Nothing
// is turned on by it — a preset types variables by name, which is a project's
// call to make. Unreadable JSON suggests nothing.
func SuggestFrameworks(boxJSON []byte, frameworks []string) []string {
	var box struct {
		Slug            string            `json:"slug"`
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}

	if json.Unmarshal(boxJSON, &box) != nil {
		return nil
	}

	names := slices.Concat([]string{box.Slug}, slices.Collect(maps.Keys(box.Dependencies)), slices.Collect(maps.Keys(box.DevDependencies)))

	var out []string

	for _, n := range names {
		p, ok := boxDependencyPresets[strings.ToLower(n)]
		if !ok || slices.Contains(out, p) || slices.ContainsFunc(frameworks, func(f string) bool { return strings.EqualFold(f, p) }) {
			continue
		}

		out = append(out, p)
	}

	slices.Sort(out)

	return out
}
