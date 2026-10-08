package resolve

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cfmleditor/clif/internal/parser"
	cfpath "github.com/cfmleditor/clif/internal/path"
)

// ColdBox renders an application's error page by including the template its
// config names as customErrorTemplate, from inside Bootstrap's
// processException(), where `var oException = new ExceptionBean( … )` is the
// exception the page reads. The include is computed (`include
// "#bugReportRelativePath#"`), so the include graph never sees it, and every
// oException call in ColdBox's own Whoops.cfm and BugReport.cfm was "has no
// component ref".
//
// coldboxErrorHosts is that include, for a template some config/ColdBox.cfc
// names: Bootstrap's own site, checked by its text and by the function it sits
// in. It needs ColdBox's source; a stub has no body to read.

var customErrorTemplateRe = regexp.MustCompile(`(?i)customErrorTemplate\s*[:=]\s*["']([^"'#]+)["']`)

const coldboxErrorInclude = `include "#bugReportRelativePath#"`

type includeHost struct {
	path string
	line int
}

func (r *Resolver) coldboxErrorHosts(file string) []includeHost {
	if r.Index == nil {
		return nil
	}

	var hosts []includeHost

	for _, config := range r.Index.FindFilesByBasename("ColdBox") {
		if !strings.EqualFold(filepath.Base(filepath.Dir(config)), "config") {
			continue
		}

		data, err := r.fs().ReadFile(config)
		if err != nil {
			continue
		}

		named := false

		for _, m := range customErrorTemplateRe.FindAllStringSubmatch(string(data), -1) {
			if cfpath.SamePath(r.IncludePath(m[1], config), file) {
				named = true
			}
		}

		if !named {
			continue
		}

		if host, ok := r.bootstrapErrorSite(filepath.Dir(config)); ok && !containsHost(hosts, host) {
			hosts = append(hosts, host)
		}
	}

	return hosts
}

// bootstrapErrorSite is the line of Bootstrap's processException() that
// includes the custom error template, read from dir.
func (r *Resolver) bootstrapErrorSite(dir string) (includeHost, bool) {
	path := r.ComponentPath("coldbox.system.Bootstrap", dir)
	if path == "" {
		return includeHost{}, false
	}

	hpr := r.handlerParse(path)
	if hpr == nil {
		return includeHost{}, false
	}

	at := strings.Index(hpr.Content, coldboxErrorInclude)
	if at < 0 {
		return includeHost{}, false
	}

	line := strings.Count(hpr.Content[:at], "\n")
	if !strings.EqualFold(parser.FindFuncScopeAt(line, hpr.Scopes).Name, "processException") {
		return includeHost{}, false
	}

	return includeHost{path: path, line: line}, true
}

func containsHost(hosts []includeHost, h includeHost) bool {
	for _, x := range hosts {
		if cfpath.SamePath(x.path, h.path) && x.line == h.line {
			return true
		}
	}

	return false
}
