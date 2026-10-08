package resolve

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cfmleditor/clif/internal/parser"
	cfpath "github.com/cfmleditor/clif/internal/path"
)

// ColdBox's unit-test bases build the component under test from the test's
// own attribute: a component that extends coldbox.system.testing.BaseModelTest
// and says model="coldbox.system.core.events.EventPool" runs with
// `variables.model = mockBox.createMock( annotations.model )`, and
// BaseInterceptorTest does the same for interceptor="…". Every call on model
// in such a test was "has no component ref". It is a mock of that class, so
// MockBox's own methods are accepted on it as on any component.

var coldboxTestBases = map[string]string{
	"coldbox.system.testing.basemodeltest":       "model",
	"coldbox.system.testing.baseinterceptortest": "interceptor",
}

// coldboxTestSubject is the class a ColdBox test base mocks into variable, or "".
func (r *Resolver) coldboxTestSubject(variable string, pr *parser.ParseResult) string {
	name := strings.ToLower(strings.TrimPrefix(strings.ToLower(variable), "variables."))
	if name != "model" && name != "interceptor" || !pr.URI.IsFile() {
		return ""
	}

	path := pr.URI.Path()
	if !strings.EqualFold(filepath.Ext(path), ".cfc") {
		return ""
	}

	m := regexp.MustCompile(`(?is)^[^{]*?\b(?:component|cfcomponent)\b[^{>]*?\b` + name + `\s*=\s*["']([\w.]+)["']`).FindStringSubmatchIndex(pr.Content)
	if m == nil {
		return ""
	}

	// The attribute itself reads as an assignment to setsName; the file
	// assigning the name anywhere else means it is not the base's mock.
	if setsName(pr.Content[m[1]:], name) {
		return ""
	}

	if base := r.coldboxTestBase(path); coldboxTestBases[base] != name {
		return ""
	}

	return r.ComponentPath(pr.Content[m[2]:m[3]], filepath.Dir(path))
}

// coldboxTestBase is the ColdBox test base path's extends chain reaches,
// lowercased, or "".
func (r *Resolver) coldboxTestBase(path string) string {
	seen := map[string]bool{}

	for depth := 0; path != "" && !seen[path] && depth < 12; depth++ {
		seen[path] = true

		ext, ok := r.extendsOf(path, cfpath.ToURI(path))
		if !ok || ext == "" {
			return ""
		}

		if _, known := coldboxTestBases[strings.ToLower(ext)]; known {
			return strings.ToLower(ext)
		}

		path = r.ComponentPath(ext, filepath.Dir(path))
	}

	return ""
}
