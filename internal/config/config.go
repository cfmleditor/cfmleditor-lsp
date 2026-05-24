// Package config defines the shared .cfmleditor.json configuration types.
package config

import "path/filepath"

// JSON is the on-disk shape of .cfmleditor.json.
type JSON struct {
	WorkspaceName       string            `json:"workspaceName"`
	WorkspacePaths      []string          `json:"workspacePaths"`
	WorkspaceIndexGlobs []string          `json:"workspaceIndexGlobs"`
	Mappings            map[string]string `json:"mappings"`
	ComponentResolvers  []Resolver        `json:"componentResolvers"`
	PropertyResolvers   []PropResolver    `json:"propertyResolvers"`
	BeanPaths           map[string]string `json:"beanPaths"`
	Formatting          *Formatting       `json:"formatting"`
	Linting             *Linting          `json:"linting"`
	Completions         *Completions      `json:"completions"`
	Debug               bool              `json:"debug"`
}

// Resolver maps a call pattern to a component path.
type Resolver struct {
	Match   string `json:"match"`
	Resolve string `json:"resolve"`
	Prefix  string `json:"prefix"`
}

// PropResolver maps a property attribute to a component path.
type PropResolver struct {
	Match     string `json:"match"`
	Resolve   string `json:"resolve"`
	Attribute string `json:"attribute"`
}

// Linting holds linting configuration.
type Linting struct {
	Enabled bool `json:"enabled"`
}

// Completions holds completion configuration.
type Completions struct {
	TagSnippets      bool `json:"tagSnippets"`
	FunctionSnippets bool `json:"functionSnippets"`
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
	Mappings           map[string]string
	ComponentResolvers []Resolver
	PropertyResolvers  []PropResolver
	BeanPaths          map[string]string
	Formatting         ResolvedFormatting
	Linting            bool
	TagSnippets        bool
	FunctionSnippets   bool
}

// ResolvedFormatting holds formatting settings with defaults applied.
type ResolvedFormatting struct {
	Enabled               bool
	Debug                 bool
	SelfCloseTags         bool
	WhitespaceOnly        bool
	QueryFormat           bool
	LowercaseTags         bool
	LowercaseAttributes   bool
	DoubleQuoteAttributes bool
	QueryUppercaseKeywords bool
	ScopeCase             string
	CommaPosition         string
	QueryCommaPosition    string
	LineWidth             int
	AttrBreakThreshold    int
	IndentWidth           int
}

// Resolve takes a parsed JSON config and its directory, returning a fully resolved config.
func Resolve(cfg *JSON, dir string) *Resolved {
	r := &Resolved{}
	if len(cfg.Mappings) > 0 {
		r.Mappings = ResolvePaths(cfg.Mappings, dir)
	}
	for _, cr := range cfg.ComponentResolvers {
		if cr.Match != "" && cr.Resolve != "" {
			r.ComponentResolvers = append(r.ComponentResolvers, cr)
		}
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
	} else {
		r.TagSnippets = true
		r.FunctionSnippets = true
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
