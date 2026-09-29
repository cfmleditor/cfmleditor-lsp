package resolve

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
)

// ColdBox mixes helper templates into every handler, view, layout and
// interceptor, so a function a helper declares is callable bare from any of
// them without an include in sight:
//
//   - each module's ModuleConfig.cfc lists its own, relative to the module:
//     this.applicationHelper = [ "helpers/Mixins.cfm" ]
//   - the application's config/ColdBox.cfc names more, relative to the app:
//     applicationHelper : "includes/helpers/ApplicationHelper.cfm"
//   - a module's onLoad may add one to the renderer itself, which is how
//     ContentBox's admin adds cbAdminComponent(): includeUDF(
//     "#moduleMapping#/helpers/Mixins.cfm" ), #moduleMapping# being the module
//   - a view also gets <view>Helper.cfm and <folder>Helper.cfm beside it
//
// cbmessagebox's cbMessageBox(), cbi18n's $r() and ContentBox's
// cbAdminComponent() all reach views this way. Which files receive helpers
// is the preset's (Resolver.HelperScope); finding them is here, since it
// needs the index and the file system.

var (
	moduleHelperRe = regexp.MustCompile(`(?is)this\.applicationHelper\s*=\s*\[([^\]]*)\]`)
	appHelperRe    = regexp.MustCompile(`(?is)\bapplicationHelper\s*[:=]\s*(\[[^\]]*\]|"[^"]*"|'[^']*')`)
	quotedRe       = regexp.MustCompile(`"([^"]*)"|'([^']*)'`)
	includeUDFRe   = regexp.MustCompile(`(?i)includeUDF\(\s*(?:udflibrary\s*=\s*)?["']#moduleMapping#/([^"'#]+)["']`)
)

// helperSet caches the application helpers for one set of config files.
type helperSet struct {
	key       string
	templates []string
}

// findThroughHelpers looks name up in the helper templates mixed into the
// parsed file, when a framework preset says it receives them.
func (r *Resolver) findThroughHelpers(file, name string) (*parser.FunctionDef, string) {
	if r.HelperScope == nil || file == "" || !r.HelperScope(file) {
		return nil, ""
	}

	for _, p := range r.helperTemplates(file) {
		if def := r.LookupFuncWithExtends(p, name); def != nil {
			return def, p
		}
	}

	return nil, ""
}

// helperTemplates are the templates mixed into file: its own view helpers,
// then every application helper in the workspace. ColdBox adds a module's
// helpers to the whole application, and which application a file belongs to
// in a workspace of several is not worth the guess: a helper found in a
// neighbouring app is a call accepted, never one invented.
func (r *Resolver) helperTemplates(file string) []string {
	var out []string

	if strings.EqualFold(filepath.Ext(file), ".cfm") {
		dir := filepath.Dir(file)
		name := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))

		for _, h := range []string{name + "Helper.cfm", filepath.Base(dir) + "Helper.cfm"} {
			if p := r.existing(filepath.Join(dir, h)); p != "" && p != file {
				out = append(out, p)
			}
		}
	}

	return append(out, r.applicationHelpers()...)
}

// applicationHelpers reads every ModuleConfig.cfc and config/ColdBox.cfc the
// index holds for the helpers they name. Remembered until that set of files
// changes; an edit that adds a helper to one is seen when the resolver is
// next rebuilt.
func (r *Resolver) applicationHelpers() []string {
	if r.Index == nil || r.FS == nil {
		return nil
	}

	modules := r.Index.FindFilesByBasename("ModuleConfig")

	var apps []string

	for _, p := range r.Index.FindFilesByBasename("ColdBox") {
		if strings.EqualFold(filepath.Base(filepath.Dir(p)), "config") {
			apps = append(apps, p)
		}
	}

	key := strings.Join(modules, "\x00") + "\x01" + strings.Join(apps, "\x00")

	r.mu.RLock()
	cached := r.helpers
	r.mu.RUnlock()

	if cached != nil && cached.key == key {
		return cached.templates
	}

	var templates []string

	for _, m := range modules {
		templates = append(templates, r.helpersIn(m, moduleHelperRe, filepath.Dir(m))...)
		templates = append(templates, r.includedUDFs(m)...)
	}

	for _, a := range apps {
		templates = append(templates, r.helpersIn(a, appHelperRe, filepath.Dir(filepath.Dir(a)))...)
	}

	r.mu.Lock()
	r.helpers = &helperSet{key: key, templates: templates}
	r.mu.Unlock()

	return templates
}

// helpersIn reads the templates the setting re matches in configFile names,
// each relative to base.
func (r *Resolver) helpersIn(configFile string, re *regexp.Regexp, base string) []string {
	data, err := r.FS.ReadFile(configFile)
	if err != nil {
		return nil
	}

	m := re.FindSubmatch(data)
	if m == nil {
		return nil
	}

	var out []string

	for _, q := range quotedRe.FindAllSubmatch(m[1], -1) {
		rel := strings.TrimSpace(string(q[1]) + string(q[2]))
		if rel == "" || strings.Contains(rel, "#") {
			continue
		}

		if filepath.Ext(rel) == "" {
			rel += ".cfm"
		}

		if p := r.existing(filepath.Join(base, filepath.FromSlash(strings.TrimPrefix(rel, "/")))); p != "" {
			out = append(out, p)
		}
	}

	return out
}

// includedUDFs are the templates a module's own code hands includeUDF under
// its #moduleMapping#. A renderer's helper reaches views and layouts, and it is
// offered to handlers as well: over-accepting a call there costs a finding
// rather than inventing one.
func (r *Resolver) includedUDFs(moduleConfig string) []string {
	data, err := r.FS.ReadFile(moduleConfig)
	if err != nil {
		return nil
	}

	var out []string

	for _, m := range includeUDFRe.FindAllSubmatch(data, -1) {
		rel := string(m[1])
		if filepath.Ext(rel) == "" {
			rel += ".cfm"
		}

		if p := r.existing(filepath.Join(filepath.Dir(moduleConfig), filepath.FromSlash(rel))); p != "" {
			out = append(out, p)
		}
	}

	return out
}

// existing is p when it names a file.
func (r *Resolver) existing(p string) string {
	if info, err := r.FS.Stat(p); err == nil && !info.IsDir() {
		return p
	}

	return ""
}
