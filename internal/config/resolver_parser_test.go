package config

import (
	"reflect"
	"testing"

	"github.com/cfmleditor/clif/internal/parser"
)

// TestTagParser_JavaStubResolver_ResolvesCreateObjectJava is the parser-level
// integration test for config.JavaStubResolver: confirms the synthesized
// resolver actually resolves through the parser's own regex/simple-match
// heuristics and $1 substitution, not just via a raw regexp.MatchString check.
func TestTagParser_JavaStubResolver_ResolvesCreateObjectJava(t *testing.T) {
	cr := JavaStubResolver("tassweb.packages.tass.javastubs")
	resolvers := []parser.Resolver{cr.Parser()}

	content := `<cfcomponent>
<cffunction name="work">
	<cfset variables.jss = createObject('java', 'java.security.Signature') />
</cffunction>
</cfcomponent>`

	pr := parser.ParseWithOptions("file:///test.cfc", content, &parser.ParseOptions{Resolvers: resolvers})

	found := ""

	for _, ref := range pr.ComponentRefs {
		if ref.Variable == "jss" {
			found = ref.Component
		}
	}

	want := "tassweb.packages.tass.javastubs.java.security.Signature"
	if found != want {
		t.Errorf("expected jss -> %s, got %q", want, found)
	}
}

// TestParserCarriesEveryResolverField: Parser is the one conversion from a
// configured resolver to the parser's, so a field it forgets is a setting that
// parses and does nothing. Every field is set to a non-zero value and must
// arrive under the same name.
func TestParserCarriesEveryResolverField(t *testing.T) {
	var in Resolver

	v := reflect.ValueOf(&in).Elem()
	for i := range v.NumField() {
		switch f := v.Field(i); f.Kind() {
		case reflect.String:
			f.SetString("x" + v.Type().Field(i).Name)
		case reflect.Bool:
			f.SetBool(true)
		default:
			t.Fatalf("field %s: add its kind to this test", v.Type().Field(i).Name)
		}
	}

	out := in.Parser()
	ov := reflect.ValueOf(&out).Elem()

	for i := range v.NumField() {
		name := v.Type().Field(i).Name

		got := ov.FieldByName(name)
		if !got.IsValid() {
			t.Errorf("parser.Resolver has no field %s", name)

			continue
		}

		if got.Interface() != v.Field(i).Interface() {
			t.Errorf("%s: Parser() gives %v, want %v", name, got.Interface(), v.Field(i).Interface())
		}
	}
}
