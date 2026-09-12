package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// dispatchedScopes are the scope keywords that must have a case in *both* of
// script_parser.go's dispatch switches — scriptParser.parse() for statements at
// component level, and handleBodyToken for statements inside a function.
//
// A scope missing from either falls through to checkAssignRef's default path,
// which only recognises a bare `x = ...`: for a scope-prefixed left-hand side
// the token after the identifier is `.` rather than `=`, so the statement is
// silently read as a bare call and any component type its right-hand side
// established is dropped. Nothing fails; the type simply is not there, and
// every call through that variable becomes an unresolved-call error somewhere
// far away.
//
// CLAUDE.md has warned about this pairing in prose since the two switches
// existed. This is that warning made executable — the same reason the
// reflective test over mergeFormatting's field list exists, and that one caught
// two real omissions.
//
// url/form/cookie/cgi/client/server are deliberately absent from both switches:
// those scopes hold primitive request and config data, not component
// instances. Adding one to the switches means adding it here.
var dispatchedScopes = []string{
	"local",
	"arguments",
	"this",
	"variables",
	"request",
	"session",
	"application",
}

// TestScopedAssignmentEstablishesAComponentRef covers the component-level
// switch: `scope.x = new pkg.Thing()` written directly in a component body.
func TestScopedAssignmentEstablishesAComponentRef(t *testing.T) {
	for _, scope := range dispatchedScopes {
		// `local.` and `arguments.` have no meaning outside a function, so the
		// component-level switch is only reachable for the rest.
		if scope == "local" || scope == "arguments" {
			continue
		}

		content := fmt.Sprintf("component {\n\t%s.svc = new pkg.Thing();\n}\n", scope)

		pr := Parse(testURI, content)
		if !hasComponentRef(pr.ComponentRefs, "svc", "pkg.Thing") {
			t.Errorf("%s.svc = new pkg.Thing() established no component ref at component level — "+
				"scriptParser.parse() is probably missing a %q case (see script_parser.go)",
				scope, scope)
		}
	}
}

// TestScopedAssignmentInAFunctionBodyEstablishesAComponentRef covers the other
// switch, handleBodyToken. A scope can be handled at component level and missed
// here, or the reverse, which is exactly why both are checked.
func TestScopedAssignmentInAFunctionBodyEstablishesAComponentRef(t *testing.T) {
	for _, scope := range dispatchedScopes {
		content := fmt.Sprintf("component {\n\tfunction f() {\n\t\t%s.svc = new pkg.Thing();\n\t}\n}\n", scope)

		pr := Parse(testURI, content)
		if len(pr.Scopes) == 0 {
			t.Fatalf("%s: no function scope parsed", scope)
		}

		s := pr.Scopes[0]

		refs := append([]ComponentRef{}, pr.FuncComponentRefs(s.Start, s.End)...)
		refs = append(refs, pr.ComponentRefs...)

		if !hasComponentRef(refs, "svc", "pkg.Thing") {
			t.Errorf("%s.svc = new pkg.Thing() established no component ref inside a function — "+
				"handleBodyToken is probably missing a %q case (see script_parser.go)",
				scope, scope)
		}
	}
}

// TestBothDispatchSwitchesHandleEveryScope enforces the pairing directly, by
// reading the two switches out of script_parser.go and requiring a case for
// every scope in each.
//
// The behavioural tests above are the better check where they discriminate, but
// they do not everywhere: `this.x = new pkg.Thing()` inside a function body
// still produces a ref with handleBodyToken's `this` case removed, because
// another path happens to cover that shape. The documented rule is structural —
// "each handled scope needs its own case in *both* dispatch switches" — so this
// checks the structure, and catches a missing case for every scope rather than
// only the ones whose absence has an observable this test happens to assert.
func TestBothDispatchSwitchesHandleEveryScope(t *testing.T) {
	src := readParserSource(t, "script_parser.go")

	switches := map[string]string{
		"scriptParser.parse":           funcBody(t, src, "func (p *scriptParser) parse() {"),
		"scriptParser.handleBodyToken": funcBody(t, src, "func (p *scriptParser) handleBodyToken(tok Token, depth int) {"),
	}

	for name, body := range switches {
		for _, scope := range dispatchedScopes {
			// `local` and `arguments` are function-only, so the component-level
			// switch has cases for them but they cannot be exercised there.
			if !strings.Contains(body, `"`+scope+`"`) {
				t.Errorf("%s has no case for %q — a scope-prefixed assignment there falls through "+
					"to checkAssignRef and its component type is silently dropped (see CLAUDE.md)",
					name, scope)
			}
		}

		// The scopes deliberately left out of both switches. If one gains a
		// case it needs a line in dispatchedScopes, or it is dispatched and
		// unchecked.
		for _, scope := range []string{"url", "form", "cookie", "cgi", "client", "server"} {
			if strings.Contains(body, `case "`+scope+`"`) {
				t.Errorf("%s now dispatches %q; add it to dispatchedScopes so both switches are "+
					"checked for it", name, scope)
			}
		}
	}
}

// funcBody returns the source of the function starting at header, up to the
// next top-level declaration.
func funcBody(t *testing.T, src, header string) string {
	t.Helper()

	at := strings.Index(src, header)
	if at < 0 {
		t.Fatalf("script_parser.go no longer contains %q — this test's anchors need updating", header)
	}

	rest := src[at+len(header):]
	if end := strings.Index(rest, "\nfunc "); end >= 0 {
		return rest[:end]
	}

	return rest
}

// readParserSource reads a file from this package's own directory, located
// relative to this source file so the test does not depend on the working
// directory.
func readParserSource(t *testing.T, name string) string {
	t.Helper()

	_, self, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate this source file")
	}

	data, err := os.ReadFile(filepath.Join(filepath.Dir(self), name))
	if err != nil {
		t.Fatal(err)
	}

	return string(data)
}

func hasComponentRef(refs []ComponentRef, variable, component string) bool {
	for _, r := range refs {
		if strings.EqualFold(r.Variable, variable) && strings.EqualFold(r.Component, component) {
			return true
		}
	}

	return false
}
