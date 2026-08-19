// Package config defines the shared .cfmleditor.json configuration types.
package config

import "path/filepath"

// JSON is the on-disk shape of .cfmleditor.json.
type JSON struct {
	WorkspaceName       string            `json:"workspaceName"`
	WorkspacePaths      []string          `json:"workspacePaths"`
	WorkspaceIndexGlobs []string          `json:"workspaceIndexGlobs"`
	Mappings            map[string]string `json:"mappings"`
	ExpressionMappings  map[string]string `json:"expressionMappings"`
	// ServicePropertyResolvers maps a "@serviceproperty" annotation kind (e.g. "package",
	// "service", "controller") to a dot-path template containing "${name}". Recognizes
	// "<!--- @serviceproperty varName kind|name --->" comments as documenting the real
	// component type of a generically-typed (e.g. <cfargument type="struct">) dependency.
	// E.g. {"service": "tassweb.packages.${name}.service"} turns "@serviceproperty
	// objGLJournal service|gljournal" into a ComponentRef for objGLJournal pointing at
	// "tassweb.packages.gljournal.service".
	ServicePropertyResolvers map[string]string `json:"servicePropertyResolvers"`
	ComponentResolvers       []Resolver        `json:"componentResolvers"`
	PropertyResolvers        []PropResolver    `json:"propertyResolvers"`
	BeanPaths                map[string]string `json:"beanPaths"`
	// JavaStubsPath is the dot-path prefix under which java stub CFCs live
	// (e.g. "tassweb.packages.tass.javastubs"). When set, createObject("java",
	// "X") calls are automatically resolved to "<JavaStubsPath>.X" without
	// needing a hand-written componentResolver for the pattern.
	JavaStubsPath string       `json:"javaStubsPath"`
	Formatting    *Formatting  `json:"formatting"`
	Linting       *Linting     `json:"linting"`
	Completions   *Completions `json:"completions"`
	Debug         bool         `json:"debug"`
}

// Resolver maps a call pattern to a component path.
type Resolver struct {
	Match    string `json:"match"`
	Resolve  string `json:"resolve"`
	Prefix   string `json:"prefix"`
	NoFollow bool   `json:"noFollow"`
	// Anchored requires prefix to appear at the start of the call expression
	// instead of anywhere inside it, so a resolver cannot claim an unrelated
	// identifier that merely contains its prefix.
	Anchored bool `json:"anchored"`
}

// PropResolver maps a property attribute to a component path.
type PropResolver struct {
	Match     string `json:"match"`
	Resolve   string `json:"resolve"`
	Attribute string `json:"attribute"`
}

// javaStubCreateObjectPattern matches createObject("java", "some.Class.Name")
// (single or double quotes, arbitrary whitespace inside the parens).
const javaStubCreateObjectPattern = `createObject\s*\(\s*['"]java['"]\s*,\s*['"](.+?)['"]\s*\)`

// JavaStubResolver synthesizes the componentResolver equivalent of hand-writing
// a createObject("java", "X") -> "<javaStubsPath>.X" pattern, so a project only
// needs to set javaStubsPath once instead of writing the regex itself. Returns
// the zero Resolver (Match == "") when javaStubsPath is empty.
func JavaStubResolver(javaStubsPath string) Resolver {
	if javaStubsPath == "" {
		return Resolver{}
	}

	return Resolver{
		Match:   javaStubCreateObjectPattern,
		Resolve: javaStubsPath + ".$1",
		Prefix:  "createObject",
	}
}

// Linting holds linting configuration.
type Linting struct {
	Enabled bool `json:"enabled"`
}

// Completions holds completion configuration.
type Completions struct {
	TagSnippets              bool `json:"tagSnippets"`
	FunctionSnippets         bool `json:"functionSnippets"`
	GlobalFunctionResolution bool `json:"globalFunctionResolution"`
}

// Formatting holds formatter configuration.
type Formatting struct {
	Enabled                bool   `json:"enabled"`
	Debug                  bool   `json:"debug"`
	SelfCloseTags          *bool  `json:"selfCloseTags"`
	WhitespaceOnly         *bool  `json:"whitespaceOnly"`
	QueryFormat            *bool  `json:"queryFormat"`
	LowercaseTags          *bool  `json:"lowercaseTags"`
	LowercaseAttributes    *bool  `json:"lowercaseAttributes"`
	DoubleQuoteAttributes  *bool  `json:"doubleQuoteAttributes"`
	QueryUppercaseKeywords *bool  `json:"queryUppercaseKeywords"`
	ScopeCase              string `json:"scopeCase"`
	CommaPosition          string `json:"commaPosition"`
	QueryCommaPosition     string `json:"queryCommaPosition"`
	LineWidth              *int   `json:"lineWidth"`
	AttrBreakThreshold     *int   `json:"attrBreakThreshold"`
	IndentWidth            *int   `json:"indentWidth"`
}

// BoolDefault returns the value of a *bool or the default if nil.
func BoolDefault(p *bool, def bool) bool {
	if p == nil {
		return def
	}

	return *p
}

// IntDefault returns the value of a *int or the default if nil.
func IntDefault(p *int, def int) int {
	if p == nil {
		return def
	}

	return *p
}

// Resolved holds fully resolved configuration ready for use by the server.
type Resolved struct {
	Mappings                 map[string]string
	ServicePropertyResolvers map[string]string
	ComponentResolvers       []Resolver
	PropertyResolvers        []PropResolver
	BeanPaths                map[string]string
	Formatting               ResolvedFormatting
	Linting                  bool
	TagSnippets              bool
	FunctionSnippets         bool
	GlobalFunctionResolution bool
}

// ResolvedFormatting holds formatting settings with defaults applied.
type ResolvedFormatting struct {
	Enabled                bool
	Debug                  bool
	SelfCloseTags          bool
	WhitespaceOnly         bool
	QueryFormat            bool
	LowercaseTags          bool
	LowercaseAttributes    bool
	DoubleQuoteAttributes  bool
	QueryUppercaseKeywords bool
	ScopeCase              string
	CommaPosition          string
	QueryCommaPosition     string
	LineWidth              int
	AttrBreakThreshold     int
	IndentWidth            int
}

// Resolve takes a parsed JSON config and its directory, returning a fully resolved config.
func Resolve(cfg *JSON, dir string) *Resolved {
	r := &Resolved{}
	if len(cfg.Mappings) > 0 {
		r.Mappings = ResolvePaths(cfg.Mappings, dir)
	}

	if len(cfg.ServicePropertyResolvers) > 0 {
		r.ServicePropertyResolvers = cfg.ServicePropertyResolvers
	}

	for _, cr := range cfg.ComponentResolvers {
		if cr.Match != "" && cr.Resolve != "" {
			r.ComponentResolvers = append(r.ComponentResolvers, cr)
		}
	}

	if jr := JavaStubResolver(cfg.JavaStubsPath); jr.Match != "" {
		r.ComponentResolvers = append(r.ComponentResolvers, jr)
	}

	for _, pr := range cfg.PropertyResolvers {
		if pr.Match != "" && pr.Resolve != "" && pr.Attribute != "" {
			r.PropertyResolvers = append(r.PropertyResolvers, pr)
		}
	}

	if len(cfg.BeanPaths) > 0 {
		r.BeanPaths = ResolvePaths(cfg.BeanPaths, dir)
	}

	if cfg.Linting != nil {
		r.Linting = cfg.Linting.Enabled
	}

	if cfg.Completions != nil {
		r.TagSnippets = cfg.Completions.TagSnippets
		r.FunctionSnippets = cfg.Completions.FunctionSnippets
		r.GlobalFunctionResolution = cfg.Completions.GlobalFunctionResolution
	} else {
		r.TagSnippets = true
		r.FunctionSnippets = true
		r.GlobalFunctionResolution = true
	}

	if f := cfg.Formatting; f != nil {
		r.Formatting = ResolvedFormatting{
			Enabled:                f.Enabled,
			Debug:                  f.Debug,
			SelfCloseTags:          BoolDefault(f.SelfCloseTags, true),
			WhitespaceOnly:         BoolDefault(f.WhitespaceOnly, true),
			QueryFormat:            BoolDefault(f.QueryFormat, false),
			LowercaseTags:          BoolDefault(f.LowercaseTags, true),
			LowercaseAttributes:    BoolDefault(f.LowercaseAttributes, true),
			DoubleQuoteAttributes:  BoolDefault(f.DoubleQuoteAttributes, true),
			QueryUppercaseKeywords: BoolDefault(f.QueryUppercaseKeywords, true),
			ScopeCase:              f.ScopeCase,
			CommaPosition:          f.CommaPosition,
			QueryCommaPosition:     f.QueryCommaPosition,
			LineWidth:              IntDefault(f.LineWidth, 100),
			AttrBreakThreshold:     IntDefault(f.AttrBreakThreshold, 4),
			IndentWidth:            IntDefault(f.IndentWidth, 4),
		}
	}

	return r
}

// ResolvePaths resolves relative paths in a map to absolute using baseDir.
func ResolvePaths(raw map[string]string, baseDir string) map[string]string {
	if len(raw) == 0 {
		return nil
	}

	out := make(map[string]string, len(raw))

	for k, v := range raw {
		if filepath.IsAbs(v) {
			out[k] = v
		} else {
			out[k] = filepath.Join(baseDir, v)
		}
	}

	return out
}

// Merge overlays over onto base and returns the combination. Any field over
// sets wins; fields it leaves unset fall through to base. Either side may be
// nil.
//
// This exists so a lower-priority configuration source (editor-supplied
// initializationOptions) can fill gaps in a higher-priority one (a project's
// .cfmleditor.json) without overriding anything the latter actually states.
// The merge has to happen on JSON rather than Resolved, because Resolve
// substitutes defaults and so loses the distinction between "set to the
// default" and "not set at all".
//
// Mappings and BeanPaths in either side should already be resolved to absolute
// paths, since their relative values are meaningless once the two sides no
// longer share a base directory.
func Merge(base, over *JSON) *JSON {
	switch {
	case base == nil && over == nil:
		return nil
	case base == nil:
		return over
	case over == nil:
		return base
	}

	out := *base

	if over.WorkspaceName != "" {
		out.WorkspaceName = over.WorkspaceName
	}

	if len(over.WorkspacePaths) > 0 {
		out.WorkspacePaths = over.WorkspacePaths
	}

	if len(over.WorkspaceIndexGlobs) > 0 {
		out.WorkspaceIndexGlobs = over.WorkspaceIndexGlobs
	}

	if over.JavaStubsPath != "" {
		out.JavaStubsPath = over.JavaStubsPath
	}

	out.Mappings = mergeStringMap(base.Mappings, over.Mappings)
	out.ExpressionMappings = mergeStringMap(base.ExpressionMappings, over.ExpressionMappings)
	out.ServicePropertyResolvers = mergeStringMap(base.ServicePropertyResolvers, over.ServicePropertyResolvers)
	out.BeanPaths = mergeStringMap(base.BeanPaths, over.BeanPaths)

	// Resolvers from both sides stay active. Order is priority — the first
	// match wins at lookup time — so over's entries lead.
	out.ComponentResolvers = append(append([]Resolver{}, over.ComponentResolvers...), base.ComponentResolvers...)
	out.PropertyResolvers = append(append([]PropResolver{}, over.PropertyResolvers...), base.PropertyResolvers...)

	if over.Formatting != nil {
		out.Formatting = over.Formatting
	}

	if over.Linting != nil {
		out.Linting = over.Linting
	}

	if over.Completions != nil {
		out.Completions = over.Completions
	}

	out.Debug = base.Debug || over.Debug

	return &out
}

// mergeStringMap unions two maps, with over's entries winning per key.
func mergeStringMap(base, over map[string]string) map[string]string {
	if len(base) == 0 {
		return over
	}

	if len(over) == 0 {
		return base
	}

	out := make(map[string]string, len(base)+len(over))
	for k, v := range base {
		out[k] = v
	}

	for k, v := range over {
		out[k] = v
	}

	return out
}
