package resolve

import (
	"path/filepath"
	"regexp"
	"strings"
)

// A template may be included by a computed path: Masa's form builder keeps
// `variables.templatePath = "/muraWRM#…#/core/utilities/formbuilder/templates"`
// and includes `<cfinclude template="#templatePath#">` from functions that
// set `var mmRBF = application.rbFactory` first. The include graph cannot see
// that edge, and every mmRBF call in the 30-odd field templates was "has no
// component ref".
//
// computedIncludeHosts treats a file as an includer of a template when it
// holds both a computed include and a string literal naming the template's
// directory by at least its last minDirSegments segments; each of its computed
// includes is a site, and includerHeld requires every site to agree, as it does
// for a literal include. Candidate files come from the batch caller index (a
// name ending a quoted string), so this runs in a batch scan only.

const minDirSegments = 3

var computedIncludeRe = regexp.MustCompile(`(?i)<cfinclude\s+template\s*=\s*["']#[^"']*#["']|\binclude\s+["']#[^"']*#["']`)

func (r *Resolver) computedIncludeHosts(file string) []includeHost {
	if len(r.owner().InferArgsFiles) == 0 {
		return nil
	}

	segs := strings.Split(filepath.ToSlash(filepath.Dir(file)), "/")
	if len(segs) < minDirSegments {
		return nil
	}

	suffix := strings.ToLower(strings.Join(segs[len(segs)-minDirSegments:], "/"))
	last := segs[len(segs)-1]

	var hosts []includeHost

	for _, path := range r.callerFiles(quotedCallerKey(last)) {
		if strings.EqualFold(filepath.Clean(path), filepath.Clean(file)) {
			continue
		}

		data, err := r.fs().ReadFile(path)
		if err != nil {
			continue
		}

		content := string(data)
		if lower := strings.ToLower(content); !strings.Contains(lower, suffix+`"`) && !strings.Contains(lower, suffix+`'`) && !strings.Contains(lower, suffix+`/"`) {
			continue
		}

		for _, m := range computedIncludeRe.FindAllStringIndex(content, -1) {
			hosts = append(hosts, includeHost{path: path, line: strings.Count(content[:m[0]], "\n")})
		}
	}

	return hosts
}
