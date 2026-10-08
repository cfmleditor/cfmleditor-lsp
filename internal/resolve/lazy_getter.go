package resolve

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
)

// A lazy getter loads its field the first time it is asked:
//
//	<cfset variables.instance.extensionManager = ""/>
//	…
//	<cffunction name="getClassExtensionManager">
//		<cfif not isObject(variables.instance.extensionManager)>
//			<cfset loadClassExtensionManager()/>
//		</cfif>
//		<cfreturn variables.instance.extensionManager />
//	</cffunction>
//
// The field is written twice, a "" placeholder and a component, so it has no
// single type, and the getter none either; Mura's configBean hands out its
// class extension manager this way, and 64 admin calls on it were untyped. The
// guard is what settles it: what the getter returns is never the placeholder.
//
// lazyGetterReturn answers for a function whose whole body is that guard and
// that return, in script or in tags: the component every other write of the
// field in the file creates in place. A write of anything else withholds the
// answer.

var (
	lazyTagRe     = regexp.MustCompile(`(?is)^<cfif\s+(?:not\s+|!\s*)isObject\(\s*([\w.]+)\s*\)\s*>(.*)</cfif>\s*<cfreturn\s+([\w.]+)\s*/?>$`)
	lazyScriptRe  = regexp.MustCompile(`(?is)^if\s*\(\s*(?:not\s+|!\s*)isObject\(\s*([\w.]+)\s*\)\s*\)\s*\{([^{}]*)\}\s*return\s+([\w.]+)\s*;$`)
	argumentTagRe = regexp.MustCompile(`(?is)<cfargument\b[^>]*>`)
	placeholderRe = regexp.MustCompile(`^(?:""|'')$`)
)

// lazyGetterReturn is the component fd returns when it is a lazy getter, or "".
// It reads the file's text, not a parse of it: a parse asks for return types,
// which is how this was reached.
func (r *Resolver) lazyGetterReturn(fd *parser.FunctionDef) string {
	path := fd.URI.Path()
	o := r.owner()
	key := pathKey(path) + "\x00" + strings.ToLower(fd.Name)

	o.mu.RLock()
	answer, ok := o.lazyGetters[key]
	guarded, scanned := o.lazyFiles[pathKey(path)]
	o.mu.RUnlock()

	if ok {
		return answer
	}

	// Asked about every untyped function, so a file with no isObject()
	// guard at all is remembered as such and not read again.
	if scanned && !guarded {
		return ""
	}

	data, err := r.fs().ReadFile(path)
	if err != nil {
		return ""
	}

	content := string(data)
	guarded = strings.Contains(strings.ToLower(content), "isobject(")

	if guarded {
		if field := lazyField(functionBodyNamed(content, fd.Name)); field != "" {
			answer = r.fieldWrites(content, field, filepath.Dir(path))
		}
	}

	o.mu.Lock()
	if o.lazyGetters == nil {
		o.lazyGetters, o.lazyFiles = map[string]string{}, map[string]bool{}
	}

	o.lazyFiles[pathKey(path)] = guarded
	o.lazyGetters[key] = answer
	o.mu.Unlock()

	return answer
}

// functionBodyNamed is the text between the braces of the function name in
// content, or between its <cffunction> tag and </cffunction>, with its
// <cfargument> tags removed; "" when there is not exactly one.
func functionBodyNamed(content, name string) string {
	tag := regexp.MustCompile(`(?is)<cffunction\b[^>]*\bname\s*=\s*["']` + regexp.QuoteMeta(name) + `["'][^>]*>(.*?)</cffunction>`)
	if all := tag.FindAllStringSubmatch(content, 2); len(all) == 1 {
		return strings.TrimSpace(argumentTagRe.ReplaceAllString(all[0][1], ""))
	}

	script := regexp.MustCompile(`(?i)\bfunction\s+` + regexp.QuoteMeta(name) + `\s*\(`)

	all := script.FindAllStringIndex(content, 2)
	if len(all) != 1 {
		return ""
	}

	rest := content[all[0][1]:]

	open := strings.IndexByte(rest, '{')
	if open < 0 {
		return ""
	}

	depth := 0

	for i := open; i < len(rest); i++ {
		switch rest[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return strings.TrimSpace(rest[open+1 : i])
			}
		}
	}

	return ""
}

// lazyField is the field body guards and returns, without a variables.
// prefix, when body is only that guard and that return.
func lazyField(body string) string {
	m := lazyTagRe.FindStringSubmatch(body)
	if m == nil {
		m = lazyScriptRe.FindStringSubmatch(body)
	}

	// The guarded block only loads the field: one that returns, or opens
	// another <cfif>, is not this shape.
	if m == nil || strings.Contains(strings.ToLower(m[2]), "return") || strings.Contains(strings.ToLower(m[2]), "<cfif") {
		return ""
	}

	guarded, returned := stripVariables(m[1]), stripVariables(m[3])
	if guarded == "" || !strings.EqualFold(guarded, returned) {
		return ""
	}

	return guarded
}

func stripVariables(name string) string {
	if len(name) > len("variables.") && strings.EqualFold(name[:len("variables.")], "variables.") {
		return name[len("variables."):]
	}

	return name
}

// fieldWrites is the component every write of field in content agrees on, the ""
// placeholder aside, or "".
func (r *Resolver) fieldWrites(content, field, dir string) string {
	// Anywhere, not only at a line's start: a write inside a one-line if is
	// a write all the same, and one missed would be one not checked.
	re := regexp.MustCompile(`(?i)(?:^|[^\w.$])(?:variables\.)?` + regexp.QuoteMeta(field) + `\s*=\s*([^=][^;\n]*?)\s*(?:/?>|;|\n|$)`)
	answer := ""

	for _, m := range re.FindAllStringSubmatchIndex(content, -1) {
		rhs := strings.TrimSpace(content[m[2]:m[3]])
		if placeholderRe.MatchString(rhs) {
			continue
		}

		comp := r.writtenComponent(rhs, dir)
		if comp == "" || strings.HasPrefix(comp, "$") || answer != "" && !strings.EqualFold(answer, comp) {
			return ""
		}

		answer = comp
	}

	return answer
}

// writtenComponent types rhs, written at line of pr: a component created in
// place, with an init() on it.
func (r *Resolver) writtenComponent(rhs string, dir string) string {
	if path := createdOnly(rhs); path != "" {
		return r.ComponentPath(path, dir)
	}

	// Anything else is not read: typing a call re-enters return inference,
	// which is what asked.
	return ""
}

// createdOnly is the component rhs creates, when that is all it does: `new
// a.b.C( … )` or `createObject( "component", "a.b.C" )`, alone or with an
// init() on it, which returns the instance by the convention every CFML
// constructor follows.
func createdOnly(rhs string) string {
	tokens := significantTokens(rhs)

	if n := len(tokens); n > 4 && tokens[n-1].Kind == parser.TokRParen {
		if open := lastCallOpen(tokens); open > 2 && tokens[open-2].Kind == parser.TokDot && strings.EqualFold(tokens[open-1].Value, "init") {
			tokens = tokens[:open-2]
		}
	}

	return instantiated(tokens)
}
