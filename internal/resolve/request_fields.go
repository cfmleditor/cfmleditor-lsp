package resolve

import (
	"regexp"
	"strings"

	cfpath "github.com/cfmleditor/clif/internal/path"
)

// onlyFileWritesRequest reports whether, among every file a batch scan
// reads, file is the only one that writes request.<name>: by assignment,
// through a bracketed key, structInsert or cfparam. Outside a batch scan the
// workspace's files are not all known, and nothing is claimed.
func (r *Resolver) onlyFileWritesRequest(name, file string) bool {
	o := r.owner()
	if len(o.InferArgsFiles) == 0 {
		return false
	}

	key := strings.ToLower(name) + "\x00" + pathKey(file)

	o.mu.RLock()
	cached, hit := o.requestWriteCache[key]
	o.mu.RUnlock()

	if hit {
		return cached
	}

	re := requestWriteRe(name)

	only := true

	for _, path := range o.InferArgsFiles {
		if cfpath.SamePath(path, file) {
			continue
		}

		data, err := r.fs().ReadFile(path)
		if err != nil || !strings.Contains(strings.ToLower(string(data)), strings.ToLower(name)) {
			continue
		}

		if re.Match(data) {
			only = false

			break
		}
	}

	o.mu.Lock()
	if o.requestWriteCache == nil {
		o.requestWriteCache = map[string]bool{}
	}

	o.requestWriteCache[key] = only
	o.mu.Unlock()

	return only
}

// requestWriteRe matches a statement that writes request.<name>: an
// assignment, a write through a bracketed key, structInsert or cfparam.
func requestWriteRe(name string) *regexp.Regexp {
	q := regexp.QuoteMeta(name)

	return regexp.MustCompile(`(?i)\brequest\s*\.\s*` + q + `\s*=[^=]` +
		`|\brequest\s*\[\s*["']` + q + `["']\s*\]\s*=[^=]` +
		`|structInsert\s*\(\s*request\s*,\s*["']` + q + `["']` +
		`|name\s*=\s*["']request\.` + q + `["']`)
}
