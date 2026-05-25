package parser

import (
	"testing"

	"go.lsp.dev/uri"
)

func TestPropertyAccessors_Script(t *testing.T) {
	content := `component {
	property name="person" type="models.Person";
	property name="age" type="numeric";
	function init() {}
}`
	pr := Parse(uri.URI("file:///test.cfc"), content)

	// Should generate getPerson, setPerson, getAge, setAge + init
	funcNames := make(map[string]bool)
	for _, f := range pr.Funcs {
		funcNames[f.Name] = true
	}
	for _, expected := range []string{"getPerson", "setPerson", "getAge", "setAge", "init"} {
		if !funcNames[expected] {
			t.Errorf("expected function %s, not found", expected)
		}
	}
}

func TestPropertyAccessors_Tag(t *testing.T) {
	content := `<cfcomponent>
	<cfproperty name="userDAO" type="model.UserDAO" />
	<cfproperty name="dsn" type="string" />
	<cffunction name="init"><cfreturn this /></cffunction>
</cfcomponent>`
	pr := Parse(uri.URI("file:///test.cfc"), content)

	funcNames := make(map[string]bool)
	for _, f := range pr.Funcs {
		funcNames[f.Name] = true
	}
	for _, expected := range []string{"getUserDAO", "setUserDAO", "getDsn", "setDsn", "init"} {
		if !funcNames[expected] {
			t.Errorf("expected function %s, not found", expected)
		}
	}
}

func TestPropertyAccessors_PositionalSyntax(t *testing.T) {
	content := `component {
	property string name;
	property numeric age;
	property UserDAO userDAO;
}`
	pr := Parse(uri.URI("file:///test.cfc"), content)

	funcNames := make(map[string]bool)
	for _, f := range pr.Funcs {
		funcNames[f.Name] = true
	}
	for _, expected := range []string{"getName", "setName", "getAge", "setAge", "getUserDAO", "setUserDAO"} {
		if !funcNames[expected] {
			t.Errorf("expected function %s, not found", expected)
		}
	}
	// UserDAO type should generate a component ref
	found := false
	for _, ref := range pr.Refs {
		if ref.Variable == "userDAO" && ref.Component == "UserDAO" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected ComponentRef for userDAO → UserDAO")
	}
}

func TestPropertyAccessors_NoOverrideExplicit(t *testing.T) {
	content := `component {
	property name="name" type="string";
	function getName() { return variables.name; }
}`
	pr := Parse(uri.URI("file:///test.cfc"), content)

	// getName should appear only once (the explicit one at line 2)
	count := 0
	for _, f := range pr.Funcs {
		if f.Name == "getName" {
			count++
			if f.Line != 2 {
				t.Errorf("getName should be at line 2 (explicit), got %d", f.Line)
			}
		}
	}
	if count != 1 {
		t.Errorf("expected 1 getName, got %d", count)
	}
	// setName should still be generated
	found := false
	for _, f := range pr.Funcs {
		if f.Name == "setName" {
			found = true
		}
	}
	if !found {
		t.Error("expected synthetic setName to be generated")
	}
}

func TestPropertyAccessors_NoDuplicates(t *testing.T) {
	content := `component {
	property name="x" type="string";
	property name="x" type="string";
}`
	pr := Parse(uri.URI("file:///test.cfc"), content)

	count := 0
	for _, f := range pr.Funcs {
		if f.Name == "getX" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 getX (no duplicates), got %d", count)
	}
}

func TestPropertyAccessors_TypeComponentRef(t *testing.T) {
	content := `component {
	property name="userService" type="services.UserService";
	property name="count" type="numeric";
}`
	pr := Parse(uri.URI("file:///test.cfc"), content)

	// services.UserService should generate a ref, numeric should not
	var refs []string
	for _, ref := range pr.Refs {
		refs = append(refs, ref.Variable+"→"+ref.Component)
	}
	found := false
	for _, ref := range pr.Refs {
		if ref.Variable == "userService" && ref.Component == "services.UserService" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected ref userService→services.UserService, got %v", refs)
	}
	for _, ref := range pr.Refs {
		if ref.Variable == "count" {
			t.Error("numeric property should not generate a component ref")
		}
	}
}

func TestPropertyAccessors_VariablesScope(t *testing.T) {
	content := `component {
	property name="userDAO" type="model.UserDAO";
	property name="logger" type="string";
}`
	pr := Parse(uri.URI("file:///test.cfc"), content)

	vars := pr.VariablesVars()
	varMap := make(map[string]bool)
	for _, v := range vars {
		varMap[v] = true
	}
	if !varMap["userDAO"] {
		t.Error("expected userDAO in variables scope")
	}
	if !varMap["logger"] {
		t.Error("expected logger in variables scope")
	}
}

func TestPropertyAccessors_InjectAttr(t *testing.T) {
	content := `component {
	property name="userDAO" inject="UserDAO@dao";
	property name="config" inject="coldbox:setting:appConfig";
}`
	pr := Parse(uri.URI("file:///test.cfc"), content)

	// Without property resolvers or bean lookup, inject alone doesn't create refs
	// (type is empty and not a CFC path)
	for _, ref := range pr.Refs {
		if ref.Variable == "config" {
			t.Error("inject with coldbox namespace should not create ref without resolvers")
		}
	}
}

func TestPropertyAccessors_BeanLookup(t *testing.T) {
	beans := map[string]string{
		"userdao":          "dao.UserDAO",
		"userdao@dao":      "dao.UserDAO",
		"orderdao":         "dao.OrderDAO",
		"userservice@services": "services.UserService",
	}
	lookup := func(name string) string {
		return beans[name]
	}

	content := `component {
	property name="userDAO" inject="UserDAO@dao";
	property name="orderDAO" inject="orderDAO";
	property name="missing" inject="nonexistent";
}`
	pr := ParseWithOptions(uri.URI("file:///test.cfc"), content, ParseOptions{
		BeanLookup: lookup,
	})

	refMap := make(map[string]string)
	for _, ref := range pr.Refs {
		refMap[ref.Variable] = ref.Component
	}
	if refMap["userDAO"] != "dao.UserDAO" {
		t.Errorf("userDAO: expected dao.UserDAO, got %q", refMap["userDAO"])
	}
	if refMap["orderDAO"] != "dao.OrderDAO" {
		t.Errorf("orderDAO: expected dao.OrderDAO, got %q", refMap["orderDAO"])
	}
	if _, ok := refMap["missing"]; ok {
		t.Error("missing should not have a ref")
	}
}

func TestPropertyResolvers(t *testing.T) {
	content := `component {
	property name="userDAO" inject="model.UserDAO";
	property name="logger" inject="coldbox:logger";
	property name="plain" inject="plainBean";
}`
	pr := ParseWithOptions(uri.URI("file:///test.cfc"), content, ParseOptions{
		PropertyResolvers: []PropertyResolver{
			{Match: "model.$1", Resolve: "models.$1", Attribute: "inject"},
			{Match: "coldbox:$1", Resolve: "coldbox.system.$1", Attribute: "inject"},
		},
	})

	refMap := make(map[string]string)
	for _, ref := range pr.Refs {
		refMap[ref.Variable] = ref.Component
	}
	if refMap["userDAO"] != "models.UserDAO" {
		t.Errorf("userDAO: expected models.UserDAO, got %q", refMap["userDAO"])
	}
	if refMap["logger"] != "coldbox.system.logger" {
		t.Errorf("logger: expected coldbox.system.logger, got %q", refMap["logger"])
	}
}

func TestResolveProperty(t *testing.T) {
	resolvers := []PropertyResolver{
		{Match: "model.$1", Resolve: "models.$1", Attribute: "inject"},
		{Match: "$1@dao", Resolve: "dao.$1", Attribute: "inject"},
	}

	tests := []struct {
		attrs    map[string]string
		expected string
	}{
		{map[string]string{"inject": "model.UserDAO"}, "models.UserDAO"},
		{map[string]string{"inject": "OrderDAO@dao"}, "dao.OrderDAO"},
		{map[string]string{"inject": "unmatched"}, ""},
		{map[string]string{"type": "string"}, ""},
	}

	for _, tt := range tests {
		got := ResolveProperty(tt.attrs, resolvers)
		if got != tt.expected {
			t.Errorf("ResolveProperty(%v) = %q, want %q", tt.attrs, got, tt.expected)
		}
	}
}

func TestExtractBeanName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"userService", "userService"},
		{"UserDAO@dao", "UserDAO"},
		{"model:UserService", "UserService"},
		{"coldbox:setting:appName", "appName"},
		{"  spacedBean  ", "spacedBean"},
	}
	for _, tt := range tests {
		got := extractBeanName(tt.input)
		if got != tt.expected {
			t.Errorf("extractBeanName(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestNormalizeBeanKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"userService", "userservice"},
		{"UserDAO@dao", "userdao@dao"},
		{"model:UserService", "userservice@model"},
		{"coldbox:setting:appName", "appname@setting"},
	}
	for _, tt := range tests {
		got := normalizeBeanKey(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeBeanKey(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
