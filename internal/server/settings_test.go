package server

import (
	"reflect"
	"testing"

	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"

	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
)

// Every field of Settings must reach the Server. The failure this guards was
// not a wrong value but a missing assignment: daemon mode configured its
// stdio session and its socket sessions from two separate blocks of code, and
// two keys were only ever written in one of them.
//
// Comparing field-by-field via reflection rather than asserting a hand-written
// list means a field added to Settings and forgotten in Apply fails here,
// which is the mistake worth catching.
func TestSettingsApplyCoversEveryField(t *testing.T) {
	set := Settings{
		WorkspaceFolders:         []string{"/w"},
		IndexGlobs:               []string{"**/*.cfc"},
		Mappings:                 map[string]string{"models": "/w/models"},
		ExpressionMappings:       map[string]string{"#CORE#": "packages.core."},
		ServicePropertyResolvers: map[string]string{"package": "packages.${name}"},
		ComponentResolvers:       []config.Resolver{{Match: "getService(\"$1\")", Resolve: "services.$1", Prefix: "getService"}},
		PropertyResolvers:        []config.PropResolver{{Match: "$1", Resolve: "beans.$1", Attribute: "name"}},
		BeanPaths:                map[string]string{"svc": "/w/services"},
		Formatting:               config.ResolvedFormatting{Enabled: true, LineWidth: 123},
		Linting:                  true,
	}

	srv := NewServer(nil, cflog.NewLogger(false))
	set.Apply(srv)

	setVal, srvVal := reflect.ValueOf(set), reflect.ValueOf(srv).Elem()

	for i := range setVal.NumField() {
		name := setVal.Type().Field(i).Name

		field := srvVal.FieldByName(name)
		if !field.IsValid() {
			t.Errorf("Settings.%s has no matching Server field", name)

			continue
		}

		if field.IsZero() {
			t.Errorf("Settings.%s was not applied to the Server", name)
		}
	}
}

// Apply must give the two session kinds identical configuration.
func TestSettingsApplyIsIdenticalAcrossSessions(t *testing.T) {
	set := Settings{
		Mappings:                 map[string]string{"models": "/w/models"},
		ExpressionMappings:       map[string]string{"#CORE#": "packages.core."},
		ServicePropertyResolvers: map[string]string{"package": "packages.${name}"},
		BeanPaths:                map[string]string{"svc": "/w/services"},
	}

	stdio := NewServer(nil, cflog.NewLogger(false))
	socket := NewServer(nil, cflog.NewLogger(false))

	set.Apply(stdio)
	set.Apply(socket)

	for _, tc := range []struct {
		name string
		a, b map[string]string
	}{
		{"Mappings", stdio.Mappings, socket.Mappings},
		{"ExpressionMappings", stdio.ExpressionMappings, socket.ExpressionMappings},
		{"ServicePropertyResolvers", stdio.ServicePropertyResolvers, socket.ServicePropertyResolvers},
		{"BeanPaths", stdio.BeanPaths, socket.BeanPaths},
	} {
		if !reflect.DeepEqual(tc.a, tc.b) {
			t.Errorf("%s differs between a stdio and a socket session: %v vs %v", tc.name, tc.a, tc.b)
		}
	}
}
