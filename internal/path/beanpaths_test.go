package path

import (
	"testing"
)

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
	// If both this.beanPaths and diLocations exist, beanPaths wins
	content := `component {
	this.beanPaths[""] = expandPath("./beans");
	variables.framework.diLocations = "model";
}`
	result := ParseAppBeanPaths(content, "/app")

	if result[""] != "/app/beans" {
		t.Errorf("expected /app/beans (explicit), got %q", result[""])
	}
	// diLocations should not be parsed since beanPaths was found
	if _, ok := result["model"]; ok {
		t.Error("diLocations should not be parsed when beanPaths exists")
	}
}
