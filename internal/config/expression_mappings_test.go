package config

import "testing"

// expressionMappings is parsed by JSON and merged by Merge, but Resolve is what
// the server actually consumes — a key missing there is a key the config file
// can set and nothing will ever read.
func TestResolveCarriesExpressionMappings(t *testing.T) {
	cfg := &JSON{
		ExpressionMappings: map[string]string{
			"#VARIABLES._core#": "packages.tass.core.",
		},
	}

	r := Resolve(cfg, "/tmp")

	if got := r.ExpressionMappings["#VARIABLES._core#"]; got != "packages.tass.core." {
		t.Errorf("expressionMappings did not survive Resolve: got %q", got)
	}
}

func TestResolveCarriesServicePropertyResolvers(t *testing.T) {
	cfg := &JSON{
		ServicePropertyResolvers: map[string]string{
			"package": "packages.${name}",
		},
	}

	r := Resolve(cfg, "/tmp")

	if got := r.ServicePropertyResolvers["package"]; got != "packages.${name}" {
		t.Errorf("servicePropertyResolvers did not survive Resolve: got %q", got)
	}
}
