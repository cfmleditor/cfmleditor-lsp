package parser

import "testing"

// Every FuncScope in a ParseResult carries the name of the function that opens
// it, whichever syntax the file is written in.
//
// This pins an invariant rather than a fix: it passed before the change that
// made it worth stating. internal/server/handler.go: scopesToFuncRanges used to
// recover the name by scanning pr.Funcs for a definition on the scope's opening
// line — quadratic in the function count, and held under the server's map lock
// — and now reads sc.Name directly. That is only equivalent while this holds,
// and the two producers that reach pr.Scopes (findTagFuncScopes for tag
// regions, scriptParser for script ones) are far enough apart to be worth
// checking. A third producer, tagParser's own p.scopes, does leave Name unset;
// those are internal to that parser and never merged into pr.Scopes.
func TestFuncScopesAreNamedByBothParsers(t *testing.T) {
	cases := map[string]struct {
		src   string
		names []string
	}{
		"tag": {
			src: `<cfcomponent output="false">
	<cffunction name="getUser" returntype="struct" access="public">
		<cfargument name="id" type="numeric" required="true" />
		<cfreturn {} />
	</cffunction>
	<cffunction name="saveUser" returntype="void" access="private">
		<cfreturn />
	</cffunction>
</cfcomponent>`,
			names: []string{"getUser", "saveUser"},
		},
		"script": {
			src: `component {
	public struct function getUser(required numeric id) {
		return {};
	}
	private void function saveUser() {
	}
}`,
			names: []string{"getUser", "saveUser"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			pr := Parse("file:///Svc.cfc", tc.src)

			if len(pr.Scopes) != len(tc.names) {
				t.Fatalf("got %d scopes, want %d: %+v", len(pr.Scopes), len(tc.names), pr.Scopes)
			}

			for i, want := range tc.names {
				if pr.Scopes[i].Name != want {
					t.Errorf("scope %d name = %q, want %q", i, pr.Scopes[i].Name, want)
				}
			}

			// The name on the scope has to be the name of the definition that
			// opens it, which is the equivalence the server's lookup relied on.
			for _, sc := range pr.Scopes {
				for _, f := range pr.Funcs {
					if int(f.Line) == sc.Start && f.Name != sc.Name {
						t.Errorf("scope at line %d named %q but its definition is %q", sc.Start, sc.Name, f.Name)
					}
				}
			}
		})
	}
}
