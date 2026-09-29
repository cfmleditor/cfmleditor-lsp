package resolve

import (
	"path/filepath"
	"regexp"
	"strings"

	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
)

// cborm's VirtualEntityService is a service bound to one entity: ContentBox's
// AuthorService calls `super.init( entityName = "cbAuthor" )`, and from then
// its new(), get(), getOrFail() and findWhere() hand back cbAuthor entities.
// The methods are the base's, declared `any`, so the answer is the entity
// the service was bound to — which the service's own source states.

// entityMethods are the VirtualEntityService methods that return one entity
// of the service's own.
var entityMethods = map[string]bool{
	"new": true, "get": true, "getorfail": true, "findwhere": true,
}

var superInitEntityRe = regexp.MustCompile(`(?is)\bsuper\.init\(\s*(?:entityName\s*=\s*)?["']([^"'#]+)["']`)

// entityReturn is the entity a call to method on the service at path
// returns, or "".
func (r *Resolver) entityReturn(path, method string) string {
	if path == "" || !entityMethods[strings.ToLower(method)] {
		return ""
	}

	return r.boundEntity(path)
}

// boundEntity is the entity the service at path, or a service it extends,
// binds itself to with super.init( entityName = … ), or "".
func (r *Resolver) boundEntity(path string) string {
	seen := map[string]bool{}

	for depth := 0; path != "" && !seen[path] && depth < 8; depth++ {
		seen[path] = true

		data, err := r.fs().ReadFile(path)
		if err != nil {
			return ""
		}

		if m := superInitEntityRe.FindSubmatch(data); m != nil {
			return strings.TrimSpace(string(m[1]))
		}

		ext, ok := r.extendsOf(path, cfpath.ToURI(path))
		if !ok || ext == "" {
			return ""
		}

		path = r.ComponentPath(ext, filepath.Dir(path))
	}

	return ""
}
