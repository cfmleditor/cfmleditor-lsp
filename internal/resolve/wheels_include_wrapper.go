package resolve

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
)

// Wheels includes many of its templates through Global's wrappers rather than
// a cfinclude: public/Application.cfc runs
// `application.wo.$includeAndOutput( template = "/wheels/events/onrequestend/debug.cfm" )`,
// and the template's bare calls ($get, urlFor, capitalize) are Global's,
// since an include runs in the variables scope of the component whose method
// performs it. A wrapper call made bare from a component includes into that
// component.
//
// The wrappers are checked against Global's pinned bodies, and only a literal
// mapping-absolute path is read: $resolveGlobalIncludeTemplate returns such a
// path as it is, and $tryIncludeTemplate includes it.

var wheelsIncludeWrappers = map[string]string{
	"$include":                `$tryIncludeTemplate($resolveGlobalIncludeTemplate(arguments.template));`,
	"$includeandoutput":       `$tryIncludeTemplate($resolveGlobalIncludeTemplate(arguments.template));`,
	"$includeandreturnoutput": "",
}

// wheelsWrappedTemplateFunc is name as the component that includes file
// through a Global wrapper declares it, when every such include names a
// component that does.
func (r *Resolver) wheelsWrappedTemplateFunc(pr *parser.ParseResult, name string) *parser.FunctionDef {
	if !pr.URI.IsFile() || !strings.EqualFold(filepath.Ext(pr.URI.Path()), ".cfm") {
		return nil
	}

	hosts, ok := r.wheelsTemplateHosts(pr.URI.Path())
	if !ok {
		return nil
	}

	var found *parser.FunctionDef

	for _, host := range hosts {
		def := r.lookupFunc(host, name, 0)
		if def == nil {
			return nil
		}

		if found == nil {
			found = def
		}
	}

	return found
}

// wheelsTemplateHosts is the components file is included into through a
// Global wrapper, and false when a call names it but its host cannot be
// proven. It reads and tokenises every file calling a wrapper, so it is
// cached per template: asked once per bare call, it was half of a scan of
// cfwheels.
func (r *Resolver) wheelsTemplateHosts(file string) ([]string, bool) {
	o := r.owner()
	key := pathKey(file)

	o.mu.RLock()
	hosts, cached := o.wrapperHosts[key]
	o.mu.RUnlock()

	if !cached {
		hosts = r.computeWheelsTemplateHosts(file)

		o.mu.Lock()
		if o.wrapperHosts == nil {
			o.wrapperHosts = map[string][]string{}
		}

		o.wrapperHosts[key] = hosts
		o.mu.Unlock()
	}

	if slices.Contains(hosts, "") {
		return nil, false
	}

	return hosts, true
}

// computeWheelsTemplateHosts is wheelsTemplateHosts uncached; an "" among
// the hosts is a call whose host cannot be proven.
func (r *Resolver) computeWheelsTemplateHosts(file string) []string {
	hosts := []string{}

	for _, wrapper := range []string{"$include", "$includeAndOutput", "$includeAndReturnOutput"} {
		for _, caller := range r.callerFiles(wrapper) {
			for _, host := range r.wheelsWrapperHosts(caller, wrapper, file) {
				if host == "" {
					return []string{""}
				}

				if !containsFold(hosts, host) {
					hosts = append(hosts, host)
				}
			}
		}
	}

	return hosts
}

// wheelsWrapperHosts is, for each call in caller to wrapper that includes
// file, the component the include runs in: Global for application.wo, the
// caller itself for a bare call. "" is a call that names file but whose host
// cannot be proven.
func (r *Resolver) wheelsWrapperHosts(caller, wrapper, file string) []string {
	data, err := r.fs().ReadFile(caller)
	if err != nil {
		return nil
	}

	tokens := allTokens(string(data))

	var hosts []string

	for i := range tokens {
		if tokens[i].Kind != parser.TokIdent || !strings.EqualFold(tokens[i].Value, wrapper) || i+1 >= len(tokens) || tokens[i+1].Kind != parser.TokLParen {
			continue
		}

		literal, ok := wrapperTemplateLiteral(tokens, i+2)
		if !ok || !strings.HasPrefix(literal, "/") || !cfpath.SamePath(r.IncludePath(literal, caller), file) {
			continue
		}

		hosts = append(hosts, r.wheelsWrapperHost(tokens, i, caller, wrapper))
	}

	return hosts
}

// wheelsWrapperHost is the component a wrapper call at tokens[i] runs in.
func (r *Resolver) wheelsWrapperHost(tokens []parser.Token, i int, caller, wrapper string) string {
	dir := filepath.Dir(caller)

	var host string

	switch {
	case i >= 4 && tokens[i-1].Kind == parser.TokDot && strings.EqualFold(tokens[i-2].Value, "wo") &&
		tokens[i-3].Kind == parser.TokDot && strings.EqualFold(tokens[i-4].Value, "application"):
		host = r.ComponentPath("wheels.Global", dir)
	case i == 0 || tokens[i-1].Kind != parser.TokDot:
		if !strings.EqualFold(filepath.Ext(caller), ".cfc") {
			return ""
		}

		host = caller
	default:
		return ""
	}

	if host == "" {
		return ""
	}

	def := r.lookupFunc(host, wrapper, 0)
	if def == nil || !def.URI.IsFile() || !r.wheelsIncludeWrapperPinned(def.URI.Path(), wrapper) {
		return ""
	}

	return host
}

// wheelsIncludeWrapperPinned reports whether global declares wrapper, and the
// two methods it calls, as pinned.
func (r *Resolver) wheelsIncludeWrapperPinned(global, wrapper string) bool {
	methods := r.wheelsSource(global).methods

	resolveBody := methods["$resolveglobalincludetemplate"].body

	if !strings.HasPrefix(resolveBody, wheelsTokens(`var normalized = Replace(arguments.template, "\", "/", "all"); if (!Len(normalized)) { return normalized; } if (Left(normalized, 1) == "/") { return LCase(normalized); }`)) {
		return false
	}

	body := methods[strings.ToLower(wrapper)].body

	if pinned := wheelsIncludeWrappers[strings.ToLower(wrapper)]; pinned != "" {
		return body == wheelsTokens(pinned) &&
			strings.HasPrefix(methods["$tryincludetemplate"].body, wheelsTokens(`var resolved = arguments.template; var state = {done = false}; try { include "#resolved#"; state.done = true; }`))
	}

	return strings.Contains(body, wheelsTokens(`local.$resolved = $resolveGlobalIncludeTemplate(arguments.$template);`)) &&
		strings.Contains(body, wheelsTokens(`savecontent variable="$captured" { include "#local.$resolved#" };`))
}

// wrapperTemplateLiteral is the template a wrapper call's argument list,
// starting at tokens[j], names: its first argument or one named template or
// $template, when that is a string literal.
func wrapperTemplateLiteral(tokens []parser.Token, j int) (string, bool) {
	if j >= len(tokens) {
		return "", false
	}

	if tokens[j].Kind == parser.TokIdent && j+2 < len(tokens) && tokens[j+1].Kind == parser.TokEquals {
		if !strings.EqualFold(tokens[j].Value, "template") && !strings.EqualFold(tokens[j].Value, "$template") {
			return "", false
		}

		j += 2
	}

	if tokens[j].Kind != parser.TokString || j+1 >= len(tokens) || tokens[j+1].Kind != parser.TokRParen && tokens[j+1].Kind != parser.TokComma {
		return "", false
	}

	text := tokens[j].Value
	if len(text) < 2 || strings.Contains(text, "#") {
		return "", false
	}

	return text[1 : len(text)-1], true
}
