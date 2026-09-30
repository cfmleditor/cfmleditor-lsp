package resolve

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
)

type diEntry struct {
	target, component string
	singleton         bool
}
type diPolicy struct {
	roots                              []string
	entries                            map[string][]diEntry
	transients                         map[string]bool
	singulars                          map[string]string
	transientPattern, singletonPattern *regexp.Regexp
	exclude                            []string
	blocked, omitAliases, shallow      bool
	omitTyped, omitDefaulted           bool
}

// InjectionBeanLookup is deliberately separate from getBean identity lookup:
// DI/1 permits transient retrieval, but not transient setter/property injection.
// Unknown factory implementations keep the configured bean-root behavior.
func (r *Resolver) InjectionBeanLookup(file string) func(string) string {
	if r.discoveringDI || !cfpath.IsCFCFile(file) {
		return r.BeanLookup
	}

	r.diOnce.Do(r.buildDIPolicies)

	var policies []*diPolicy

	for i := range r.diPolicies {
		p := &r.diPolicies[i]
		if p.contains(file) {
			policies = append(policies, p)
		}
	}

	if len(policies) == 0 {
		return r.BeanLookup
	}

	return func(name string) string {
		comp := r.BeanLookup(name)
		result := ""

		for _, p := range policies {
			if !p.discovers(file) {
				return ""
			}

			candidate := p.injected(strings.ToLower(name), comp, r, map[string]bool{})
			if candidate == "" || result != "" && !cfpath.SamePath(result, candidate) {
				return ""
			}

			result = candidate
		}

		return result
	}
}

// InjectionPropertyLookup applies DI/1 property eligibility in addition to lifetime.
func (r *Resolver) InjectionPropertyLookup(file string) func(string, map[string]string) string {
	if r.discoveringDI || !cfpath.IsCFCFile(file) {
		return nil
	}

	lookup := r.InjectionBeanLookup(file)
	if !slices.ContainsFunc(r.diPolicies, func(p diPolicy) bool { return p.contains(file) }) {
		return nil
	}

	return func(name string, attrs map[string]string) string {
		for i := range r.diPolicies {
			p := &r.diPolicies[i]
			if !p.contains(file) {
				continue
			}

			_, defaulted := attrs["default"]
			if strings.EqualFold(attrs["setter"], "false") || p.omitDefaulted && defaulted || p.omitTyped && attrs["type"] != "" && !strings.EqualFold(attrs["type"], "any") {
				return ""
			}
		}

		return lookup(name)
	}
}

func (p *diPolicy) contains(file string) bool {
	for _, root := range p.roots {
		rel, err := filepath.Rel(root, file)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}

	return false
}

func (p *diPolicy) injected(name, comp string, r *Resolver, seen map[string]bool) string {
	if p.blocked || seen[name] || len(seen) >= maxStartupTemplates {
		return ""
	}

	if entries, found := p.entries[name]; found {
		seen[name] = true
		defer delete(seen, name)

		result := ""

		for _, entry := range entries {
			candidate := entry.component
			if entry.target != "" {
				candidate = p.injected(entry.target, r.BeanLookup(entry.target), r, seen)
			} else if !entry.singleton {
				return ""
			}

			if candidate == "" || result != "" && !cfpath.SamePath(result, candidate) {
				return ""
			}

			result = candidate
		}

		return result
	}

	if comp == "" || !p.discovers(comp) {
		return ""
	}

	base := strings.TrimSuffix(filepath.Base(comp), filepath.Ext(comp))
	if p.omitAliases && !strings.EqualFold(name, base) {
		return ""
	}

	dir := strings.ToLower(filepath.Base(filepath.Dir(comp)))

	singular := strings.TrimSuffix(dir, "s")
	if custom, ok := p.singulars[dir]; ok {
		singular = custom
	}

	if singular == "bean" || p.transients[dir] || p.transientPattern != nil && p.transientPattern.MatchString(base) || p.singletonPattern != nil && !p.singletonPattern.MatchString(base) {
		return ""
	}

	return comp
}

func (p *diPolicy) discovers(file string) bool {
	if !p.contains(file) {
		return false
	}

	for _, pattern := range p.exclude {
		if strings.Contains(strings.ToLower(filepath.ToSlash(file)), strings.ToLower(pattern)) {
			return false
		}
	}

	if p.shallow {
		for _, root := range p.roots {
			if cfpath.SamePath(filepath.Dir(file), root) {
				return true
			}
		}

		return false
	}

	return true
}

type diSource struct {
	file  string
	pr    *parser.ParseResult
	calls []beanCall
}

func (r *Resolver) buildDIPolicies() {
	d := &Resolver{FS: r.FS, Index: index.New(), WorkspaceFolders: r.WorkspaceFolders, Mappings: r.Mappings, Resolvers: r.Resolvers, ExpressionMappings: r.ExpressionMappings, discoveringDI: true}
	sources := r.diSources(d)
	factories := map[string][]int{}

	for _, source := range sources {
		for i := range source.calls {
			call := &source.calls[i]
			if call.method != "new" || !d.isDI1(call.component, filepath.Dir(source.file)) {
				continue
			}

			roots := d.diFolders(diArgument(call.args, "folders", 0), source.file)
			if len(roots) == 0 {
				continue
			}

			p := diConfig(diArgument(call.args, "config", 1))
			p.roots = roots
			p.entries["beanfactory"] = []diEntry{{component: d.ComponentPath(call.component, filepath.Dir(source.file)), singleton: true}}
			key := strings.ToLower(call.receiver)
			factories[key] = append(factories[key], len(r.diPolicies))
			r.diPolicies = append(r.diPolicies, p)
		}

		if p, ok := d.automaticDIPolicy(source); ok {
			r.diPolicies = append(r.diPolicies, p)
		}
	}

	r.diRegistrations(d, sources, factories)
}

func (r *Resolver) diSources(d *Resolver) []diSource {
	sources := []diSource{}

	queue := r.configuredStartupFiles()
	for _, root := range r.WorkspaceFolders {
		if app := r.FindApplicationRoot(root); app != "" {
			queue = append(queue, filepath.Join(app, "Application.cfc"))
		}
	}

	seen := map[string]bool{}
	for len(queue) > 0 && len(seen) < maxStartupTemplates {
		file := queue[0]
		queue = queue[1:]

		if seen[pathKey(file)] {
			continue
		}

		seen[pathKey(file)] = true

		data, err := r.fs().ReadFile(file)
		if err != nil {
			continue
		}

		content := string(data)
		pr := parser.ParseWithOptions(cfpath.ToURI(file), content, &parser.ParseOptions{Resolvers: r.Resolvers, FuncLookup: d.FuncLookup(filepath.Dir(file))})

		sources = append(sources, diSource{file, pr, beanCalls(content)})
		for _, raw := range parser.ExtractIncludes(content) {
			if target := r.IncludePath(raw, file); target != "" {
				queue = append(queue, target)
			}
		}
	}

	return sources
}

func (r *Resolver) automaticDIPolicy(source diSource) (diPolicy, bool) {
	if !strings.EqualFold(filepath.Base(source.file), "Application.cfc") || !strings.EqualFold(source.pr.Extends, "framework.one") {
		return diPolicy{}, false
	}

	cfg, ok := diFrameworkConfig(source.pr.Content)

	engine := diString(cfg["diengine"])
	if !ok || len(cfg["diengine"]) > 0 && engine != "di1" {
		return diPolicy{}, false
	}

	component := diString(cfg["dicomponent"])
	if len(cfg["dicomponent"]) > 0 && !strings.EqualFold(component, "framework.ioc") {
		return diPolicy{}, false
	}

	wired := parser.ParseWithOptions(source.pr.URI, source.pr.Content, &parser.ParseOptions{ExtractCalls: true, ScanAllScopes: true})

	calls := wired.AllCalls()
	for i := range calls {
		if strings.EqualFold(calls[i].FuncName, "setBeanFactory") {
			return diPolicy{}, false
		}
	}

	folders := cfg["dilocations"]
	if folders == nil {
		folders = []parser.Token{{Kind: parser.TokString, Value: `"model,controllers"`}}
	}

	roots := r.diFolders(folders, source.file)
	if len(roots) == 0 {
		return diPolicy{}, false
	}

	p := diConfig(cfg["diconfig"])
	p.roots = roots

	return p, true
}

func (r *Resolver) diRegistrations(d *Resolver, sources []diSource, factories map[string][]int) {
	for _, s := range sources {
		for i := range s.calls {
			call := &s.calls[i]

			ids := factories[strings.ToLower(call.receiver)]
			if len(ids) > 1 {
				for _, id := range ids {
					r.diPolicies[id].blocked = true
				}

				continue
			}

			if len(ids) == 0 || call.method == "new" {
				continue
			}

			name := beanArgument(call.args, "beanName", 0)
			entry := diEntry{}

			switch call.method {
			case "addalias":
				name = beanArgument(call.args, "aliasName", 0)
				entry.target = strings.ToLower(beanArgument(call.args, "beanName", 1))
			case "declarebean":
				entry.component = d.ComponentPath(beanArgument(call.args, "dottedPath", 1), filepath.Dir(s.file))
				lifetime := diArgument(call.args, "isSingleton", 2)
				entry.singleton = len(lifetime) == 0 || len(lifetime) == 1 && strings.EqualFold(lifetime[0].Value, "true")
			case "addbean":
				entry.singleton = true

				value := diArgument(call.args, "beanValue", 1)
				if len(value) > 0 {
					var expr strings.Builder
					for _, tok := range value {
						expr.WriteString(tok.Value)
					}

					if isCallChain(expr.String()) && !strings.ContainsAny(expr.String(), "()") || len(value) == 1 && value[0].Kind == parser.TokIdent {
						comp, _ := d.receiverComponent(expr.String(), call.line, "", "", s.pr, filepath.Dir(s.file), nil)
						entry.component = d.ComponentPath(comp, filepath.Dir(s.file))
					}
				}
			default:
				continue
			}

			if name != "" {
				for _, i := range ids {
					p := &r.diPolicies[i]
					p.entries[strings.ToLower(name)] = append(p.entries[strings.ToLower(name)], entry)
				}
			}
		}
	}
}

func (r *Resolver) isDI1(component, dir string) bool {
	seen := map[string]bool{}
	for len(seen) < 16 {
		file := r.ComponentPath(component, dir)
		if file == "" || seen[pathKey(file)] {
			return false
		}

		seen[pathKey(file)] = true

		data, err := r.fs().ReadFile(file)
		if err != nil {
			return false
		}

		pr := parser.Parse(cfpath.ToURI(file), string(data))

		methods := map[string]bool{}
		for i := range pr.Funcs {
			methods[strings.ToLower(pr.Funcs[i].Name)] = true
		}

		if methods["beanistransient"] && methods["findsetters"] && methods["issingleton"] && methods["getbean"] {
			return true
		}
		// An override of the lifetime/injection implementation makes its policy unknown.
		if methods["beanistransient"] || methods["findsetters"] || methods["issingleton"] {
			return false
		}

		if pr.Extends == "" {
			return false
		}

		component, dir = pr.Extends, filepath.Dir(file)
	}

	return false
}

func diArgument(args [][]parser.Token, name string, position int) []parser.Token {
	for i, arg := range args {
		if len(arg) >= 2 && arg[0].Kind == parser.TokIdent && (arg[1].Kind == parser.TokEquals || arg[1].Kind == parser.TokColon) {
			if strings.EqualFold(arg[0].Value, name) {
				return arg[2:]
			}
		} else if i == position {
			return arg
		}
	}

	return nil
}

func diString(tokens []parser.Token) string { return beanArgument([][]parser.Token{tokens}, "", 0) }

func (r *Resolver) diFolders(tokens []parser.Token, file string) []string {
	names := diStrings(tokens)
	roots := []string{}

	dir := r.FindApplicationRoot(filepath.Dir(file))
	if dir == "" {
		dir = filepath.Dir(file)
	}

	for _, name := range names {
		for folder := range strings.SplitSeq(name, ",") {
			folder = strings.TrimSpace(folder)
			if folder == "" {
				continue
			}

			candidates := []string{filepath.Join(dir, filepath.FromSlash(folder))}
			if filepath.IsAbs(folder) {
				candidates = append(candidates, folder)
			}

			prefix, rest, _ := strings.Cut(strings.TrimPrefix(folder, "/"), "/")
			for key, root := range r.EffectiveMappings(dir) {
				if strings.EqualFold(strings.Trim(key, "/"), prefix) {
					candidates = append(candidates, filepath.Join(root, filepath.FromSlash(rest)))
				}
			}

			for _, root := range r.WorkspaceFolders {
				candidates = append(candidates, filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(folder, "/"))))
			}

			for _, candidate := range candidates {
				if info, err := r.fs().Stat(candidate); err == nil && info.IsDir() {
					roots = append(roots, candidate)

					break
				}
			}
		}
	}

	return roots
}

func diStrings(tokens []parser.Token) []string {
	if value := diString(tokens); value != "" {
		return []string{value}
	}

	if len(tokens) < 2 || tokens[0].Kind != parser.TokLBracket || tokens[len(tokens)-1].Kind != parser.TokRBracket {
		return nil
	}

	values := []string{}

	for _, tok := range tokens[1 : len(tokens)-1] {
		if tok.Kind == parser.TokComma {
			continue
		}

		value := diString([]parser.Token{tok})
		if value == "" {
			return nil
		}

		values = append(values, value)
	}

	return values
}

func diStruct(tokens []parser.Token) (map[string][]parser.Token, bool) {
	out := map[string][]parser.Token{}
	if len(tokens) < 2 || tokens[0].Kind != parser.TokLBrace || tokens[len(tokens)-1].Kind != parser.TokRBrace {
		return out, false
	}

	tokens = tokens[1 : len(tokens)-1]
	for len(tokens) > 0 {
		if len(tokens) < 3 || (tokens[1].Kind != parser.TokColon && tokens[1].Kind != parser.TokEquals) {
			return out, false
		}

		key := strings.ToLower(tokens[0].Value)
		if tokens[0].Kind == parser.TokString {
			key = strings.ToLower(diString(tokens[:1]))
		} else if tokens[0].Kind != parser.TokIdent {
			return out, false
		}

		end, depth := 2, 0
		for end < len(tokens) {
			kind := tokens[end].Kind
			if kind == parser.TokComma && depth == 0 {
				break
			}

			depth += diDepth(kind)

			end++
		}

		if depth != 0 || end == 2 {
			return out, false
		}

		if _, found := out[key]; found {
			return out, false
		}

		out[key] = tokens[2:end]

		tokens = tokens[end:]
		if len(tokens) > 0 {
			tokens = tokens[1:]
		}
	}

	return out, true
}

func diConfig(tokens []parser.Token) diPolicy {
	p := diPolicy{entries: map[string][]diEntry{}, transients: map[string]bool{}, singulars: map[string]string{}, omitTyped: true, omitDefaulted: true}
	if len(tokens) == 0 {
		return p
	}

	cfg, ok := diStruct(tokens)
	if !ok {
		p.blocked = true

		return p
	}

	for key, value := range cfg {
		p.configure(key, value)
	}

	if p.transientPattern != nil && p.singletonPattern != nil {
		p.blocked = true
	}

	return p
}

// Only literal framework structs/direct assignments establish an automatic mode.
func diFrameworkConfig(content string) (map[string][]parser.Token, bool) {
	cfg := map[string][]parser.Token{}

	for _, region := range parser.ClassifyRegions(content) {
		if region.Kind != parser.RegionScript {
			continue
		}

		sc := parser.NewScanner(region.Text)
		previous := []parser.Token{}

		for {
			tok := sc.NextSkipComments()
			if tok.Kind == parser.TokEOF {
				break
			}

			if tok.Kind == parser.TokEquals {
				lhs := beanReceiver(append(slices.Clip(previous), parser.Token{Kind: parser.TokDot}))

				lhs = strings.TrimPrefix(strings.ToLower(lhs), "variables.")
				if lhs != "framework" && !strings.HasPrefix(lhs, "framework.") {
					previous = nil

					continue
				}

				value := diReadValue(sc)
				if lhs == "framework" {
					parsed, ok := diStruct(value)
					if !ok {
						return cfg, false
					}

					cfg = parsed
				} else {
					key := strings.TrimPrefix(lhs, "framework.")
					if _, found := cfg[key]; found {
						return cfg, false
					}

					cfg[key] = value
				}

				previous = nil

				continue
			}

			previous = append(previous, tok)
			if len(previous) > 16 {
				previous = previous[1:]
			}
		}
	}

	return cfg, true
}

func (p *diPolicy) configure(key string, value []parser.Token) {
	switch key {
	case "transientpattern", "singletonpattern":
		pattern := diString(value)
		if pattern == "" {
			p.blocked = true

			return
		}

		re, err := regexp.Compile("(?i)" + pattern)
		if err != nil {
			p.blocked = true

			return
		}

		if key == "transientpattern" {
			p.transientPattern = re
		} else {
			p.singletonPattern = re
		}
	case "transients":
		for _, folder := range diStrings(value) {
			p.transients[strings.ToLower(folder)] = true
		}

		if diStrings(value) == nil {
			p.blocked = true
		}
	case "exclude":
		p.exclude = diStrings(value)
		if p.exclude == nil {
			p.blocked = true
		}
	case "singulars":
		singulars, valid := diStruct(value)
		if !valid {
			p.blocked = true
		}

		for plural, singular := range singulars {
			if name := diString(singular); name != "" {
				p.singulars[plural] = strings.ToLower(name)
			} else {
				p.blocked = true
			}
		}
	case "constants":
		constants, valid := diStruct(value)
		if !valid {
			p.blocked = true
		}

		for name := range constants {
			p.entries[name] = []diEntry{{singleton: true}}
		}
	case "omitdirectoryaliases", "recurse", "liberal", "omittypedproperties", "omitdefaultedproperties":
		if len(value) != 1 || !strings.EqualFold(value[0].Value, "true") && !strings.EqualFold(value[0].Value, "false") {
			p.blocked = true

			return
		}

		flag := strings.EqualFold(value[0].Value, "true")

		switch key {
		case "omittypedproperties":
			p.omitTyped = flag
		case "omitdefaultedproperties":
			p.omitDefaulted = flag
		case "omitdirectoryaliases":
			p.omitAliases = flag
		case "recurse":
			p.shallow = !flag
		case "liberal":
			if flag {
				p.blocked = true
			}
		}
	}
}

func diReadValue(sc *parser.Scanner) []parser.Token {
	value := []parser.Token{}
	depth := 0

	for {
		tok := sc.NextSkipComments()
		if tok.Kind == parser.TokEOF || tok.Kind == parser.TokSemicolon && depth == 0 {
			return value
		}

		depth += diDepth(tok.Kind)
		if depth < 0 {
			return value
		}

		value = append(value, tok)
	}
}

func diDepth(kind parser.TokenKind) int {
	switch kind {
	case parser.TokLBrace, parser.TokLBracket, parser.TokLParen:
		return 1
	case parser.TokRBrace, parser.TokRBracket, parser.TokRParen:
		return -1
	default:
		return 0
	}
}
