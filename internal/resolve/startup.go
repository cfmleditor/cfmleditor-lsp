package resolve

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
)

// A shared-scope variable — REQUEST.context, SERVER.kernel — is set up once,
// usually by a template the application's Application.cfc includes, and read
// everywhere else. The calling file then has no assignment of its own to type
// it by, and neither has anything it extends. In tassweb, bootstrap.cfm holds
// `REQUEST.context = REQUEST.kernel.getContextObject()`, included by each
// application's Application.cfc; 6,013 calls on REQUEST.context depended on a
// componentResolver matching the name, and 5,321 of them sit under an
// Application.cfc that includes that template.
//
// The lookup here reads those templates' assignments and types the right-hand
// side itself: a created component, or a chain on another shared variable (or
// on something a componentResolver names), walked through each hop's return
// type. It does not go back through canResolveCall, so its recursion is its
// own and bounded by a visited set, and its answer is the same whichever
// goroutine asks first.

// maxStartupTemplates bounds the include walk from one Application.cfc.
const maxStartupTemplates = 64

// sharedScopes are the scopes a startup template's assignments are visible in
// from other files.
var sharedScopes = []string{"request", "session", "application", "server"}

var (
	tagSharedAssign    = regexp.MustCompile(`(?i)<cfset\s+(request|session|application|server)\.([A-Za-z_$][\w$]*)\s*=\s*(.+?)\s*/?>`)
	scriptSharedAssign = regexp.MustCompile(`(?i)^\s*(request|session|application|server)\.([A-Za-z_$][\w$]*)\s*=\s*([^;]+?)\s*;`)
	createdComponent   = regexp.MustCompile(`(?i)^createObject\s*\(\s*["']component["']\s*,\s*["']([^"']+)["']`)
	newComponent       = regexp.MustCompile(`(?i)^new\s+(?:cfml:)?([A-Za-z_$#][\w$.#]*)\s*\(`)
	chainExpr          = regexp.MustCompile(`^[A-Za-z_$][\w$]*(?:\.[A-Za-z_$][\w$]*(?:\([^()]*\))?)+$`)
)

// startupAssign is one shared-scope assignment a startup template makes.
type startupAssign struct {
	key string // "request.context", lowercased
	rhs string
	dir string // the template's directory, for resolving what the RHS names
}

// splitSharedScope splits "REQUEST.context" into its lowercased key, or reports
// that variable is not a shared-scope variable of one segment.
func splitSharedScope(variable string) (string, bool) {
	scope, name, ok := strings.Cut(variable, ".")
	if !ok || name == "" || strings.ContainsAny(name, ".[(") {
		return "", false
	}

	scope = strings.ToLower(scope)
	if !slices.Contains(sharedScopes, scope) {
		return "", false
	}

	return scope + "." + strings.ToLower(name), true
}

// startupAssigns returns the shared-scope assignments made by the templates
// the Application.cfc (or .cfm) governing baseDir includes, transitively, and
// by that file itself, then by the configured StartupFiles and what they
// include, in the order they are met. It is cached per application root —
// "" for a file under none, which the configured files still reach — for the
// life of the resolver, which the server drops wherever a path answer may
// have gone stale.
func (r *Resolver) startupAssigns(baseDir string) []startupAssign {
	appDir := r.FindApplicationRoot(baseDir)
	if appDir == "" && len(r.StartupFiles) == 0 {
		return nil
	}

	r.mu.RLock()
	got, ok := r.startupCache[appDir]
	r.mu.RUnlock()

	if ok {
		return got
	}

	var (
		out   []startupAssign
		seen  = map[string]bool{}
		queue []string
	)

	if appDir != "" {
		for _, name := range []string{"Application.cfc", "Application.cfm"} {
			queue = append(queue, filepath.Join(appDir, name))
		}
	}

	queue = append(queue, r.configuredStartupFiles()...)

	for len(queue) > 0 && len(seen) < maxStartupTemplates {
		file := queue[0]
		queue = queue[1:]

		if seen[pathKey(file)] {
			continue
		}

		seen[pathKey(file)] = true

		data, err := r.fs().ReadFile(file)
		if err != nil {
			continue
		}

		content := string(data)
		out = append(out, sharedAssignments(content, filepath.Dir(file))...)

		for _, raw := range parser.ExtractIncludes(content) {
			if target := r.IncludePath(raw, file); target != "" {
				queue = append(queue, target)
			}
		}
	}

	r.mu.Lock()
	if r.startupCache == nil {
		r.startupCache = map[string][]startupAssign{}
	}

	r.startupCache[appDir] = out
	r.mu.Unlock()

	return out
}

// configuredStartupFiles is StartupFiles as files on disk. An entry that names
// a file is used as it is; one that does not and starts with "/" is a CFML
// template path, resolved as a cfinclude of it would be — through the
// mappings, the workspace folders and a folder named by its first segment —
// so "/tassweb/packages/tass/core/bootstrap.cfm" is written the way every
// Application.cfc that includes the template writes it, and reads the same
// from any application's config.
func (r *Resolver) configuredStartupFiles() []string {
	out := make([]string, 0, len(r.StartupFiles))

	for _, p := range r.StartupFiles {
		if info, err := r.fs().Stat(p); err == nil && !info.IsDir() {
			out = append(out, p)

			continue
		}

		if !strings.HasPrefix(filepath.ToSlash(p), "/") || len(r.WorkspaceFolders) == 0 {
			continue
		}

		// IncludePath tries the including file's own directory first; a
		// template path from config has no including file, and a leading
		// slash makes that candidate a workspace-folder one anyway.
		if target := r.IncludePath(filepath.ToSlash(p), filepath.Join(r.WorkspaceFolders[0], "startupFiles")); target != "" {
			out = append(out, target)
		}
	}

	return out
}

// sharedAssignments finds `SCOPE.name = rhs` in content, in tag and in script
// syntax, one assignment per line.
func sharedAssignments(content, dir string) []startupAssign {
	var out []startupAssign

	for line := range strings.SplitSeq(content, "\n") {
		m := tagSharedAssign.FindStringSubmatch(line)
		if m == nil {
			m = scriptSharedAssign.FindStringSubmatch(line)
		}

		if m == nil {
			continue
		}

		out = append(out, startupAssign{
			key: strings.ToLower(m[1]) + "." + strings.ToLower(m[2]),
			rhs: strings.TrimSpace(m[3]),
			dir: dir,
		})
	}

	return out
}

// startupComponent is the component a shared-scope variable holds according
// to the startup templates of the application governing baseDir, or "" when
// they do not say. Several assignments that disagree name every component
// they give, pipe-separated, as a componentResolver may.
func (r *Resolver) startupComponent(variable, baseDir string, tr *callTrace) string {
	key, ok := splitSharedScope(variable)
	if !ok {
		return ""
	}

	assigns := r.startupAssigns(baseDir)
	if len(assigns) == 0 {
		return ""
	}

	comp := r.typeOfShared(key, assigns, map[string]bool{})
	if comp != "" {
		tr.addf("resolved %q to %q from an assignment in a startup template (one the Application.cfc includes, or a configured startupFiles entry)", variable, comp)
	}

	return comp
}

// typeOfShared types key from every assignment of it in assigns.
func (r *Resolver) typeOfShared(key string, assigns []startupAssign, visiting map[string]bool) string {
	if visiting[key] {
		return ""
	}

	visiting[key] = true
	defer delete(visiting, key)

	var comps []string

	for _, a := range assigns {
		if a.key != key {
			continue
		}

		if c := r.typeOfExpr(a.rhs, a.dir, assigns, visiting); c != "" && !slices.Contains(comps, c) {
			comps = append(comps, c)
		}
	}

	return strings.Join(comps, "|")
}

// typeOfExpr types a startup assignment's right-hand side: a created
// component, another shared variable, or a chain of calls on either, walked
// through each hop's return type. Anything else is not typed.
func (r *Resolver) typeOfExpr(rhs, dir string, assigns []startupAssign, visiting map[string]bool) string {
	if m := createdComponent.FindStringSubmatch(rhs); m != nil {
		return r.staticPath(m[1])
	}

	if m := newComponent.FindStringSubmatch(rhs); m != nil {
		return r.staticPath(m[1])
	}

	if !chainExpr.MatchString(rhs) {
		return ""
	}

	segs := strings.Split(stripCallArgs(rhs), ".")

	// The receiver is every segment before the first call; the hops, the calls.
	first := len(segs)

	for i, s := range segs {
		if strings.HasSuffix(s, "()") {
			first = i

			break
		}
	}

	if first == 0 {
		return ""
	}

	base := strings.Join(segs[:first], ".")

	comp := ""
	if key, ok := splitSharedScope(base); ok {
		comp = r.typeOfShared(key, assigns, visiting)
	}

	if comp == "" {
		comp, _, _ = r.matchResolver(base, nil, nil)
	}

	for _, s := range segs[first:] {
		if comp == "" || comp == "$any" {
			return ""
		}

		hop := strings.TrimSuffix(s, "()")

		fd := r.ResolveFunc(comp, hop, dir)
		if fd == nil {
			return ""
		}

		comp, _, _ = r.chainHopReturn(comp, hop, fd, nil)
	}

	// A return type is recorded as written, runtime expressions and all:
	// kernel2's getContextObject() returns "#VARIABLES._core#context".
	if comp != "" && comp != "$any" {
		comp = r.staticPath(comp)
	}

	return comp
}

// stripCallArgs empties every argument list in a chain, so it splits on dots:
// a.b(x.y).c() is a.b().c(). chainExpr has already refused nested parens.
func stripCallArgs(s string) string {
	var b strings.Builder

	depth := 0

	for _, c := range s {
		switch {
		case c == '(':
			if depth == 0 {
				b.WriteRune(c)
			}

			depth++
		case c == ')':
			depth--
			if depth == 0 {
				b.WriteRune(c)
			}
		case depth == 0:
			b.WriteRune(c)
		}
	}

	return b.String()
}

// staticPath is a created component's path with the expression mappings
// applied, or "" when a runtime expression is left in it: "#x#.core.context"
// names nothing the resolver can find.
func (r *Resolver) staticPath(p string) string {
	for _, key := range r.expressionKeys() {
		for expr := range strings.SplitSeq(key, "|") {
			if expr != "" {
				p = strings.ReplaceAll(p, expr, r.ExpressionMappings[key])
			}
		}
	}

	if strings.Contains(p, "#") {
		return ""
	}

	return p
}
