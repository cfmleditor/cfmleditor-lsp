package config

import "testing"

func TestBoolDefault(t *testing.T) {
	trueVal := true
	falseVal := false

	if !BoolDefault(nil, true) {
		t.Error("expected default true when pointer is nil")
	}

	if BoolDefault(nil, false) {
		t.Error("expected default false when pointer is nil")
	}

	if !BoolDefault(&trueVal, false) {
		t.Error("expected pointer value true to override default false")
	}

	if BoolDefault(&falseVal, true) {
		t.Error("expected pointer value false to override default true")
	}
}

func TestIntDefault(t *testing.T) {
	val := 42

	if got := IntDefault(nil, 100); got != 100 {
		t.Errorf("expected default 100 when pointer is nil, got %d", got)
	}

	if got := IntDefault(&val, 100); got != 42 {
		t.Errorf("expected pointer value 42 to override default, got %d", got)
	}
}

func TestResolvePaths(t *testing.T) {
	if got := ResolvePaths(nil, "/base"); got != nil {
		t.Errorf("expected nil for empty input map, got %v", got)
	}

	if got := ResolvePaths(map[string]string{}, "/base"); got != nil {
		t.Errorf("expected nil for empty (non-nil) input map, got %v", got)
	}

	out := ResolvePaths(map[string]string{
		"rel": "./models",
		"abs": "/already/absolute",
	}, "/base/dir")

	if out["rel"] != "/base/dir/models" {
		t.Errorf("expected relative path joined with baseDir, got %q", out["rel"])
	}

	if out["abs"] != "/already/absolute" {
		t.Errorf("expected absolute path left untouched, got %q", out["abs"])
	}
}

func TestResolve_MappingsAndBeanPaths(t *testing.T) {
	cfg := &JSON{
		Mappings:  map[string]string{"models": "./src/models"},
		BeanPaths: map[string]string{"UserService": "./services/User.cfc"},
	}

	r := Resolve(cfg, "/proj")

	if r.Mappings["models"] != "/proj/src/models" {
		t.Errorf("expected resolved mapping, got %q", r.Mappings["models"])
	}

	if r.BeanPaths["UserService"] != "/proj/services/User.cfc" {
		t.Errorf("expected resolved bean path, got %q", r.BeanPaths["UserService"])
	}

	// Absent maps stay nil rather than becoming an empty non-nil map.
	empty := Resolve(&JSON{}, "/proj")
	if empty.Mappings != nil {
		t.Errorf("expected nil Mappings when config has none, got %v", empty.Mappings)
	}

	if empty.BeanPaths != nil {
		t.Errorf("expected nil BeanPaths when config has none, got %v", empty.BeanPaths)
	}
}

func TestResolve_ComponentResolversFiltering(t *testing.T) {
	cfg := &JSON{
		ComponentResolvers: []Resolver{
			{Match: "getService(\"$1\")", Resolve: "services.$1", Prefix: "getService"}, // valid
			{Match: "", Resolve: "services.Foo", Prefix: "foo"},                         // missing match — dropped
			{Match: "getFoo()", Resolve: "", Prefix: "getFoo"},                          // missing resolve — dropped
		},
	}

	r := Resolve(cfg, "/proj")

	if len(r.ComponentResolvers) != 1 {
		t.Fatalf("expected only the valid resolver to survive filtering, got %d", len(r.ComponentResolvers))
	}

	if r.ComponentResolvers[0].Prefix != "getService" {
		t.Errorf("expected the valid resolver to survive, got %+v", r.ComponentResolvers[0])
	}
}

func TestResolve_PropertyResolversFiltering(t *testing.T) {
	cfg := &JSON{
		PropertyResolvers: []PropResolver{
			{Match: "$1", Resolve: "services.$1", Attribute: "inject"}, // valid
			{Match: "$1", Resolve: "services.$1", Attribute: ""},       // missing attribute — dropped
			{Match: "", Resolve: "services.$1", Attribute: "inject"},   // missing match — dropped
			{Match: "$1", Resolve: "", Attribute: "inject"},            // missing resolve — dropped
		},
	}

	r := Resolve(cfg, "/proj")

	if len(r.PropertyResolvers) != 1 {
		t.Fatalf("expected only the fully-populated resolver to survive filtering, got %d", len(r.PropertyResolvers))
	}
}

func TestResolve_LintingDefault(t *testing.T) {
	if r := Resolve(&JSON{}, "/proj"); r.Linting {
		t.Error("expected Linting false when Linting section is absent")
	}

	enabled := Resolve(&JSON{Linting: &Linting{Enabled: true}}, "/proj")
	if !enabled.Linting {
		t.Error("expected Linting true when explicitly enabled")
	}

	disabled := Resolve(&JSON{Linting: &Linting{Enabled: false}}, "/proj")
	if disabled.Linting {
		t.Error("expected Linting false when explicitly disabled")
	}
}

func TestResolve_CompletionsDefaults(t *testing.T) {
	r := Resolve(&JSON{}, "/proj")
	if !r.TagSnippets || !r.FunctionSnippets || !r.GlobalFunctionResolution {
		t.Errorf("expected all completions to default true when section is absent, got %+v", r)
	}

	// An explicit, all-false Completions section must not be overridden by the defaults.
	allFalse := Resolve(&JSON{Completions: &Completions{}}, "/proj")
	if allFalse.TagSnippets || allFalse.FunctionSnippets || allFalse.GlobalFunctionResolution {
		t.Errorf("expected explicit all-false completions to be respected, got %+v", allFalse)
	}
}

func TestResolve_FormattingAbsentIsZeroValueNotDefaults(t *testing.T) {
	// When the "formatting" section is entirely absent, ResolvedFormatting stays at its
	// zero value (Enabled=false, LineWidth=0, ...) — it does NOT pick up the same defaults
	// (LineWidth=100 etc.) that apply to individually-unset fields within a present section.
	r := Resolve(&JSON{}, "/proj")

	if r.Formatting.Enabled {
		t.Error("expected Formatting.Enabled false when section is absent")
	}

	if r.Formatting.LineWidth != 0 {
		t.Errorf("expected LineWidth 0 (zero value, not the 100 default) when section is absent, got %d", r.Formatting.LineWidth)
	}
}

func TestResolve_FormattingPresentAppliesFieldDefaults(t *testing.T) {
	// When the section IS present but individual pointer fields are nil, those specific
	// fields fall back to their documented defaults.
	r := Resolve(&JSON{Formatting: &Formatting{Enabled: true}}, "/proj")

	if !r.Formatting.Enabled {
		t.Error("expected Enabled to reflect the explicit true")
	}

	if !r.Formatting.SelfCloseTags {
		t.Error("expected SelfCloseTags to default true")
	}

	if !r.Formatting.WhitespaceOnly {
		t.Error("expected WhitespaceOnly to default true")
	}

	if r.Formatting.QueryFormat {
		t.Error("expected QueryFormat to default false")
	}

	if r.Formatting.LineWidth != 100 {
		t.Errorf("expected LineWidth to default 100, got %d", r.Formatting.LineWidth)
	}

	if r.Formatting.AttrBreakThreshold != 4 {
		t.Errorf("expected AttrBreakThreshold to default 4, got %d", r.Formatting.AttrBreakThreshold)
	}

	if r.Formatting.IndentWidth != 4 {
		t.Errorf("expected IndentWidth to default 4, got %d", r.Formatting.IndentWidth)
	}

	// Explicit values override the defaults.
	falseVal := false
	width := 120
	r2 := Resolve(&JSON{Formatting: &Formatting{
		SelfCloseTags: &falseVal,
		LineWidth:     &width,
	}}, "/proj")

	if r2.Formatting.SelfCloseTags {
		t.Error("expected explicit SelfCloseTags=false to override the default")
	}

	if r2.Formatting.LineWidth != 120 {
		t.Errorf("expected explicit LineWidth=120 to override the default, got %d", r2.Formatting.LineWidth)
	}
}
