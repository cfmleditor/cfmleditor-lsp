package resolve

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cfmleditor/clif/internal/parser"
	cfpath "github.com/cfmleditor/clif/internal/path"
)

// A Wheels model's methods are mostly not written anywhere: the finders are
// wheels.Model's, declared `any` because they return whichever model they
// are called on, and an association declared in config() —
// `hasMany( "comments" )`, `belongsTo( "author" )` — adds a family of
// methods named after it. Both are rules the Wheels docs state, and this
// file applies them.

// wheelsFinders are the Model methods that return one instance of the model
// they are called on. The findAll family returns a query, and the dynamic
// finders go through onMissingMethod, which makes the rest of a chain
// dynamic already.
var wheelsFinders = map[string]bool{
	"findbykey": true, "findone": true, "findfirst": true, "findlast": true,
	"new": true, "create": true, "reload": true,
}

// isWheelsModel reports whether the component at path extends wheels.Model,
// its own copy or the bundled one.
func (r *Resolver) isWheelsModel(path string) bool {
	seen := map[string]bool{}

	for depth := 0; path != "" && !seen[path] && depth < 8; depth++ {
		seen[path] = true

		if strings.HasSuffix(strings.ToLower(filepath.ToSlash(path)), "/wheels/model.cfc") {
			return true
		}

		ext, ok := r.extendsOf(path, cfpath.ToURI(path))
		if !ok || ext == "" {
			return false
		}

		if strings.EqualFold(ext, "wheels.Model") {
			return true
		}

		path = r.ComponentPath(ext, filepath.Dir(path))
	}

	return false
}

// receiverReturn is what a call to method on the component at path returns
// when the method's declaration does not say: the entity a cborm service is
// bound to, or the Wheels model a finder is called on.
func (r *Resolver) receiverReturn(path, method string) string {
	if path == "" {
		return ""
	}

	if e := r.entityReturn(path, method); e != "" {
		return e
	}

	if wheelsFinders[strings.ToLower(method)] && r.isWheelsModel(path) {
		return path
	}

	return ""
}

// associationRe is a Wheels association in a model's config():
// `hasMany( "comments" )`, `belongsTo( name = "author", modelName = "User" )`.
var associationRe = regexp.MustCompile(`(?is)\b(hasMany|belongsTo|hasOne)\(\s*(?:name\s*=\s*)?["'](\w+)["']([^)]*)\)`)

var modelNameArgRe = regexp.MustCompile(`(?i)\bmodelName\s*=\s*["'](\w+)["']`)

// associationFunc is the method name an association declared in the model
// at path gives it, as a definition returning the associated model, or nil.
func (r *Resolver) associationFunc(path, name string) *parser.FunctionDef {
	data, err := r.fs().ReadFile(path)
	if err != nil {
		return nil
	}

	lower := strings.ToLower(name)
	src := string(data)

	for _, m := range associationRe.FindAllStringSubmatchIndex(src, -1) {
		kind, assoc, args := src[m[2]:m[3]], src[m[4]:m[5]], src[m[6]:m[7]]
		singular := singularize(assoc)

		model := ucFirstWord(singular)
		if mm := modelNameArgRe.FindStringSubmatch(args); mm != nil {
			model = mm[1]
		}

		for _, method := range associationMethods(strings.ToLower(kind), assoc, singular) {
			if method.name != lower {
				continue
			}

			def := &parser.FunctionDef{
				Name: name, URI: cfpath.ToURI(path),
				Line: uint32(strings.Count(src[:m[0]], "\n")), //nolint:gosec // a line count of a file read here
			}

			if method.returnsModel {
				def.ReturnComponent = model
			}

			return def
		}
	}

	return nil
}

type associationMethod struct {
	name         string
	returnsModel bool
}

// associationMethods are the methods Wheels adds for an association, from its
// documentation of hasMany, belongsTo and hasOne, lowercased.
func associationMethods(kind, assoc, singular string) []associationMethod {
	a, s := strings.ToLower(assoc), strings.ToLower(singular)

	switch kind {
	case "hasmany":
		return []associationMethod{
			{a, false},
			{"add" + s, false},
			{"remove" + s, false},
			{"delete" + s, false},
			{"removeall" + a, false},
			{"deleteall" + a, false},
			{s + "count", false},
			{"has" + a, false},
			{"findone" + s, true},
			{"new" + s, true},
			{"create" + s, true},
		}
	case "belongsto":
		return []associationMethod{{a, true}, {"has" + a, false}}
	case "hasone":
		return []associationMethod{
			{a, true},
			{"create" + a, true},
			{"new" + a, true},
			{"delete" + a, false},
			{"remove" + a, false},
			{"set" + a, false},
			{"has" + a, false},
		}
	}

	return nil
}

// singularize is the English singular Wheels derives an association's
// object name from, for the regular plurals a model name has.
func singularize(word string) string {
	lower := strings.ToLower(word)

	switch {
	case strings.HasSuffix(lower, "ies") && len(word) > 3:
		return word[:len(word)-3] + "y"
	case strings.HasSuffix(lower, "sses"), strings.HasSuffix(lower, "xes"), strings.HasSuffix(lower, "ches"), strings.HasSuffix(lower, "shes"):
		return word[:len(word)-2]
	case strings.HasSuffix(lower, "s") && !strings.HasSuffix(lower, "ss") && len(word) > 1:
		return word[:len(word)-1]
	}

	return word
}

func ucFirstWord(s string) string {
	if s == "" {
		return s
	}

	return strings.ToUpper(s[:1]) + s[1:]
}
