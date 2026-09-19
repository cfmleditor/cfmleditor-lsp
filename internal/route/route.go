// Package route resolves convention-based framework routes — the dotted strings a
// dispatcher turns into a component, a method and a view file — to the code they
// actually reach.
//
// Static analysis cannot follow a dispatcher. A framework reads a route out of a
// URL or an HTML attribute, builds a component path and a method name from it, and
// invokes them; nothing in the source names either, so every controller method in
// a routed application looks uncalled and every view looks unreferenced. On one
// real workspace that pattern accounted for most of what a call graph reported as
// orphaned.
//
// The convention is configuration, not code. FW/1 maps "section.item" to
// controllers/section.cfc's item() and views/section/item.cfm; TASS maps
// "kiosk.customrollcall.dialog.custom.roll" to packages/tass/kiosk-customrollcall.cfc's
// dialogCustomRoll(). Those are the same shape with different templates, and a
// third framework will be a third set — so the rules live in .cfmleditor.json and
// this package only knows how to apply them.
package route

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// Config is the "routes" block of .cfmleditor.json.
type Config struct {
	// Attributes are the HTML attribute names that carry a route, e.g. "data-view",
	// "data-read" and "data-process". Matching is case-insensitive.
	Attributes []string `json:"attributes,omitempty"`

	// QueryParams are URL parameter names that carry a route, e.g. "do" for links
	// written href="index.cfm?do=kiosk.lms.main.learningobjects". Such a value sits
	// inside the href's own quotes, so it runs to the next URL delimiter rather
	// than to a quote.
	QueryParams []string `json:"queryParams,omitempty"`

	// Properties are JavaScript object keys that carry a route, e.g. "read",
	// "view" and "process" in { view: "ui.web.user.dialog.login" }. The key may be
	// quoted or bare.
	//
	// These are the loosest of the three, and the reason Plausible exists: "read"
	// and "view" are ordinary words, so scanning for them finds plenty of object
	// keys holding something that is not a route at all.
	Properties []string `json:"properties,omitempty"`

	// Functions are function names whose first string argument is a route, in
	// CFML or in JavaScript: redirect("intranet.ims.main.incidents"),
	// setPrint("fundraising.fundraising.event_print_action").
	//
	// A route passed this way often carries a query string or a fragment after it
	// — redirect('intranet.client.detail&customercode=x') — so the value is cut at
	// the first URL delimiter rather than rejected for containing one.
	Functions []string `json:"functions,omitempty"`

	// Aliases rewrite a leading portion of a route before any rule is tried. TASS
	// spells "the product this page is being served by" as "ui.web", and a view
	// shared between products resolves into whichever one is running — so an alias
	// maps to *several* replacements and every one is a candidate.
	Aliases map[string][]string `json:"aliases,omitempty"`

	// Controllers are tried in order; the first whose component resolves to a file
	// that declares the method wins.
	Controllers []ControllerRule `json:"controllers,omitempty"`

	// Views are tried in order, after the controllers.
	Views []ViewRule `json:"views,omitempty"`
}

// ControllerRule maps a route onto a component and a method.
type ControllerRule struct {
	// Component is a dot-path template, e.g. "packages.tass.${1}-${2}".
	Component string `json:"component"`

	// Method is a template for the method name, e.g. "${3+:concat}".
	Method string `json:"method"`
}

// ViewRule maps a route onto a file.
type ViewRule struct {
	// Path is a template for the file, relative to Root, e.g. "views/${1}/${2}.cfm".
	// Leave it empty and set LongestDir instead when the split between directory
	// and file name is not at a fixed segment.
	Path string `json:"path,omitempty"`

	// LongestDir resolves by finding the longest leading run of segments that names
	// a real directory and treating the remainder as a dotted file name:
	// "ui.web.general.popup.lookup.filter" is ui/web/general/popup.lookup.filter.cfm,
	// and no template can express that because the split depends on what is on disk.
	LongestDir bool `json:"longestDir,omitempty"`

	// Ext is the file extension to try, ".cfm" by default. Several may be listed.
	Ext []string `json:"ext,omitempty"`

	// Root is a directory the path is resolved under, relative to the workspace.
	Root string `json:"root,omitempty"`
}

// Kind says what a route resolved to.
type Kind string

// The kinds of thing a route can name.
const (
	// KindController is a component and a method on it.
	KindController Kind = "controller"

	// KindView is a template file.
	KindView Kind = "view"
)

// Target is one resolution of a route.
type Target struct {
	Kind      Kind
	Component string // dot-path, for a controller
	Method    string // method name, for a controller
	Path      string // absolute file path, for either
	Rule      int    // index of the rule that matched, for diagnostics
	Alias     string // the alias expansion used, if any
}

// segRef is one ${...} reference in a template.
type segRef struct {
	from   int    // 1-based first segment
	to     int    // last segment, or 0 for "to the end"
	join   string // "." (default), "" for concat, "/" for slash
	fold   string // "", "lower", "upper"
	search bool   // try every later start point too, longest first
}

// tmplRx matches ${N}, ${N+}, ${N-M}, each with an optional :mode suffix.
var tmplRx = regexp.MustCompile(`\$\{(\d+)(\+|-\d+)?(?::([a-z]+))?\}`)

// expandAll returns every candidate a template produces, most specific first.
//
// Only :search yields more than one. A route's segments do not say where the
// controller's name stops and the method's begins: TASS spells
// "kiosk.student.main.student" as both studentMainStudent() and mainStudent()
// depending on the controller, and the difference is not visible in the route.
// Enumerating start points is how a single rule covers both, instead of one
// hand-written rule per start point — and because each candidate is still checked
// against the component's real methods, a wrong guess resolves to nothing rather
// than to an edge.
//
// Longest first, so the most specific name wins: studentmainstudent before
// mainstudent before student. A short trailing segment like "read" or "view"
// would otherwise match a common method on entirely the wrong route.
func expandAll(tmpl string, segs []string) []string {
	ref, ok := searchRef(tmpl)
	if !ok {
		if v, ok := expand(tmpl, segs); ok {
			return []string{v}
		}

		return nil
	}

	var out []string

	for from := ref.from; from <= len(segs); from++ {
		shifted := segRef{from: from, to: 0, join: ref.join, fold: ref.fold}

		v, valid := shifted.apply(segs)
		if !valid || v == "" {
			continue
		}

		out = append(out, v)
	}

	return out
}

// searchRef reports the reference when a template is exactly one :search ref and
// nothing else. A :search mixed into a larger template is not supported, because
// the candidates would multiply against whatever else the template holds for no
// use anyone has asked for.
func searchRef(tmpl string) (segRef, bool) {
	m := tmplRx.FindStringSubmatchIndex(tmpl)
	if m == nil || m[0] != 0 || m[1] != len(tmpl) {
		return segRef{}, false
	}

	ref, err := parseRef(tmpl[m[2]:m[3]], group(tmpl, m, 4), group(tmpl, m, 6))
	if err != nil || !ref.search {
		return segRef{}, false
	}

	return ref, true
}

// expand fills a template from the route's segments. It reports false when the
// template references a segment the route does not have, which is how a rule
// declines a route that is too short for it rather than producing a truncated
// component path that happens to resolve to something else.
func expand(tmpl string, segs []string) (string, bool) {
	var (
		out strings.Builder
		ok  = true
		at  = 0
	)

	for _, m := range tmplRx.FindAllStringSubmatchIndex(tmpl, -1) {
		out.WriteString(tmpl[at:m[0]])
		at = m[1]

		ref, err := parseRef(tmpl[m[2]:m[3]], group(tmpl, m, 4), group(tmpl, m, 6))
		if err != nil {
			return "", false
		}

		value, valid := ref.apply(segs)
		if !valid {
			ok = false

			break
		}

		out.WriteString(value)
	}

	if !ok {
		return "", false
	}

	out.WriteString(tmpl[at:])

	return out.String(), true
}

func group(s string, m []int, i int) string {
	if m[i] < 0 {
		return ""
	}

	return s[m[i]:m[i+1]]
}

func parseRef(first, span, mode string) (segRef, error) {
	from, err := strconv.Atoi(first)
	if err != nil || from < 1 {
		return segRef{}, fmt.Errorf("bad segment index %q", first)
	}

	ref := segRef{from: from, to: from, join: "."}

	switch {
	case span == "+":
		ref.to = 0
	case strings.HasPrefix(span, "-"):
		to, err := strconv.Atoi(span[1:])
		if err != nil || to < from {
			return segRef{}, fmt.Errorf("bad segment span %q", span)
		}

		ref.to = to
	}

	switch mode {
	case "", "dotted":
	case "concat":
		ref.join = ""
	case "slash":
		ref.join = "/"
	case "lower", "upper":
		ref.fold = mode
	case "search":
		// Only meaningful on an open-ended ${N+}: it is the span that varies.
		if ref.to != 0 {
			return segRef{}, fmt.Errorf("mode :search needs an open span like ${%d+}", from)
		}

		ref.join = ""
		ref.search = true
	default:
		return segRef{}, fmt.Errorf("unknown segment mode %q", mode)
	}

	return ref, nil
}

func (r segRef) apply(segs []string) (string, bool) {
	if r.from > len(segs) {
		return "", false
	}

	end := r.to
	if end == 0 || end > len(segs) {
		end = len(segs)
	}

	if end < r.from {
		return "", false
	}

	value := strings.Join(segs[r.from-1:end], r.join)

	switch r.fold {
	case "lower":
		value = strings.ToLower(value)
	case "upper":
		value = strings.ToUpper(value)
	}

	return value, true
}

// Split breaks a route into its segments, lower-cased.
//
// Routes are matched case-insensitively throughout, as CFML matches everything
// else: a route written data-view="UI.Web.General" names the same thing as
// data-view="ui.web.general", and treating them as different would make
// resolution depend on how a page happened to be typed.
func Split(route string) []string {
	route = strings.TrimSpace(strings.ToLower(route))
	if route == "" {
		return nil
	}

	segs := strings.Split(route, ".")
	for i := range segs {
		segs[i] = strings.TrimSpace(segs[i])
		if segs[i] == "" {
			return nil
		}
	}

	return segs
}

// Plausible reports whether a string looks like a route at all.
//
// The attributes these come from carry other things too — a panel id, a CSS hook,
// a string with a runtime #expression# in it — and running every one of them
// through resolution produces confident nonsense rather than nothing. Two dotted
// segments of identifier characters is the floor.
func Plausible(route string) bool {
	if strings.ContainsAny(route, "#<>\"' \t") {
		return false
	}

	segs := Split(route)
	if len(segs) < 2 {
		return false
	}

	for _, s := range segs {
		for _, c := range s {
			switch {
			case c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '_', c == '-':
			default:
				return false
			}
		}
	}

	return true
}

// expansion is one candidate reading of a route: its segments, and which alias
// replacement produced them.
type expansion struct {
	alias string
	segs  []string
}

// expansions returns the readings to try: the route as written, then one per
// alias replacement whose key matches a leading run of segments.
//
// Alias keys are sorted longest-first and then lexically. Sorting at all is the
// point: ranging over the map directly made the winner depend on Go's randomised
// map order, so the same route resolved to different components between runs of
// the same build. Longest-first means a more specific alias ("ui.web.lab") beats
// a broader one ("ui.web") that also matches, and the replacements inside one
// alias stay in the order the config lists them, which is the author's priority.
func (c Config) expansions(segs []string) []expansion {
	out := []expansion{{segs: segs}}

	keys := make([]string, 0, len(c.Aliases))
	for k := range c.Aliases {
		keys = append(keys, k)
	}

	sort.Slice(keys, func(i, j int) bool {
		a, b := Split(keys[i]), Split(keys[j])
		if len(a) != len(b) {
			return len(a) > len(b)
		}

		return keys[i] < keys[j]
	})

	for _, key := range keys {
		keySegs := Split(key)
		if len(keySegs) == 0 || len(keySegs) > len(segs) {
			continue
		}

		if !slices.Equal(keySegs, segs[:len(keySegs)]) {
			continue
		}

		for _, rep := range c.Aliases[key] {
			repSegs := Split(rep)
			if len(repSegs) == 0 {
				continue
			}

			expanded := make([]string, 0, len(repSegs)+len(segs)-len(keySegs))
			expanded = append(expanded, repSegs...)
			expanded = append(expanded, segs[len(keySegs):]...)

			out = append(out, expansion{alias: key + " -> " + rep, segs: expanded})
		}
	}

	return out
}
