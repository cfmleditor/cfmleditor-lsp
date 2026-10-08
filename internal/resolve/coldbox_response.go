package resolve

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cfmleditor/clif/internal/parser"
	cfpath "github.com/cfmleditor/clif/internal/path"
)

// ColdBox keeps a request's response object in prc.response:
// RequestContext.getResponse() — "The response object lives in
// `prc.response`" — stores a coldbox.system.web.context.Response there, and
// RestHandler calls it before reading `arguments.prc.response`. In a handler
// whose chain reaches ColdBox's EventHandler, prc.response is that Response,
// unless the file assigns it itself. No application in the corpus does.

const coldboxResponse = "coldbox.system.web.context.Response"

var prcResponseWriteRe = regexp.MustCompile(`(?i)\bprc\s*(?:\.\s*response|\[\s*["']response["']\s*\])\s*=[^=]`)

// coldboxPrcResponse is ColdBox's Response for prc.response in a handler, or "".
func (r *Resolver) coldboxPrcResponse(variable string, pr *parser.ParseResult) string {
	v := strings.ToLower(strings.TrimPrefix(strings.ToLower(variable), "arguments."))
	if v != "prc.response" || !pr.URI.IsFile() || prcResponseWriteRe.MatchString(pr.Content) {
		return ""
	}

	path := pr.URI.Path()
	if !r.reachesEventHandler(path) {
		return ""
	}

	return r.ComponentPath(coldboxResponse, filepath.Dir(path))
}

// reachesEventHandler reports whether path's extends chain reaches ColdBox's
// EventHandler: by its dotted name, or by resolving to its file, as ColdBox's
// own RestHandler does with a bare extends="EventHandler".
func (r *Resolver) reachesEventHandler(path string) bool {
	target := r.ComponentPath("coldbox.system.EventHandler", filepath.Dir(path))
	seen := map[string]bool{}

	for depth := 0; path != "" && !seen[path] && depth < 12; depth++ {
		seen[path] = true

		ext, ok := r.extendsOf(path, cfpath.ToURI(path))
		if !ok || ext == "" {
			return false
		}

		if strings.EqualFold(ext, "coldbox.system.EventHandler") {
			return true
		}

		next := r.ComponentPath(ext, filepath.Dir(path))
		if next != "" && target != "" && cfpath.SamePath(next, target) {
			return true
		}

		path = next
	}

	return false
}
