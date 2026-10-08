package resolve

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cfmleditor/clif/internal/conv"
	"github.com/cfmleditor/clif/internal/parser"
	cfpath "github.com/cfmleditor/clif/internal/path"
)

// An FW/1 view reads the rc its controller filled: views/<section>/<item>.cfm
// is rendered after controllers/<section>.cfc's <item>( rc ), which runs after
// its before( rc ), and Mura's admin views read rc.contentBean, rc.siteBean and
// the rest without a word about their type. So rc.X in such a view is what the
// item action assigns arguments.rc.X by its end, or, when it assigns nothing,
// what before() does; a controller action that calls setView("section.item")
// renders the view too, and every action must agree.
//
// The convention applies only where a framework preset gives views a base
// (ImplicitExtends), which is how the fw1 preset says views run inside the
// framework.

var fw1SetViewRe = regexp.MustCompile(`(?i)\bsetView\s*\(\s*["']([\w.]+)["']\s*\)`)

// fw1ViewRc is the component rc.X holds in the FW/1 view pr, or "".
func (r *Resolver) fw1ViewRc(variable, funcName string, pr *parser.ParseResult, tr *callTrace) string {
	scope, name, ok := strings.Cut(variable, ".")
	if !ok || !strings.EqualFold(scope, "rc") || name == "" || strings.ContainsAny(name, ".[") || !pr.URI.IsFile() {
		return ""
	}

	path := pr.URI.Path()

	section, item, app, ok := fw1ViewName(path)
	if !ok || r.ImplicitExtends == nil || r.ImplicitExtends(path) == "" {
		return ""
	}

	controllers := filepath.Join(app, "controllers")

	actions := r.fw1Actions(controllers, section, item)
	if len(actions) == 0 {
		return ""
	}

	answer := ""

	for _, a := range actions {
		comp := r.fw1ActionRc(a.file, a.action, name, funcName)
		if comp == "" {
			return ""
		}

		if answer != "" && !cfpath.SamePath(answer, comp) {
			return ""
		}

		answer = comp
	}

	tr.addf("%q is what the FW/1 controller action(s) rendering this view assign it: %q", variable, answer)

	return answer
}

type fw1Action struct{ file, action string }

// fw1ViewName splits a view's path into its section and item and the
// application directory holding views/.
func fw1ViewName(path string) (section, item, app string, ok bool) {
	slash := filepath.ToSlash(path)

	before, after, found := strings.CutLast(slash, "/views/")
	if !found || !strings.EqualFold(filepath.Ext(path), ".cfm") {
		return "", "", "", false
	}

	section, file, found := strings.Cut(after, "/")
	if !found || strings.Contains(file, "/") || section == "" {
		return "", "", "", false
	}

	return section, strings.TrimSuffix(file, filepath.Ext(file)), filepath.FromSlash(before), true
}

// fw1Actions are the controller actions that render section.item: the
// section controller's item action, and every action whose body calls
// setView("section.item").
func (r *Resolver) fw1Actions(controllers, section, item string) []fw1Action {
	var out []fw1Action

	own := filepath.Join(controllers, section+".cfc")
	if hpr := r.handlerParse(own); hpr != nil && hasFunc(hpr, item) {
		out = append(out, fw1Action{own, item})
	}

	entries, err := r.fs().ReadDir(controllers)
	if err != nil {
		return out
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".cfc") {
			continue
		}

		file := filepath.Join(controllers, entry.Name())

		hpr := r.handlerParse(file)
		if hpr == nil {
			continue
		}

		for _, m := range fw1SetViewRe.FindAllStringSubmatchIndex(hpr.Content, -1) {
			if !strings.EqualFold(hpr.Content[m[2]:m[3]], section+"."+item) {
				continue
			}

			line := strings.Count(hpr.Content[:m[0]], "\n")

			scope := parser.FindFuncScopeAt(line, hpr.Scopes)
			if scope.Start == -1 {
				return nil
			}

			out = append(out, fw1Action{file, scope.Name})
		}
	}

	return out
}

// fw1ActionRc is what action in the controller file leaves in rc.name: its
// own last assignment, else before()'s.
func (r *Resolver) fw1ActionRc(file, action, name, funcName string) string {
	hpr := r.handlerParse(file)
	if hpr == nil {
		return ""
	}

	for _, fn := range []string{action, "before"} {
		scope, ok := funcScope(hpr, fn)
		if !ok {
			continue
		}

		if !fw1Assigns(hpr.Content, scope, name) {
			continue
		}

		for _, receiver := range []string{"arguments.rc." + name, "rc." + name} {
			comp, _ := r.receiverComponentD(receiver, conv.Uint32(scope.End), scope.Name, funcName, hpr, filepath.Dir(file), nil, lookupCtx{})
			if comp != "" && !strings.HasPrefix(comp, "$") {
				return r.pathsOf(comp, filepath.Dir(file))
			}
		}

		return ""
	}

	return ""
}

func hasFunc(pr *parser.ParseResult, name string) bool {
	_, ok := funcScope(pr, name)

	return ok
}

func funcScope(pr *parser.ParseResult, name string) (parser.FuncScope, bool) {
	for _, s := range pr.Scopes {
		if strings.EqualFold(s.Name, name) {
			return s, true
		}
	}

	return parser.FuncScope{}, false
}

// fw1Assigns reports whether the function at scope assigns rc.name.
func fw1Assigns(content string, scope parser.FuncScope, name string) bool {
	re := regexp.MustCompile(`(?i)\b(?:arguments\.)?rc\.` + regexp.QuoteMeta(name) + `\s*=[^=]`)
	lines := strings.Split(content, "\n")

	for i := scope.Start; i <= scope.End && i < len(lines); i++ {
		if i >= 0 && re.MatchString(lines[i]) {
			return true
		}
	}

	return false
}
