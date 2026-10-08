package resolve

import (
	"path/filepath"
	"strings"

	"github.com/cfmleditor/clif/internal/parser"
)

// thisCallAnswered reports whether call is `this.name()` on a component that
// defines onMissingMethod, itself or through its extends chain. CFML hands
// a missing method called on the object to onMissingMethod, and a method
// called through `this.` is called on the object: cborm's services call
// their dynamic finders so, `this.findBySlug( slug )`. An unscoped call is a
// function lookup, which onMissingMethod never answers, so it is not
// accepted. The parser records both as the same bare call and says which
// was written in CallSite.This.
func (r *Resolver) thisCallAnswered(call *parser.CallSite, pr *parser.ParseResult, baseDir string) bool {
	return call.This && r.definesMissingMethod(pr, baseDir)
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
