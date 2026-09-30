package parser

import (
	"fmt"
	"testing"
)

func TestColonNamedArgumentTypesResolverAssignment(t *testing.T) {
	for _, assignment := range []string{
		`var svc = getService(name: "users", initArguments: { count: helper.check() }); return svc;`,
		`variables.svc = getService(name: "users"); return variables.svc;`,
		`this.svc = getService(name: "users"); return this.svc;`,
		`var svc = factory.getService(name: "users"); return svc;`,
	} {
		t.Run(assignment, func(t *testing.T) {
			pr := ParseWithOptions(testURI, fmt.Sprintf("component { function make() { %s } }", assignment), &ParseOptions{
				Resolvers:    []Resolver{{Match: `getService("$1")`, Resolve: "services.$1", Prefix: "getService"}},
				ExtractCalls: true,
			})
			if len(pr.Funcs) != 1 {
				t.Fatalf("got %d functions, want 1", len(pr.Funcs))
			}

			if got := pr.Funcs[0].ReturnComponent; got != "services.users" {
				t.Errorf("returned assignment type = %q, want services.users", got)
			}
		})
	}
}

func TestComputedNamedArgumentDoesNotUseLiteralPrefix(t *testing.T) {
	for _, separator := range []string{"=", ":"} {
		pr := ParseWithOptions(testURI, fmt.Sprintf(`component {
function make(suffix) { var svc = getService(name %s "users" & suffix); return svc; }
}`, separator), &ParseOptions{
			Resolvers: []Resolver{{Match: `getService("$1")`, Resolve: "services.$1", Prefix: "getService"}},
		})
		if len(pr.Funcs) != 1 {
			t.Fatal("expected one function")
		}

		if got := pr.Funcs[0].ReturnComponent; got != "" {
			t.Errorf("computed id with %s returns %q, want no component", separator, got)
		}
	}
}
