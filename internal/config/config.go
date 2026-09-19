// Package config defines the shared .cfmleditor.json configuration types.
package config

import (
	"maps"
	"path/filepath"

	"github.com/cfmleditor/cfmleditor-lsp/internal/route"
)

// CodeMap is the "codemap" block of .cfmleditor.json.
//
// These are properties of the codebase, not of one invocation: which directories
// hold code a runner invokes, which components are infrastructure. Keeping them in
// config rather than on a command line means every person and every tool that
// builds the map describes the same codebase — a map built without them reports
// different dead code and a different set of hubs, and nothing about the result
// says which run was configured correctly.
//
// It lives here rather than in internal/codemap because internal/config is the
// leaf every other package reads: codemap imports parser, and parser's tests
// import config, so config importing codemap is a cycle.
type CodeMap struct {
	// Entry marks files as entry points by path, for code a runner invokes by a
	// constructed name — release scripts, scheduled tasks, plugin directories.
	Entry []string `json:"entry,omitempty"`

	// Utility marks files as infrastructure rather than application code: the
	// logging, the PDF writer, the context accessor every request touches. They
	// are marked, never excluded — see codemap.Options.UtilityGlobs.
	Utility []string `json:"utility,omitempty"`

	// HideUtility opens a generated HTML report with utility code switched off.
	// The toggle is still there and the nodes are still in the page.
	HideUtility bool `json:"hideUtility,omitempty"`
}

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

	// Routes describes a framework's convention for turning a dotted route string
	// into a component, a method and a view file. Static analysis cannot follow a
	// dispatcher, so without it every controller method in a routed application
	// looks uncalled and every view unreferenced. See internal/route.
	Routes route.Config `json:"routes"`

	// CodeMap holds the entry and utility globs the `graph` command uses. They
	// describe the codebase rather than one invocation, so they belong beside the
	// mappings and resolvers rather than on a command line somebody has to
	// remember to repeat.
	CodeMap CodeMap `json:"codemap"`

	ComponentResolvers []Resolver        `json:"componentResolvers"`
	PropertyResolvers  []PropResolver    `json:"propertyResolvers"`
	BeanPaths          map[string]string `json:"beanPaths"`
	// JavaStubsPath is the dot-path prefix under which java stub CFCs live
	// (e.g. "tassweb.packages.tass.javastubs"). When set, createObject("java",
	// "X") calls are automatically resolved to "<JavaStubsPath>.X" without
	// needing a hand-written componentResolver for the pattern.
	JavaStubsPath string       `json:"javaStubsPath"`
	Formatting    *Formatting  `json:"formatting"`
	Linting       *Linting     `json:"linting"`
	Completions   *Completions `json:"completions"`
	References    *References  `json:"references"`
	Features      *Features    `json:"features"`
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
//
// Enabled is a pointer for the reason the completions and features blocks
// document: the block now has more than one key, so a plain bool could not tell
// "the config turned linting off" from "the config named minSeverity and said
// nothing about enabled". Without that distinction a child config setting only
// `minSeverity` would switch linting off in the course of tuning it.
type Linting struct {
	Enabled *bool `json:"enabled"`

	// MinSeverity is the least severe CFLint level still reported, named on
	// CFLint's own scale: FATAL, CRITICAL, ERROR, WARNING, CAUTION, INFO,
	// COSMETIC. Empty — the default — reports everything.
	//
	// It exists because mapSeverity folds INFO and COSMETIC up onto Warning so
	// they are not hidden by an editor that shows neither Hint nor Information
	// by default. That makes every advisory rule as loud as a real one, which
	// on a large legacy file is a lot of yellow; a floor of WARNING is the way
	// back to only the rules worth acting on, and it drops those issues rather
	// than merely making them quiet.
	MinSeverity string `json:"minSeverity"`
}

// References holds textDocument/references configuration.
//
// Off by default, and advertised to the client only when enabled, so a session
// that has not opted in behaves exactly as before: the editor never offers
// "Find All References" and never sends the request. It is a flag rather than
// a plain capability because answering one request walks and parses every CFML
// file under the workspace roots — the same scan `cfmleditor.findRefs` and the
// `refs` CLI do — and how that feels on a large workspace is the thing being
// tried out.
type References struct {
	Enabled bool `json:"enabled"`
}

// Features switches off individual LSP capabilities, for when one misbehaves on
// a real workspace and the alternative is downgrading the binary.
//
// Every flag defaults to *on*: these are opt-outs, so no existing setup
// changes by adding the block or by upgrading into it. They are pointers for
// the reason the completions block documents at length — a defaults-true flag
// stored as a plain bool cannot tell "the config turned this off" from "the
// config did not mention it", so naming any one key would silently switch off
// its siblings.
//
// Disabling one *un-advertises* its capability rather than merely refusing the
// request. That matters: the editor then falls back to its own behaviour — its
// word-based highlighting, its indentation folding — instead of offering a
// command the server declines and leaving the user with nothing.
//
// `linting` and `references` are the same kind of switch and predate this
// block, so they keep their own top-level keys; `references` additionally
// defaults *off*, since answering one request scans the whole workspace.
//
// `folding` defaults off as well, and is the one member of this block that
// does. It is new, and it is the most expensive thing here to answer: a
// script-syntax component's body reaches the CFML grammar as one opaque
// region, so folding it means parsing the whole body with the CFScript grammar
// on every request. That is a few milliseconds on a large component even after
// the walk around it was cut down, and an editor that never asked for it loses
// nothing it had — it falls back to folding by indentation, which is what it
// did before the feature existed. Turn it on with
// `{"features": {"folding": true}}`.
type Features struct {
	DocumentHighlight *bool `json:"documentHighlight"`
	Folding           *bool `json:"folding"`
	WatchedFiles      *bool `json:"watchedFiles"`
	// RangeFormatting is a second gate under formatting.enabled, not an
	// alternative to it. Range formatting is the one feature here that writes
	// to the buffer, and it shares every line of its machinery with
	// format-on-save — so without this, the only way to stop it is to switch
	// off formatting altogether and lose format-on-save with it.
	RangeFormatting *bool `json:"rangeFormatting"`
	// VariableDefinitions answers go-to-definition on a variable rather than a
	// component or a function. It is the newest of these and the one with the
	// most surface — nine scopes, two declaration sites the parser did not
	// record before, and a cross-file lookup for the shared scopes — so it is
	// the one most worth being able to switch off without giving up
	// go-to-definition entirely.
	VariableDefinitions *bool `json:"variableDefinitions"`
	// Routes answers go-to-definition and document links for framework routes.
	//
	// Switching it off stops the route scan, which is the most expensive thing
	// any of these do: it reads the document rather than an index, so its cost
	// is the size of the file and it is paid on every request that could be a
	// route. On a 64,000-line component that was three seconds per
	// go-to-definition before the scan was narrowed to the cursor, and document
	// links still pay the whole-file price because they are the whole file.
	Routes *bool `json:"routes"`
}

// ResolvedFeatures holds the feature switches with defaults applied.
type ResolvedFeatures struct {
	DocumentHighlight   bool
	Folding             bool
	WatchedFiles        bool
	RangeFormatting     bool
	VariableDefinitions bool
	Routes              bool
}

// foldingDefault is off. Named rather than inlined so the tests that assert the
// defaults read the same constant the resolver does, instead of restating it.
const foldingDefault = false

// ResolveFeatures applies the defaults for a `features` block: absent means
// every feature is on except folding, which is opt-in for the reason the type
// above gives.
//
// As with ResolveCompletions, the defaults live here so that every path to a
// Server agrees on them. A path that skipped this and took the zero value
// would come up with every feature switched off — which is not a subtle
// failure, but is a silent one, indistinguishable from the features not
// existing. That trap is unchanged by folding's default: the zero value is
// still wrong for the other three.
func ResolveFeatures(f *Features) ResolvedFeatures {
	if f == nil {
		f = &Features{}
	}

	return ResolvedFeatures{
		DocumentHighlight:   BoolDefault(f.DocumentHighlight, true),
		Folding:             BoolDefault(f.Folding, foldingDefault),
		WatchedFiles:        BoolDefault(f.WatchedFiles, true),
		RangeFormatting:     BoolDefault(f.RangeFormatting, true),
		VariableDefinitions: BoolDefault(f.VariableDefinitions, true),
		Routes:              BoolDefault(f.Routes, true),
	}
}

// Completions holds completion configuration.
//
// Pointers for the same reason the formatting flags are: all three default to
// true, so a plain bool cannot tell "the config turned this off" from "the
// config did not mention it". As plain bools, `{"completions": {"tagSnippets":
// false}}` also switched off global function resolution — the exact
// go-to-definition breakage this package was fixed for elsewhere.
type Completions struct {
	TagSnippets              *bool `json:"tagSnippets"`
	FunctionSnippets         *bool `json:"functionSnippets"`
	GlobalFunctionResolution *bool `json:"globalFunctionResolution"`
}

// ResolvedCompletions holds completion settings with defaults applied.
type ResolvedCompletions struct {
	TagSnippets              bool
	FunctionSnippets         bool
	GlobalFunctionResolution bool
}

// ResolveCompletions applies the defaults for a `completions` block: an absent
// block means all three are on, which is not what their zero value says.
//
// It exists so that the defaults are written down once. They used to live
// inside Resolve alone, and every path to a Server that did not run Resolve —
// a daemon session, or a standalone session with no config file and no editor
// settings — got the zero value instead, silently turning off global function
// resolution and both kinds of snippet.
func ResolveCompletions(c *Completions) ResolvedCompletions {
	if c == nil {
		c = &Completions{}
	}

	return ResolvedCompletions{
		TagSnippets:              BoolDefault(c.TagSnippets, true),
		FunctionSnippets:         BoolDefault(c.FunctionSnippets, true),
		GlobalFunctionResolution: BoolDefault(c.GlobalFunctionResolution, true),
	}
}

// Formatting holds formatter configuration.
type Formatting struct {
	// Enabled and Debug are pointers for the same reason every other flag here
	// is: Merge has to tell "the file turned this off" from "the file did not
	// mention it". As plain bools, a config file naming any formatting key at
	// all silently switched formatting off for a client that had enabled it
	// through initializationOptions.
	Enabled                *bool `json:"enabled"`
	Debug                  *bool `json:"debug"`
	SelfCloseTags          *bool `json:"selfCloseTags"`
	WhitespaceOnly         *bool `json:"whitespaceOnly"`
	QueryFormat            *bool `json:"queryFormat"`
	LowercaseTags          *bool `json:"lowercaseTags"`
	LowercaseAttributes    *bool `json:"lowercaseAttributes"`
	DoubleQuoteAttributes  *bool `json:"doubleQuoteAttributes"`
	QueryUppercaseKeywords *bool `json:"queryUppercaseKeywords"`
	// BlankLinesInBlocks pads a block's body with a blank line after the
	// opening brace and before the closing one. Unset is true, what the
	// formatter has always emitted.
	BlankLinesInBlocks *bool `json:"blankLinesInBlocks"`
	// SwitchCaseIndent indents `case` and `default` labels one level inside the
	// switch, level with the statements under them. Unset is false, which keeps
	// the label pulled back to the `switch` keyword's own column.
	SwitchCaseIndent *bool `json:"switchCaseIndent"`
	// ParenSpacing is the padding inside parentheses: "pad" for `( a )`,
	// "tight" for `(a)`. Unset keeps what the formatter has always emitted,
	// which is neither consistently — conditions and grouping are padded,
	// call and parameter lists are not — so a project that wants one rule
	// everywhere has to say which.
	ParenSpacing string `json:"parenSpacing"`
	// BraceStyle is where a block's opening brace goes: "same-line" (K&R,
	// `function f() {`) or "next-line" (Allman, the brace alone on the line
	// under the header). Unset is same-line, what the formatter has always
	// emitted.
	BraceStyle         string `json:"braceStyle"`
	ScopeCase          string `json:"scopeCase"`
	CommaPosition      string `json:"commaPosition"`
	QueryCommaPosition string `json:"queryCommaPosition"`
	LineWidth          *int   `json:"lineWidth"`
	// ParamBreakThreshold is the number of parameters above which a function
	// declaration's parameter list is expanded onto separate lines. Unset
	// breaks every list that has parameters, as the formatter always has.
	ParamBreakThreshold *int `json:"paramBreakThreshold"`
	AttrBreakThreshold  *int `json:"attrBreakThreshold"`
	IndentWidth         *int `json:"indentWidth"`
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
	ExpressionMappings       map[string]string
	ServicePropertyResolvers map[string]string
	Routes                   route.Config
	CodeMap                  CodeMap
	ComponentResolvers       []Resolver
	PropertyResolvers        []PropResolver
	BeanPaths                map[string]string
	Formatting               ResolvedFormatting
	Features                 ResolvedFeatures
	Linting                  bool
	LintMinSeverity          string
	References               bool
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
	BlankLinesInBlocks     bool
	SwitchCaseIndent       bool
	ParenSpacing           string
	BraceStyle             string
	ScopeCase              string
	CommaPosition          string
	QueryCommaPosition     string
	LineWidth              int
	ParamBreakThreshold    int
	AttrBreakThreshold     int
	IndentWidth            int
}

// Resolve takes a parsed JSON config and its directory, returning a fully resolved config.
func Resolve(cfg *JSON, dir string) *Resolved {
	r := &Resolved{}
	if len(cfg.Mappings) > 0 {
		r.Mappings = ResolvePaths(cfg.Mappings, dir)
	}

	if len(cfg.ExpressionMappings) > 0 {
		r.ExpressionMappings = cfg.ExpressionMappings
	}

	if len(cfg.ServicePropertyResolvers) > 0 {
		r.ServicePropertyResolvers = cfg.ServicePropertyResolvers
	}

	if cfg.Routes.Enabled() {
		r.Routes = cfg.Routes
	}

	if len(cfg.CodeMap.Entry)+len(cfg.CodeMap.Utility) > 0 || cfg.CodeMap.HideUtility {
		r.CodeMap = cfg.CodeMap
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
		r.Linting = BoolDefault(cfg.Linting.Enabled, false)
		r.LintMinSeverity = cfg.Linting.MinSeverity
	}

	if cfg.References != nil {
		r.References = cfg.References.Enabled
	}

	r.Features = ResolveFeatures(cfg.Features)

	comp := ResolveCompletions(cfg.Completions)
	r.TagSnippets = comp.TagSnippets
	r.FunctionSnippets = comp.FunctionSnippets
	r.GlobalFunctionResolution = comp.GlobalFunctionResolution

	if f := cfg.Formatting; f != nil {
		r.Formatting = ResolvedFormatting{
			Enabled:                BoolDefault(f.Enabled, false),
			Debug:                  BoolDefault(f.Debug, false),
			SelfCloseTags:          BoolDefault(f.SelfCloseTags, true),
			WhitespaceOnly:         BoolDefault(f.WhitespaceOnly, true),
			QueryFormat:            BoolDefault(f.QueryFormat, false),
			LowercaseTags:          BoolDefault(f.LowercaseTags, true),
			LowercaseAttributes:    BoolDefault(f.LowercaseAttributes, true),
			DoubleQuoteAttributes:  BoolDefault(f.DoubleQuoteAttributes, true),
			QueryUppercaseKeywords: BoolDefault(f.QueryUppercaseKeywords, true),
			BlankLinesInBlocks:     BoolDefault(f.BlankLinesInBlocks, true),
			SwitchCaseIndent:       BoolDefault(f.SwitchCaseIndent, false),
			ParenSpacing:           f.ParenSpacing,
			BraceStyle:             f.BraceStyle,
			ScopeCase:              f.ScopeCase,
			CommaPosition:          f.CommaPosition,
			QueryCommaPosition:     f.QueryCommaPosition,
			LineWidth:              IntDefault(f.LineWidth, 100),
			ParamBreakThreshold:    IntDefault(f.ParamBreakThreshold, 0),
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

	// A routes block is taken whole rather than merged key by key. The rule list
	// is ordered and the order is load-bearing — a rule that matches early stops
	// the ones below it — so interleaving two projects' lists would produce a
	// convention neither of them wrote.
	out.Routes = base.Routes
	if over.Routes.Enabled() {
		out.Routes = over.Routes
	}

	// Taken whole for the same reason: the globs are a description of one
	// codebase, and interleaving two projects' lists describes neither.
	out.CodeMap = base.CodeMap
	if len(over.CodeMap.Entry)+len(over.CodeMap.Utility) > 0 || over.CodeMap.HideUtility {
		out.CodeMap = over.CodeMap
	}

	out.BeanPaths = mergeStringMap(base.BeanPaths, over.BeanPaths)

	// Resolvers from both sides stay active. Order is priority — the first
	// match wins at lookup time — so over's entries lead.
	out.ComponentResolvers = append(append([]Resolver{}, over.ComponentResolvers...), base.ComponentResolvers...)
	out.PropertyResolvers = append(append([]PropResolver{}, over.PropertyResolvers...), base.PropertyResolvers...)

	out.Formatting = mergeFormatting(base.Formatting, over.Formatting)

	out.Linting = mergeLinting(base.Linting, over.Linting)

	out.Completions = mergeCompletions(base.Completions, over.Completions)

	if over.References != nil {
		out.References = over.References
	}

	out.Features = mergeFeatures(base.Features, over.Features)

	out.Debug = base.Debug || over.Debug

	return &out
}

// mergeFeatures unions two feature blocks key by key, with over winning
// wherever over states a value — the same rule mergeFormatting follows, and for
// the same reason: replacing the block wholesale would let one side naming a
// single switch discard every switch the other had set.
// mergeLinting unions two linting blocks key by key, with over's value winning
// wherever over states one.
//
// Replacing the whole block was correct while `enabled` was its only key and is
// not now: a child config naming only `minSeverity` would have carried a
// zero-valued `enabled` with it and switched linting off, which is why Enabled
// became a pointer.
func mergeLinting(base, over *Linting) *Linting {
	if base == nil {
		return over
	}

	if over == nil {
		return base
	}

	out := *base

	if over.Enabled != nil {
		out.Enabled = over.Enabled
	}

	if over.MinSeverity != "" {
		out.MinSeverity = over.MinSeverity
	}

	return &out
}

func mergeFeatures(base, over *Features) *Features {
	if base == nil {
		return over
	}

	if over == nil {
		return base
	}

	out := *base

	for _, f := range []struct{ dst, src **bool }{
		{&out.DocumentHighlight, &over.DocumentHighlight},
		{&out.Folding, &over.Folding},
		{&out.WatchedFiles, &over.WatchedFiles},
		{&out.RangeFormatting, &over.RangeFormatting},
		{&out.VariableDefinitions, &over.VariableDefinitions},
		{&out.Routes, &over.Routes},
	} {
		if *f.src != nil {
			*f.dst = *f.src
		}
	}

	return &out
}

// mergeFormatting unions two formatting blocks key by key, with over's value
// winning wherever over states one.
//
// It used to replace the whole block, which reads as "the file wins" but means
// something much stronger: a config file naming a single formatting key
// discarded every other formatting setting the editor had sent. That is how an
// IDE configures the formatter when it has no config file of its own to write
// — IntelliLucee sends all fourteen settings from its settings UI this way —
// so one `"formatting": {"lineWidth": 120}` in a project reverted the other
// thirteen to their defaults and switched the formatter off entirely.
func mergeFormatting(base, over *Formatting) *Formatting {
	if base == nil {
		return over
	}

	if over == nil {
		return base
	}

	out := *base

	for _, f := range []struct{ dst, src **bool }{
		{&out.Enabled, &over.Enabled},
		{&out.Debug, &over.Debug},
		{&out.SelfCloseTags, &over.SelfCloseTags},
		{&out.WhitespaceOnly, &over.WhitespaceOnly},
		{&out.QueryFormat, &over.QueryFormat},
		{&out.LowercaseTags, &over.LowercaseTags},
		{&out.LowercaseAttributes, &over.LowercaseAttributes},
		{&out.DoubleQuoteAttributes, &over.DoubleQuoteAttributes},
		{&out.QueryUppercaseKeywords, &over.QueryUppercaseKeywords},
		{&out.BlankLinesInBlocks, &over.BlankLinesInBlocks},
		{&out.SwitchCaseIndent, &over.SwitchCaseIndent},
	} {
		if *f.src != nil {
			*f.dst = *f.src
		}
	}

	for _, f := range []struct{ dst, src **int }{
		{&out.LineWidth, &over.LineWidth},
		{&out.ParamBreakThreshold, &over.ParamBreakThreshold},
		{&out.AttrBreakThreshold, &over.AttrBreakThreshold},
		{&out.IndentWidth, &over.IndentWidth},
	} {
		if *f.src != nil {
			*f.dst = *f.src
		}
	}

	for _, f := range []struct{ dst, src *string }{
		{&out.ParenSpacing, &over.ParenSpacing},
		{&out.BraceStyle, &over.BraceStyle},
		{&out.ScopeCase, &over.ScopeCase},
		{&out.CommaPosition, &over.CommaPosition},
		{&out.QueryCommaPosition, &over.QueryCommaPosition},
	} {
		if *f.src != "" {
			*f.dst = *f.src
		}
	}

	return &out
}

// mergeCompletions unions two completions blocks key by key, for the same
// reason mergeFormatting does: replacing the block wholesale would let a config
// file naming one completion setting silently reset the other two.
func mergeCompletions(base, over *Completions) *Completions {
	if base == nil {
		return over
	}

	if over == nil {
		return base
	}

	out := *base

	for _, f := range []struct{ dst, src **bool }{
		{&out.TagSnippets, &over.TagSnippets},
		{&out.FunctionSnippets, &over.FunctionSnippets},
		{&out.GlobalFunctionResolution, &over.GlobalFunctionResolution},
	} {
		if *f.src != nil {
			*f.dst = *f.src
		}
	}

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
	maps.Copy(out, base)

	maps.Copy(out, over)

	return out
}
