package resolve

import (
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/conv"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
)

// A template runs where it is included: in its includer's variables scope,
// and inside the includer's function when the include is written in one, so
// an unscoped name the template reads but never sets is whatever the includer
// holds by that name at the include. Mura's configBean.applyDbUpdates declares
// `var dbUtility = getBean("dbUtility")` and includes every dbUpdates/*.cfm,
// each of which calls dbUtility.setTable() and nothing else gives it a type.
//
// includerHeld asks each include site that reaches the template what the name
// holds at that line, as any receiver is asked. Every site must answer; the
// answer is their union. A template that assigns or declares the name itself
// is not asked about, since its own value may be the one read.

const maxIncluderSites = 16

func (r *Resolver) includerHeld(variable string, pr *parser.ParseResult, tr *callTrace, ctx lookupCtx) string {
	if pr == nil || pr.URI == "" || ctx.depth >= maxAssignedDepth {
		return ""
	}

	name := variable
	if rest, ok := strings.CutPrefix(strings.ToLower(variable), "local."); ok {
		name = variable[len(variable)-len(rest):]
	}

	// A name, or a member of one: an FW/1 view's partial reads rc.contentBean
	// as the view including it holds it.
	base, member, dotted := strings.Cut(name, ".")
	if base == "" || strings.ContainsAny(name, "[(") || isCFMLScope(base) || dotted && (member == "" || strings.Contains(member, ".")) {
		return ""
	}

	g := r.includes()
	if g == nil {
		return ""
	}

	file := cfpath.FromURI(string(pr.URI))

	extra := r.frameworkIncludeHosts(file)
	if len(g.rev[pathKey(file)]) == 0 && len(extra) == 0 {
		return ""
	}

	// The answer does not depend on where in the template the name is read,
	// and a template's untyped names are read once per call made on them.
	key := pathKey(file) + "\x00" + strings.ToLower(variable) + "\x00" + strconv.Itoa(ctx.depth) + "\x00" + strconv.FormatUint(g.gen, 10)

	r.mu.RLock()
	cached, hit := r.includerCache[key]
	r.mu.RUnlock()

	if hit {
		if cached != "" {
			tr.addf("resolved %q to %q: what every file including this one holds by that name at the include", variable, cached)
		}

		return cached
	}

	answer := ""
	if !setsName(pr.Content, name) {
		answer = r.includerHeldUncached(variable, file, g, extra, ctx)
	}

	r.mu.Lock()
	if r.includerCache == nil || len(r.includerCache) >= 8192 {
		r.includerCache = map[string]string{}
	}

	r.includerCache[key] = answer
	r.mu.Unlock()

	if answer != "" {
		tr.addf("resolved %q to %q: what every file including this one holds by that name at the include", variable, answer)
	}

	return answer
}

func (r *Resolver) includerHeldUncached(variable, file string, g *includeGraph, extra []includeHost, ctx lookupCtx) string {
	includers := g.rev[pathKey(file)]

	if len(includers) == 0 && len(extra) == 0 {
		return ""
	}

	var (
		comps []string
		sites int
	)

	for _, k := range includers {
		path := g.paths[k]

		hpr := r.handlerParse(path)
		if hpr == nil {
			return ""
		}

		dir := filepath.Dir(path)

		for _, site := range parser.IncludeSites(hpr.Content) {
			if !slices.ContainsFunc(r.includeTargets(site.Path, path), func(t string) bool { return cfpath.SamePath(t, file) }) {
				continue
			}

			sites++
			if sites > maxIncluderSites {
				return ""
			}

			line := uint32(strings.Count(hpr.Content[:site.Offset], "\n")) //nolint:gosec // a file offset's line count is non-negative
			caller := parser.FindFuncScopeAt(int(line), hpr.Scopes).Name

			comp, _ := r.receiverComponentD(variable, line, caller, "", hpr, dir, nil, lookupCtx{depth: ctx.depth + 1})
			if comp == "" || strings.HasPrefix(comp, "$") {
				return ""
			}

			for alt := range strings.SplitSeq(r.pathsOf(comp, dir), "|") {
				if !containsFold(comps, alt) {
					comps = append(comps, alt)
				}
			}
		}
	}

	// A framework's own computed include (coldboxErrorHosts).
	for _, h := range extra {
		hpr := r.handlerParse(h.path)
		if hpr == nil {
			return ""
		}

		sites++

		line := conv.Uint32(h.line)
		caller := parser.FindFuncScopeAt(h.line, hpr.Scopes).Name

		comp, _ := r.receiverComponentD(variable, line, caller, "", hpr, filepath.Dir(h.path), nil, lookupCtx{depth: ctx.depth + 1})
		if comp == "" || strings.HasPrefix(comp, "$") {
			return ""
		}

		for alt := range strings.SplitSeq(r.pathsOf(comp, filepath.Dir(h.path)), "|") {
			if !containsFold(comps, alt) {
				comps = append(comps, alt)
			}
		}
	}

	if sites == 0 {
		return ""
	}

	slices.Sort(comps)

	return strings.Join(comps, "|")
}

// setsName reports whether content assigns or declares name anywhere: an
// assignment, a var, a loop variable, a cfset, a function's argument, or a
// tag attribute naming it as the variable it fills.
func setsName(content, name string) bool {
	q := regexp.QuoteMeta(name)
	re := regexp.MustCompile(`(?i)(?:^|[^\w.$])(?:local\.|variables\.)?` + q + `\s*(?:[-+*/&]?=[^=]|\+\+|--)` +
		`|\bvar\s+` + q + `\b` +
		`|\bfor\s*\(\s*(?:var\s+)?` + q + `\s+in\b` +
		`|\b(?:item|index|name|returnvariable|variable|result)\s*=\s*["']` + q + `["']`)

	return re.MatchString(content)
}

// isCFMLScope reports whether s names one of CFML's own scopes. An FW/1 or
// ColdBox view's rc and prc are not among them: they are variables the view
// holds, and a template it includes reads them.
func isCFMLScope(s string) bool {
	switch strings.ToLower(s) {
	case "local", "variables", "this", "arguments", "session", "application", "request", "server", "url", "form", "cgi", "cookie", "client":
		return true
	default:
		return false
	}
}

// frameworkIncludeHosts are the include sites the include graph cannot see
// for file: ColdBox's error template and a computed include naming the file's
// directory. Cached per file, since includerHeld asks for every untyped name
// a template reads, and finding them reads files.
func (r *Resolver) frameworkIncludeHosts(file string) []includeHost {
	o := r.owner()
	key := pathKey(file)

	o.mu.RLock()
	hosts, ok := o.extraIncludeHosts[key]
	o.mu.RUnlock()

	if ok {
		return hosts
	}

	hosts = append(r.coldboxErrorHosts(file), r.computedIncludeHosts(file)...)

	o.mu.Lock()
	if o.extraIncludeHosts == nil {
		o.extraIncludeHosts = map[string][]includeHost{}
	}

	o.extraIncludeHosts[key] = hosts
	o.mu.Unlock()

	return hosts
}
