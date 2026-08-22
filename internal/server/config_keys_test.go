package server

import "testing"

// Both keys are parsed by config.JSON, merged by config.Merge and — since this
// change — carried by config.Resolve, but applyConfig is the last hop into the
// Server. A key it does not copy is a key the user can set, the loader can
// read, and nothing will ever act on.
func TestInitializeAppliesExpressionMappings(t *testing.T) {
	s := initializeWithConfig(t, `{
		"mappings": {"models": "./src/models"},
		"expressionMappings": {"#VARIABLES._core#": "packages.tass.core."}
	}`)

	if got := s.ExpressionMappings["#VARIABLES._core#"]; got != "packages.tass.core." {
		t.Errorf("expressionMappings not applied to the server: got %q", got)
	}
}

func TestInitializeAppliesServicePropertyResolvers(t *testing.T) {
	s := initializeWithConfig(t, `{
		"mappings": {"models": "./src/models"},
		"servicePropertyResolvers": {"package": "packages.${name}"}
	}`)

	if got := s.ServicePropertyResolvers["package"]; got != "packages.${name}" {
		t.Errorf("servicePropertyResolvers not applied to the server: got %q", got)
	}
}
