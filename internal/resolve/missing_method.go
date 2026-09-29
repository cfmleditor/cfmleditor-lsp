package resolve

import (
	"path/filepath"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
)

// thisCallAnswered reports whether call is `this.name()` on a component that
// defines onMissingMethod, itself or through its extends chain. CFML hands
// a missing method called on the object to onMissingMethod, and a method
// called through `this.` is called on the object: cborm's services call
// their dynamic finders so, `this.findBySlug( slug )`. An unscoped call is a
// function lookup, which onMissingMethod never answers, so it is not
// accepted. The parser records both as the same bare call, so the line says
// which was written; it is read only here, on the way to reporting a call.
func (r *Resolver) thisCallAnswered(call *parser.CallSite, pr *parser.ParseResult, baseDir string) bool {
	if !r.definesMissingMethod(pr, baseDir) {
		return false
	}

	line := strings.ToLower(lineAt(pr.Content, int(call.Line)))
	needle := "this." + strings.ToLower(call.FuncName)

	for off := 0; ; {
		i := strings.Index(line[off:], needle)
		if i < 0 {
			return false
		}

		at := off + i
		rest := strings.TrimLeft(line[at+len(needle):], " \t")

		if strings.HasPrefix(rest, "(") && (at == 0 || !isIdentByte(line[at-1])) {
			return true
		}

		off = at + 1
	}
}

func (r *Resolver) definesMissingMethod(pr *parser.ParseResult, baseDir string) bool {
	for i := range pr.Funcs {
		if strings.EqualFold(pr.Funcs[i].Name, "onMissingMethod") {
			return true
		}
	}

	ext := r.fileExtends(pr)

	return ext != "" && r.ResolveFunc(ext, "onMissingMethod", baseDir) != nil
}

// lineAt is line n, 0-based, of content, or "".
func lineAt(content string, n int) string {
	for ; n > 0; n-- {
		i := strings.IndexByte(content, '\n')
		if i < 0 {
			return ""
		}

		content = content[i+1:]
	}

	if i := strings.IndexByte(content, '\n'); i >= 0 {
		content = content[:i]
	}

	return content
}

func isIdentByte(b byte) bool {
	return b == '_' || b == '$' || b == '.' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

// selfTyped is the receiver's own path when fd returns the class that
// declares it and is called on a subclass of that class. A fluent method
// returns `this`, which is the object it was called on: cborm's BaseBuilder
// declares `BaseBuilder function add()`, and on a CriteriaBuilder add()
// hands back the CriteriaBuilder, whose onMissingMethod answers isEq(). A
// subclass has every method its base has, so the narrower type accepts
// everything the declared one does. It is "" when the question does not
// arise.
func (r *Resolver) selfTyped(receiver, baseDir string, fd *parser.FunctionDef, ret string) string {
	if fd == nil || ret == "" || receiver == "" || !fd.URI.IsFile() {
		return ""
	}

	declaring := fd.URI.Path()

	if !samePath(r.ComponentPath(ret, filepath.Dir(declaring)), declaring) {
		return ""
	}

	recv := r.ComponentPath(receiver, baseDir)
	if recv == "" || samePath(recv, declaring) {
		return ""
	}

	return recv
}

func samePath(a, b string) bool {
	return a != "" && strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}
