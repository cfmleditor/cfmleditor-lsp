package resolve

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
)

// Methods that ColdBox modules add to objects at run time, where the
// workspace may not hold the module: a project installs its modules with
// CommandBox and rarely commits them. Each is a rule of the module's own,
// read from its source at the commit named, and each list is only what that
// source declares.

// moduleHelpers are the functions a module's `this.applicationHelper`
// template declares, which ColdBox mixes into every handler, interceptor,
// view and layout. helpers.go finds them when the module is on disk and a
// preset says where they go; these are for when neither holds.
var moduleHelpers = map[string]string{
	// coldbox-modules/cbi18n@79418694 helpers/Mixins.cfm; $r is
	// `variables.$r = variables.getResource`.
	"getfwlocale": "cbi18n", "setfwlocale": "cbi18n", "getresource": "cbi18n",
	"$r": "cbi18n", "i18n": "cbi18n", "resourceservice": "cbi18n",
	// coldbox-modules/cbfs@def71cdf helpers/Mixins.cfm.
	"cbfs": "cbfs",
	// ColdBox's own HTMLHelper module, system/modules/HTMLHelper.
	"addasset": "HTMLHelper",
}

// moduleHelper reports whether a bare call to funcName in pr is a module
// helper ColdBox mixes into it: a helper by name, in a component whose
// extends chain reaches ColdBox.
func (r *Resolver) moduleHelper(pr *parser.ParseResult, funcName, baseDir string) (module string, ok bool) {
	module, ok = moduleHelpers[strings.ToLower(funcName)]
	if !ok || !pr.URI.IsFile() {
		return "", false
	}

	return module, r.reachesColdBox(pr.URI.Path(), baseDir)
}

// reachesColdBox reports whether the component at path extends one of
// ColdBox's own, directly or through its bases.
func (r *Resolver) reachesColdBox(path, baseDir string) bool {
	seen := map[string]bool{}

	for depth := 0; path != "" && !seen[path] && depth < 12; depth++ {
		seen[path] = true

		ext, ok := r.extendsOf(path, cfpath.ToURI(path))
		if !ok || ext == "" {
			return false
		}

		if len(ext) > len("coldbox.system.") && strings.EqualFold(ext[:len("coldbox.system.")], "coldbox.system.") {
			return true
		}

		next := r.ComponentPath(ext, filepath.Dir(path))
		if next == "" {
			next = r.ComponentPath(ext, baseDir)
		}

		path = next
	}

	return false
}

var mementoRe = regexp.MustCompile(`(?i)\bthis\.memento\s*=`)

// mementoFunc is getMemento() on a component that declares `this.memento`,
// itself or through its extends chain: mementifier
// (coldbox-modules/mementifier@d7113dbb, interceptors/Mementifier.cfc)
// injects it into every ORM entity and WireBox object carrying one, unless
// the object has its own.
func (r *Resolver) mementoFunc(chain []string, funcName string) *parser.FunctionDef {
	if !strings.EqualFold(funcName, "getMemento") {
		return nil
	}

	for _, p := range chain {
		data, err := r.fs().ReadFile(p)
		if err != nil || !mementoRe.Match(data) {
			continue
		}

		return &parser.FunctionDef{Name: funcName, URI: cfpath.ToURI(p), ReturnType: "struct"}
	}

	return nil
}
