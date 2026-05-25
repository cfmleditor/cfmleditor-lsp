package parser

import (
	"path/filepath"
	"testing"
)

func TestParseApplicationMappings_ExpandPath(t *testing.T) {
	content := `component {
		this.mappings["/models"] = expandPath("./src/models");
		this.mappings["/lib"] = expandPath("./lib");
	}`
	appDir := "/project"
	got := ParseApplicationMappings(content, appDir)
	if got["models"] != filepath.Join(appDir, "src/models") {
		t.Errorf("models = %q, want %q", got["models"], filepath.Join(appDir, "src/models"))
	}
	if got["lib"] != filepath.Join(appDir, "lib") {
		t.Errorf("lib = %q, want %q", got["lib"], filepath.Join(appDir, "lib"))
	}
}

func TestParseApplicationMappings_PlainString(t *testing.T) {
	content := `this.mappings["/vendor"] = "/opt/cfml/vendor";`
	got := ParseApplicationMappings(content, "/project")
	if got["vendor"] != "/opt/cfml/vendor" {
		t.Errorf("vendor = %q, want %q", got["vendor"], "/opt/cfml/vendor")
	}
}

func TestParseApplicationMappings_SingleQuotes(t *testing.T) {
	content := `this.mappings['/utils'] = expandPath('./utils');`
	got := ParseApplicationMappings(content, "/app")
	if got["utils"] != filepath.Join("/app", "utils") {
		t.Errorf("utils = %q, want %q", got["utils"], filepath.Join("/app", "utils"))
	}
}

func TestParseApplicationMappings_NoMappings(t *testing.T) {
	content := `component { this.name = "myApp"; }`
	got := ParseApplicationMappings(content, "/project")
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestParseApplicationMappings_RelativeStringValue(t *testing.T) {
	content := `this.mappings["/shared"] = "./shared/lib";`
	got := ParseApplicationMappings(content, "/project")
	if got["shared"] != filepath.Join("/project", "shared/lib") {
		t.Errorf("shared = %q, want %q", got["shared"], filepath.Join("/project", "shared/lib"))
	}
}

func TestParseAppBeanPaths_ExplicitStruct(t *testing.T) {
	content := `component {
	this.name = "myApp";
	this.beanPaths[""] = expandPath("./model");
	this.beanPaths["dao"] = expandPath("./model/dao");
	this.beanPaths["services"] = expandPath("./services");
}`
	result := ParseAppBeanPaths(content, "/app")

	tests := map[string]string{
		"":         "/app/model",
		"dao":      "/app/model/dao",
		"services": "/app/services",
	}
	for ns, expected := range tests {
		if result[ns] != expected {
			t.Errorf("namespace %q: got %q, want %q", ns, result[ns], expected)
		}
	}
}

func TestParseAppBeanPaths_DiLocations_Single(t *testing.T) {
	content := `component {
	this.name = "fw1App";
	variables.framework.diLocations = "model";
}`
	result := ParseAppBeanPaths(content, "/app")

	if result[""] != "/app/model" {
		t.Errorf("expected /app/model for empty namespace, got %q", result[""])
	}
}

func TestParseAppBeanPaths_DiLocations_Multiple(t *testing.T) {
	content := `component {
	this.name = "fw1App";
	variables.framework.diLocations = "model,common/beans";
}`
	result := ParseAppBeanPaths(content, "/app")

	if result["model"] != "/app/model" {
		t.Errorf("expected /app/model for 'model' namespace, got %q", result["model"])
	}
	if result["beans"] != "/app/common/beans" {
		t.Errorf("expected /app/common/beans for 'beans' namespace, got %q", result["beans"])
	}
}

func TestParseAppBeanPaths_DiLocations_FrameworkShorthand(t *testing.T) {
	content := `component {
	framework.diLocations = "model";
}`
	result := ParseAppBeanPaths(content, "/app")

	if result[""] != "/app/model" {
		t.Errorf("expected /app/model, got %q", result[""])
	}
}

func TestParseAppBeanPaths_Empty(t *testing.T) {
	content := `component {
	this.name = "noBeansApp";
}`
	result := ParseAppBeanPaths(content, "/app")
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestParseAppBeanPaths_ExplicitTakesPrecedence(t *testing.T) {
	content := `component {
	this.beanPaths[""] = expandPath("./beans");
	variables.framework.diLocations = "model";
}`
	result := ParseAppBeanPaths(content, "/app")

	if result[""] != "/app/beans" {
		t.Errorf("expected /app/beans (explicit), got %q", result[""])
	}
	if _, ok := result["model"]; ok {
		t.Error("diLocations should not be parsed when beanPaths exists")
	}
}

func TestParseOrmLocations_Single(t *testing.T) {
	content := `this.ormSettings = { cfcLocation: "model/entities" };`
	got := ParseOrmLocations(content, "/app")
	if len(got) != 1 || got[0] != "/app/model/entities" {
		t.Errorf("got %v, want [/app/model/entities]", got)
	}
}

func TestParseOrmLocations_Array(t *testing.T) {
	content := `this.ormSettings = { cfcLocation: ["model", "entities"] };`
	got := ParseOrmLocations(content, "/app")
	if len(got) != 2 || got[0] != "/app/model" || got[1] != "/app/entities" {
		t.Errorf("got %v, want [/app/model /app/entities]", got)
	}
}

func TestParseOrmLocations_Empty(t *testing.T) {
	content := `component { this.name = "noOrm"; }`
	got := ParseOrmLocations(content, "/app")
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}
