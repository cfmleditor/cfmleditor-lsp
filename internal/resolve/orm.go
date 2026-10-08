package resolve

import (
	"path/filepath"
	"regexp"
	"strings"

	cfpath "github.com/cfmleditor/clif/internal/path"
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

// superInitArgRe is `super.init( entityName = arguments.x )`: the service
// passes on an argument of its own init, bound to that argument's default.
var superInitArgRe = regexp.MustCompile(`(?is)\bsuper\.init\(\s*(?:entityName\s*=\s*)?(?:arguments\.)?([a-z_][\w]*)\s*[,)]`)

// scriptInitRe and tagInitRe find the service's own init(): its parameter
// list in script syntax, its body in tag syntax, where the <cfargument>s are.
var (
	scriptInitRe = regexp.MustCompile(`(?is)\bfunction\s+init\s*\(([^)]*)\)`)
	tagInitRe    = regexp.MustCompile(`(?is)<cffunction\s[^>]*\bname\s*=\s*["']init["'][^>]*>(.*?)</cffunction>`)
	cfargumentRe = regexp.MustCompile(`(?is)<cfargument\s[^>]*>`)

	// paramDefaultRe is a script parameter with a literal default.
	paramDefaultRe = regexp.MustCompile(`(?is)(?:^|[,\s])([a-z_]\w*)\s*=\s*["']([^"'#]+)["']`)
)

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

		if m := superInitArgRe.FindSubmatch(data); m != nil {
			// The service binds whatever its own init() was handed, so the
			// answer is the argument's default, and nothing without one.
			return initArgumentDefault(data, string(m[1]))
		}

		ext, ok := r.extendsOf(path, cfpath.ToURI(path))
		if !ok || ext == "" {
			return ""
		}

		path = r.ComponentPath(ext, filepath.Dir(path))
	}

	return ""
}

// initArgumentDefault is the literal default of the argument name of the
// init() declared in src, or "": `init( entityName = "cbContent" )`, or a
// `<cfargument name="entityName" default="cbContent">` inside
// `<cffunction name="init">`.
func initArgumentDefault(src []byte, name string) string {
	if m := scriptInitRe.FindSubmatch(src); m != nil {
		for _, d := range paramDefaultRe.FindAllSubmatch(m[1], -1) {
			if strings.EqualFold(string(d[1]), name) {
				return strings.TrimSpace(string(d[2]))
			}
		}
	}

	m := tagInitRe.FindSubmatch(src)
	if m == nil {
		return ""
	}

	for _, tag := range cfargumentRe.FindAll(m[1], -1) {
		if !strings.EqualFold(tagAttr(tag, "name"), name) {
			continue
		}

		if d := tagAttr(tag, "default"); d != "" && !strings.Contains(d, "#") {
			return strings.TrimSpace(d)
		}

		return ""
	}

	return ""
}

var tagAttrRe = regexp.MustCompile(`(?is)\b([a-z]+)\s*=\s*(?:"([^"]*)"|'([^']*)')`)

// tagAttr is the value of attribute attr in tag, or "".
func tagAttr(tag []byte, attr string) string {
	for _, m := range tagAttrRe.FindAllSubmatch(tag, -1) {
		if strings.EqualFold(string(m[1]), attr) {
			return string(m[2]) + string(m[3])
		}
	}

	return ""
}
