package resolve

import (
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/cfmleditor/clif/internal/parser"
)

// Wheels' Mapper copies public function references from wheels.mapper.* into
// variables and this. Recognize its cached-plan loader from source, rather than
// treating a directory name or an unused $integrateComponents helper as a mixin.
// The other Wheels loaders have different override rules and are not covered.
func (r *Resolver) wheelsMapperFunc(path, name string) (*parser.FunctionDef, bool) {
	if !strings.EqualFold(filepath.Base(path), "Mapper.cfc") {
		return nil, false
	}

	setup, ok := r.wheelsMapperSetup(path)
	if !ok {
		return nil, false
	}

	source, copyBody, global, folder := setup.source, setup.copyBody, setup.global, setup.folder

	entries, err := r.fs().ReadDir(folder)
	if err != nil || len(entries) > 128 {
		return nil, false
	}

	var found *parser.FunctionDef

	methods := maps.Clone(source.methods)
	seenMethods := make(map[string]bool)

	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".cfc") {
			continue
		}

		file := filepath.Join(folder, entry.Name())
		// The loader's metadata scan is nonrecursive and reads this component's
		// public functions. Do not import includes, inherited or private methods.
		fileMethods := r.wheelsSource(file).methods
		for key, method := range fileMethods {
			if !method.public {
				continue
			}

			if seenMethods[key] {
				methods[key] = wheelsMethod{}
			} else {
				methods[key] = method
			}

			seenMethods[key] = true
		}

		method, ok := fileMethods[strings.ToLower(name)]
		if !ok || !method.public {
			continue
		}

		for _, def := range r.EnsureIndexed(file) {
			if strings.EqualFold(def.Name, name) {
				// DirectoryList does not specify an order. An ambiguous override cannot
				// provide a stable definition or return contract.
				if found != nil {
					return nil, true
				}

				found = def
			}
		}
	}

	if found != nil {
		// CFML's struct return annotation is also used for these components.
		// A copied function returning this is bound to Mapper, not its stateless
		// source holder. Keep the index definition untouched for direct calls.
		if wheelsReturnsMapper(strings.ToLower(name), methods) {
			bound := *found
			bound.ReturnComponent = path
			bound.ReturnSources = nil

			return &bound, true
		}
	}

	if found == nil && !strings.EqualFold(name, "get") && !strings.EqualFold(name, "controller") {
		// Global is copied first, before mapper package methods. Only add its
		// otherwise missing public API; included UDFs require the Adobe fallback.
		if _, own := source.methods[strings.ToLower(name)]; !own {
			def := r.wheelsPlanFunc(global, name)
			if def != nil && (samePath(def.URI.Path(), global) || strings.Contains(copyBody, wheelsTokens(wheelsGlobalIncludeCopy))) {
				found = def
			}
		}
	}

	return found, found != nil
}

// The cached public-method producer is shared by Mapper and Controller.
func (r *Resolver) wheelsIntegrationPlan(global string) bool {
	plan := r.wheelsPlanFunc(global, "$componentIntegrationPlan")

	builder := r.wheelsPlanFunc(global, "$buildComponentIntegrationPlan")
	if plan == nil || builder == nil {
		return false
	}

	planBody := r.wheelsSource(plan.URI.Path()).methods["$componentintegrationplan"].body
	builderBody := r.wheelsSource(builder.URI.Path()).methods["$buildcomponentintegrationplan"].body
	// Both the producer and consumer must agree on publicMethods. Comments and
	// quoted descriptions cannot supply this evidence: token boundaries are kept.
	for _, evidence := range []string{
		`local.folderPath = ExpandPath("/#Replace(arguments.path, ".", "/", "all")#");`,
		`local.fileList = DirectoryList(local.folderPath, false, "name", "*.cfc");`,
		`local.instance = CreateObject("component", "#arguments.path#.#local.componentName#");`,
		`if (local.fns[local.f].access == "public") { local.ref = local.instance[local.fns[local.f].name];`,
		`ArrayAppend(local.publicMethods, {name = local.fns[local.f].name, ref = local.ref});`,
		`publicMethods = local.publicMethods`,
		`return local.rv;`,
	} {
		if !strings.Contains(builderBody, wheelsTokens(evidence)) {
			return false
		}
	}

	return strings.Contains(planBody, wheelsTokens(`$buildComponentIntegrationPlan(arguments.path)`))
}

// Global's known integration helpers are declared directly or in literal
// includes. Do not re-enter arbitrary inheritance/mixin lookup while checking
// the loader itself (Global can otherwise form a cycle back to Mapper).
func (r *Resolver) wheelsPlanFunc(global, name string) *parser.FunctionDef {
	var found *parser.FunctionDef

	_, own := r.wheelsSource(global).methods[strings.ToLower(name)]
	if own {
		for _, def := range r.EnsureIndexed(global) {
			if strings.EqualFold(def.Name, name) {
				found = def

				break
			}
		}
	} else {
		found = r.includedFunc(global, name)
	}

	if found == nil || !r.wheelsSource(found.URI.Path()).methods[strings.ToLower(name)].public {
		return nil
	}

	return found
}

type wheelsMethod struct {
	body             string
	public           bool
	noArgsReturn     string
	selfDefaultParam string
}

type wheelsSource struct {
	content   string
	methods   map[string]wheelsMethod
	producers *producerPlans

	// size and modTime are the file's stamp when content was read. An
	// unchanged stamp serves the cache without reading the file again: this
	// is asked for every untyped return lookup, on the keystroke path.
	size    int64
	modTime time.Time

	// gen is the index generation the stamp was last checked at, when the
	// index tracks the disk.
	gen uint64
}

// Cache lexical work, not definitions or directory listings. Re-read bytes
// when the file's size or modification time moves, so a changed loader/access
// modifier is not accepted by an old policy; EnsureIndexed supplies the
// current definition from the shared index.
func (r *Resolver) wheelsSource(path string) wheelsSource {
	if r.indexer != nil {
		return r.indexer.wheelsSource(path)
	}

	var gen uint64
	if r.IndexTracksFiles && r.Index != nil {
		gen = r.Index.Generation()
	}

	r.mu.RLock()
	cached, ok := r.wheelsSources[path]
	r.mu.RUnlock()

	// Nothing the index has seen has changed since the stamp was checked, and
	// the index sees every change: see IndexTracksFiles.
	if ok && r.IndexTracksFiles && r.Index != nil && cached.gen == gen {
		return cached
	}

	info, err := r.fs().Stat(path)
	if err != nil {
		return wheelsSource{}
	}

	if ok && cached.size == info.Size() && cached.modTime.Equal(info.ModTime()) {
		if r.IndexTracksFiles {
			cached.gen = gen
			r.storeWheelsSource(path, cached)
		}

		return cached
	}

	data, err := r.fs().ReadFile(path)
	if err != nil {
		return wheelsSource{}
	}

	content := string(data)

	result := cached
	if !ok || cached.content != content {
		result = wheelsSource{content: content, methods: wheelsMethods(content), producers: newProducerPlans(content)}
	}

	result.size, result.modTime, result.gen = info.Size(), info.ModTime(), gen
	r.storeWheelsSource(path, result)

	return result
}

func (r *Resolver) storeWheelsSource(path string, src wheelsSource) {
	r.mu.Lock()
	if r.wheelsSources == nil {
		r.wheelsSources = make(map[string]wheelsSource)
	}

	r.wheelsSources[path] = src
	r.mu.Unlock()
}

func wheelsMethods(content string) map[string]wheelsMethod {
	scanner := parser.NewScanner(content)
	methods := make(map[string]wheelsMethod)
	depth, componentDepth := 0, 0

	header := make([]parser.Token, 0, 8)

	for {
		tok := scanner.NextSkipComments()
		if tok.Kind == parser.TokEOF {
			return methods
		}

		if tok.Kind == parser.TokIdent && strings.EqualFold(tok.Value, "component") && depth == 0 {
			componentDepth = 1
		}

		if tok.Kind != parser.TokIdent || !strings.EqualFold(tok.Value, "function") || depth != componentDepth {
			switch tok.Kind {
			case parser.TokLBrace:
				depth++
				header = header[:0]
			case parser.TokRBrace:
				depth--
				header = header[:0]
			case parser.TokSemicolon:
				header = header[:0]
			default:
				header = append(header, tok)
			}

			continue
		}

		name, method := wheelsReadMethod(scanner, header)
		if name != "" {
			methods[name] = method
		}

		header = header[:0]
	}
}

func wheelsReadMethod(scanner *parser.Scanner, header []parser.Token) (string, wheelsMethod) {
	name := scanner.NextSkipComments()
	if name.Kind != parser.TokIdent || scanner.NextSkipComments().Kind != parser.TokLParen {
		return "", wheelsMethod{}
	}

	public := true

	for _, h := range header {
		if strings.EqualFold(h.Value, "private") || strings.EqualFold(h.Value, "package") || strings.EqualFold(h.Value, "remote") {
			public = false
		}
	}

	params := beanArguments(scanner)
	if params == nil {
		return "", wheelsMethod{}
	}

	for tok := scanner.NextSkipComments(); tok.Kind != parser.TokLBrace; tok = scanner.NextSkipComments() {
		if tok.Kind == parser.TokEOF || tok.Kind == parser.TokSemicolon {
			return "", wheelsMethod{}
		}

		if tok.Kind == parser.TokIdent && strings.EqualFold(tok.Value, "access") {
			if scanner.NextSkipComments().Kind != parser.TokEquals {
				return "", wheelsMethod{}
			}

			value := scanner.NextSkipComments()
			if value.Kind != parser.TokString || !strings.EqualFold(strings.Trim(value.Value, "\"'"), "public") {
				public = false
			}
		}
	}

	var (
		body   strings.Builder
		tokens []parser.Token
	)

	braces := 1
	for braces > 0 {
		tok := scanner.NextSkipComments()
		if tok.Kind == parser.TokEOF {
			return "", wheelsMethod{}
		}

		if tok.Kind == parser.TokLBrace {
			braces++
		}

		if tok.Kind == parser.TokRBrace {
			braces--
		}

		if braces > 0 {
			wheelsWriteToken(&body, tok)

			if len(tokens) <= 2048 {
				tokens = append(tokens, tok)
			}
		}
	}

	text := body.String()

	return strings.ToLower(name.Value), wheelsMethod{
		body: text, public: public, noArgsReturn: absentArgumentReturn(params, tokens),
		selfDefaultParam: defaultSelfReturnParameter(params, text),
	}
}

// defaultSelfReturnParameter recognizes a final literal-mode dispatch whose
// default parameter value selects `return this`. Work before the final dispatch
// may be too dynamic for a producer plan (beanORM.loadBy builds SQL with
// savecontent). The prefix must still exclude earlier returns and uses of the
// selected parameter or arguments scope that could change the final dispatch.
func defaultSelfReturnParameter(params [][]parser.Token, body string) string {
	for _, param := range params {
		eq := slices.IndexFunc(param, func(t parser.Token) bool { return t.Kind == parser.TokEquals })
		if eq < 1 || eq+2 != len(param) || param[eq-1].Kind != parser.TokIdent || param[eq+1].Kind != parser.TokString || !strings.EqualFold(strings.Trim(param[eq+1].Value, "\"'"), "self") {
			continue
		}

		name := strings.ToLower(param[eq-1].Value)
		condition := `if\t\(\targuments\t\.\t` + regexp.QuoteMeta(name) + `\t(?:eq|=\t=)\t"[^"#]*"\t\)\t\{\treturn\t[^;{}]+\t;\t\}`
		pattern := regexp.MustCompile(condition + `(?:\telse\t` + condition + `)*\telse\t\{\treturn\tthis\t;\t\}\t$`)

		loc := pattern.FindStringIndex(body)
		if loc == nil || !selfDispatchPrefixSafe(body[:loc[0]], name) {
			continue
		}

		match := body[loc[0]:loc[1]]
		if !strings.Contains(match, name+"\teq\t\"self\"") && !strings.Contains(match, name+"\t=\t=\t\"self\"") {
			return name
		}
	}

	return ""
}

// A suffix match alone says nothing about the code that precedes it. Keep the
// dispatch at method scope and reject returns and any parameter/scope use in
// the prefix, including passing the arguments scope to another function.
func selfDispatchPrefixSafe(prefix, name string) bool {
	tokens := producerTokens(prefix)
	if tokens == nil {
		return false
	}

	depth := 0

	for i, token := range tokens {
		if producerUnbracedControl(tokens, i) {
			return false
		}

		if token.Kind == parser.TokIdent && (strings.EqualFold(token.Value, "return") || strings.EqualFold(token.Value, "arguments") || strings.EqualFold(token.Value, name)) {
			return false
		}

		switch token.Kind {
		case parser.TokLBrace:
			depth++
		case parser.TokRBrace:
			depth--
		default:
		}

		if depth < 0 {
			return false
		}
	}

	return depth == 0
}

func wheelsTokens(source string) string {
	scanner := parser.NewScanner(source)

	var result strings.Builder

	for {
		tok := scanner.NextSkipComments()
		if tok.Kind == parser.TokEOF {
			return result.String()
		}

		wheelsWriteToken(&result, tok)
	}
}

func wheelsWriteToken(dst *strings.Builder, tok parser.Token) {
	value := tok.Value
	if tok.Kind == parser.TokString && len(value) >= 2 {
		quote := value[:1]
		value = `"` + strings.ReplaceAll(value[1:len(value)-1], quote+quote, quote) + `"`
	}

	dst.WriteString(strings.ToLower(value))
	dst.WriteByte('\t')
}

// Follow only self calls in return expressions, with bounded recursion. This
// covers scope()/resource() wrappers and $match's recursive member()/end()
// branch without assigning a Mapper type to an arbitrary struct return.
func wheelsReturnsMapper(name string, methods map[string]wheelsMethod) bool {
	anchored := false
	budget := 64

	var visit func(string, int, map[string]bool) bool

	visit = func(name string, depth int, active map[string]bool) bool {
		if active[name] {
			return true
		}

		if depth > 8 || budget <= 0 {
			return false
		}

		budget--

		body := methods[name].body
		if body == "" {
			return false
		}

		active[name] = true
		defer delete(active, name)

		scanner := parser.NewScanner(body)
		returns := 0

		for {
			tok := scanner.NextSkipComments()
			if tok.Kind == parser.TokEOF {
				return returns > 0
			}

			if tok.Kind != parser.TokIdent || !strings.EqualFold(tok.Value, "return") {
				continue
			}

			returns++

			first := scanner.NextSkipComments()
			if strings.EqualFold(first.Value, "this") && scanner.PeekSkipComments().Kind == parser.TokSemicolon {
				anchored = true

				continue
			}

			if strings.EqualFold(first.Value, "this") || strings.EqualFold(first.Value, "variables") {
				if scanner.NextSkipComments().Kind != parser.TokDot {
					return false
				}

				first = scanner.NextSkipComments()
			}

			for {
				if first.Kind != parser.TokIdent || scanner.NextSkipComments().Kind != parser.TokLParen {
					return false
				}

				if beanArguments(scanner) == nil || !visit(strings.ToLower(first.Value), depth+1, active) {
					return false
				}

				next := scanner.NextSkipComments()
				if next.Kind == parser.TokSemicolon {
					break
				}

				if next.Kind != parser.TokDot {
					return false
				}

				first = scanner.NextSkipComments()
			}
		}
	}

	return visit(name, 0, make(map[string]bool)) && anchored
}

const wheelsGlobalIncludeCopy = `if (StructKeyExists(arguments.componentInstance, "$frameworkGlobalFunctionNames")) {
 local.includeNames=arguments.componentInstance.$frameworkGlobalFunctionNames();
 local.includeCount=ArrayLen(local.includeNames);
 for(local.n=1;local.n<=local.includeCount;local.n++) {
  local.functionName=local.includeNames[local.n];
  if(!StructKeyExists(this,local.functionName) && (!ListFindNoCase(local.excludeList,local.functionName) || FindNoCase("wheels.mapper",local.componentName))) {
   variables[local.functionName]=arguments.componentInstance[local.functionName];
   this[local.functionName]=arguments.componentInstance[local.functionName];
  }
 }
}`

type wheelsMapperLoader struct {
	source                   wheelsSource
	copyBody, global, folder string
}

// wheelsMapperSetup reports whether path is Wheels' Mapper.cfc as pinned,
// with what its loader reads: its source, the copy body, wheels.Global and
// the mapper package folder.
func (r *Resolver) wheelsMapperSetup(path string) (wheelsMapperLoader, bool) {
	if !strings.EqualFold(filepath.Base(path), "Mapper.cfc") {
		return wheelsMapperLoader{}, false
	}

	source := r.wheelsSource(path)
	if source.methods["init"].body != wheelsTokens(`local.globalComponent = createObject("wheels.Global"); $integrateFunctions(local.globalComponent); $integrateComponents("wheels.mapper"); return this;`) ||
		source.methods["$integratecomponents"].body != wheelsTokens(`local.plan = $componentIntegrationPlan(arguments.path); local.iEnd = ArrayLen(local.plan); for (local.i = 1; local.i <= local.iEnd; local.i++) { $integrateFunctions(local.plan[local.i].instance, local.plan[local.i].publicMethods); }`) {
		return wheelsMapperLoader{}, false
	}

	copyBody := source.methods["$integratefunctions"].body
	if !strings.HasPrefix(copyBody, wheelsTokens(`if (ArrayLen(arguments.publicMethods)) { local.iEnd = ArrayLen(arguments.publicMethods); for (local.i = 1; local.i <= local.iEnd; local.i++) { local.m = arguments.publicMethods[local.i]; variables[local.m.name] = local.m.ref; this[local.m.name] = local.m.ref; } return; }`)) {
		return wheelsMapperLoader{}, false
	}
	// init first copies Global's methods; otherwise the integration-plan call
	// below has no implementation on Mapper. Require that fallback path too.
	for _, evidence := range []string{
		`local.meta = getMetaData(arguments.componentInstance);`,
		`local.excludeList = "get,controller";`,
		`if (local.method.access == "public" && (!listFindNoCase(local.excludeList, local.functionName) || findNoCase("wheels.mapper", local.componentName))) { variables[local.functionName] = componentInstance[local.functionName]; this[local.functionName] = componentInstance[local.functionName]; }`,
	} {
		if !strings.Contains(copyBody, wheelsTokens(evidence)) {
			return wheelsMapperLoader{}, false
		}
	}

	global := r.ComponentPath("wheels.Global", filepath.Dir(path))
	if global == "" {
		return wheelsMapperLoader{}, false
	}

	if !r.wheelsIntegrationPlan(global) {
		return wheelsMapperLoader{}, false
	}

	// ExpandPath in the loader uses the active wheels mapping. Resolve an actual
	// source component as an anchor, and reject a basename fallback elsewhere.
	anchor := r.ComponentPath("wheels.mapper.mapping", filepath.Dir(path))
	if anchor == "" || !strings.EqualFold(filepath.Base(filepath.Dir(anchor)), "mapper") || !samePath(filepath.Dir(filepath.Dir(anchor)), filepath.Dir(path)) {
		return wheelsMapperLoader{}, false
	}

	folder := filepath.Dir(anchor)

	return wheelsMapperLoader{source: source, copyBody: copyBody, global: global, folder: folder}, true
}
