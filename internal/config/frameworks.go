package config

import (
	"path/filepath"
	"slices"
	"strings"
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
}

// implicitBase is the component a file extends when it names none: a
// ColdBox handler with no extends attribute is an EventHandler all the same,
// and a view is rendered from inside the Renderer, so a bare view() or
// getInstance() in either is the framework's.
type implicitBase struct {
	dir       string // a directory of the file's path, at any depth
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
		alts = append(alts, strings.ReplaceAll(n, ".", `\.`))
		prefixes = append(prefixes, n, "variables."+n, "arguments."+n)
	}

	return Resolver{
		Match:            `^(?:variables\.|arguments\.)?(?:` + strings.Join(alts, "|") + `)$`,
		Resolve:          component,
		Prefix:           strings.Join(prefixes, "|"),
		Anchored:         true,
		DynamicIfMissing: true,
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

const coldboxSystem = "coldbox.system."

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
		),
		bases: []implicitBase{
			{dir: "handlers", ext: ".cfc", component: coldboxSystem + "EventHandler"},
			{dir: "interceptors", ext: ".cfc", component: coldboxSystem + "Interceptor"},
			{dir: "config", file: "Router.cfc", ext: ".cfc", component: coldboxSystem + "web.routing.Router"},
			{dir: "config", file: "Scheduler.cfc", ext: ".cfc", component: coldboxSystem + "web.tasks.ColdBoxScheduler"},
			{dir: "views", ext: ".cfm", component: coldboxSystem + "web.Renderer"},
			{dir: "layouts", ext: ".cfm", component: coldboxSystem + "web.Renderer"},
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
				if b.ext == ext && strings.EqualFold(b.dir, dir) && (b.file == "" || strings.EqualFold(b.file, name)) {
					return b.component
				}
			}
		}

		return ""
	}
}
