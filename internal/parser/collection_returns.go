package parser

import (
	"sort"
	"strings"
)

type collectionWrite struct {
	target, expression, function string
	offset                       int
	element, local, unknown      bool
}

type collectionNode struct {
	sources                     map[string]ReturnSource
	structure, invalid, written bool
	dependencies                []string
	aliases                     []string
}

// collectionTarget reads static paths with at most one final subscript. The
// key may be dynamic: inference describes all values, never a particular key.
func collectionTarget(sc *Scanner) (root string, element bool) {
	t := sc.NextSkipComments()
	if t.Kind != TokIdent {
		return "", false
	}

	var path chainBuilder
	path.reset(t.Value)

	for sc.PeekSkipComments().Kind == TokDot {
		sc.NextSkipComments()

		t = sc.NextSkipComments()
		if t.Kind != TokIdent {
			return "", false
		}

		path.writeDot()
		path.writeString(t.Value)
	}

	root = path.String()
	if sc.PeekSkipComments().Kind != TokLBracket {
		return root, false
	}

	sc.NextSkipComments()

	depth := 1
	for depth > 0 {
		switch sc.NextSkipComments().Kind {
		case TokEOF:
			return "", false
		case TokLBracket:
			depth++
		case TokRBracket:
			depth--
		default:
		}
	}

	return root, true
}

func collectionRead(expression string) (root string, element bool) {
	sc := NewScanner(expression)

	root, element = collectionTarget(sc)
	if sc.NextSkipComments().Kind != TokEOF {
		return "", false
	}

	return root, element
}

func collectionReturnExpressions(calls []pendingCall) map[string][]string {
	candidates := false

	for i := range calls {
		c := &calls[i]
		if c.returnExpr && strings.Contains(c.varName, "[") {
			if _, element := collectionRead(c.varName); element {
				candidates = true

				break
			}
		}
	}

	if !candidates {
		return nil
	}

	returns := map[string][]string{}

	for i := range calls {
		c := &calls[i]
		if c.returnExpr {
			returns[c.funcKey] = append(returns[c.funcKey], c.varName)
		}
	}

	return returns
}

func (pr *ParseResult) collectionKeys(writes []collectionWrite) (map[string]bool, map[int]string, func(string, string) string) {
	locals := map[string]bool{}

	scopeKeys := map[int]string{}
	for _, scope := range pr.Scopes {
		scopeKeys[scope.Start] = funcKey(scope.Start, scope.End)
	}

	for i := range pr.Funcs {
		f := &pr.Funcs[i]
		for _, a := range f.Arguments {
			locals[scopeKeys[int(f.Line)]+"\t"+strings.ToLower(a.Name)] = true
		}
	}

	for _, w := range writes {
		if w.local && w.function != "" {
			locals[w.function+"\t"+strings.TrimPrefix(strings.ToLower(w.target), "local.")] = true
		}
	}

	return locals, scopeKeys, func(root, function string) string {
		root = strings.ToLower(root)
		if strings.HasPrefix(root, "variables.") || strings.HasPrefix(root, "this.") {
			return root
		}

		if name, ok := strings.CutPrefix(root, "local."); ok {
			return function + "\t" + name
		}

		if name, ok := strings.CutPrefix(root, "arguments."); ok {
			return function + "\t" + name
		}

		base, _, _ := strings.Cut(root, ".")
		if locals[function+"\t"+base] {
			return function + "\t" + root
		}

		return "variables." + root
	}
}

func (pr *ParseResult) applyCollectionReturns(calls []pendingCall) {
	returns := collectionReturnExpressions(calls)
	if returns == nil {
		return
	}

	writes := pr.collectionWrites()
	locals, keys, key := pr.collectionKeys(writes)
	roots := map[string]bool{}

	for _, w := range writes {
		if w.element || emptyCollection(w.expression) {
			roots[key(w.target, w.function)] = true
		}
	}

	for function, expressions := range returns {
		for _, expr := range expressions {
			if root, element := collectionRead(expr); element {
				roots[key(root, function)] = true
			}
		}
	}

	nodes := map[string]*collectionNode{}
	node := func(k string) *collectionNode {
		if nodes[k] == nil {
			nodes[k] = &collectionNode{}
		}

		return nodes[k]
	}

	for i := range writes {
		pr.recordCollectionWrite(writes[i], roots, locals, key, node)
	}

	for _, w := range writes {
		if w.target == "" && w.unknown {
			for _, n := range nodes {
				n.invalid = true
			}

			break
		}
	}

	invalidateCollectionParents(writes, key, nodes)

	// A collection argument may contain caller-supplied values even when its
	// declaration has an empty-struct default.
	for i := range pr.Funcs {
		f := &pr.Funcs[i]
		for _, arg := range f.Arguments {
			if n := nodes[key("arguments."+arg.Name, keys[int(f.Line)])]; n != nil {
				n.invalid = true
			}
		}
	}

	settleCollections(nodes)

	for i := range pr.Funcs {
		f := &pr.Funcs[i]
		pr.applyCollectionFunction(f, returns[keys[int(f.Line)]], keys[int(f.Line)], key, nodes)
	}
}

// Walking each node's static ancestors avoids comparing every write to every
// node. Dynamic writes into a scope invalidate any field in that scope.
func invalidateCollectionParents(writes []collectionWrite, key func(string, string) string, nodes map[string]*collectionNode) {
	parents := map[string]bool{}

	for _, w := range writes {
		if !w.element {
			parents[key(w.target, w.function)] = true

			continue
		}

		switch {
		case strings.EqualFold(w.target, "variables"), strings.EqualFold(w.target, "this"):
			parents[strings.ToLower(w.target)] = true
		case strings.EqualFold(w.target, "local"), strings.EqualFold(w.target, "arguments"):
			parents[w.function+"\t"] = true
		}
	}

	for k, n := range nodes {
		if function, _, ok := strings.Cut(k, "\t"); ok && parents[function+"\t"] {
			n.invalid = true
		}

		for parent := k; ; {
			index := strings.LastIndexByte(parent, '.')
			if index < 0 {
				break
			}

			parent = parent[:index]
			if parents[parent] {
				n.invalid = true

				break
			}
		}
	}
}

func (pr *ParseResult) recordCollectionWrite(w collectionWrite, roots map[string]bool, locals map[string]bool, key func(string, string) string, node func(string) *collectionNode) {
	target := key(w.target, w.function)
	for prefix := target; ; {
		index := strings.LastIndexByte(prefix, '.')
		if index < 0 {
			break
		}

		prefix = prefix[:index]
		if roots[prefix] {
			target = prefix
			w.element = true

			break
		}
	}

	n := node(target)

	n.written = true
	if w.unknown {
		n.invalid = true

		return
	}

	if source, element := collectionRead(w.expression); source != "" {
		if element == w.element {
			dep := key(source, w.function)
			n.dependencies = append(n.dependencies, dep)
			node(dep)

			if !element {
				// Whole struct assignment shares storage. Element copies
				// only carry the source's value contract in one direction.
				n.aliases = append(n.aliases, dep)
				sourceNode := node(dep)
				sourceNode.dependencies = append(sourceNode.dependencies, target)
				sourceNode.aliases = append(sourceNode.aliases, target)
			}

			return
		}
	}

	if !w.element {
		if emptyCollection(w.expression) {
			n.structure = true
		} else {
			n.invalid = true
		}

		return
	}

	source := pr.collectionValueSource(w.expression, w.function, locals)
	if source.Component == "" || strings.HasPrefix(source.Component, "$") {
		n.invalid = true
	} else {
		n.addSource(source)
	}
}

func (pr *ParseResult) applyCollectionFunction(f *FunctionDef, expressions []string, function string, key func(string, string) string, nodes map[string]*collectionNode) {
	combined := &collectionNode{}

	candidate, invalid := false, false

	for _, expression := range expressions {
		root, element := collectionRead(expression)
		if !element {
			invalid = true

			continue
		}

		candidate = true

		n := nodes[key(root, function)]
		if n == nil || n.invalid {
			invalid = true

			continue
		}

		for _, source := range n.sources {
			combined.addSource(source)
		}
	}

	if !candidate {
		return
	}

	f.returnVar = ""
	f.ReturnComponent = ""
	f.ReturnSources = nil

	if invalid || combined.invalid {
		return
	}

	var buf foldScratch
	switch string(buf.lowerFold(f.ReturnType)) {
	case "", "any", "component", "object":
	default:
		return
	}

	sources := make([]ReturnSource, 0, len(combined.sources))
	for _, source := range combined.sources {
		sources = append(sources, source)
	}

	sort.Slice(sources, func(i, j int) bool {
		return sources[i].Component+"\t"+strings.Join(sources[i].Methods, "\t") < sources[j].Component+"\t"+strings.Join(sources[j].Methods, "\t")
	})
	f.ReturnSources = sources
	answer := ""

	for _, source := range sources {
		comp := source.Component
		if pr.FuncLookup != nil {
			comp = pr.walkChainRest(comp, source.Methods)
		} else {
			comp = dynamicIfTyped(comp, source.Methods)
		}

		if comp == "" || strings.HasPrefix(comp, "$") || answer != "" && !strings.EqualFold(answer, comp) {
			answer = ""

			break
		}

		answer = comp
	}

	f.ReturnComponent = pr.componentReturnFor(f, answer)
}

func emptyCollection(expression string) bool {
	sc := NewScanner(expression)

	t := sc.NextSkipComments()
	switch {
	case t.Kind == TokLBrace:
		if sc.NextSkipComments().Kind != TokRBrace {
			return false
		}
	case t.Kind == TokIdent && identEq(t.Value, "structNew"):
		if sc.NextSkipComments().Kind != TokLParen {
			return false
		}

		if sc.NextSkipComments().Kind != TokRParen {
			return false
		}
	default:
		return false
	}

	return sc.NextSkipComments().Kind == TokEOF
}

// Element-copy dependencies are directed; whole aliases share writes.
// A seed can ground a cycle, but an empty or
// uninitialized source cannot acquire a type from a collection that reads it.
func settleCollections(nodes map[string]*collectionNode) {
	reverse := map[string][]string{}

	for key, n := range nodes {
		if !n.written {
			n.invalid = true
		}

		for _, dep := range n.dependencies {
			reverse[dep] = append(reverse[dep], key)
		}
	}

	queue := make([]string, 0, len(nodes))
	for key := range nodes {
		queue = append(queue, key)
	}

	propagate := func() {
		for len(queue) > 0 {
			key := queue[0]
			queue = queue[1:]

			n := nodes[key]
			for _, target := range reverse[key] {
				dst := nodes[target]

				count, structure, invalid := len(dst.sources), dst.structure, dst.invalid
				if n.invalid {
					dst.invalid = true
				}

				for _, source := range n.sources {
					dst.addSource(source)
				}

				for _, alias := range dst.aliases {
					if alias == key && n.structure {
						dst.structure = true
					}
				}

				if len(dst.sources) != count || dst.structure != structure || dst.invalid != invalid {
					queue = append(queue, target)
				}
			}
		}
	}
	propagate()

	for key, n := range nodes {
		if len(n.sources) == 0 || !n.structure {
			n.invalid = true

			queue = append(queue, key)
		}
	}

	propagate()
}

func (pr *ParseResult) collectionWrites() []collectionWrite {
	var writes []collectionWrite

	lines := pr.contentLineIdx
	if lines == nil {
		lines = buildLineIdx(pr.Content)
	}

	functionAt := func(offset int) string {
		line := sort.Search(len(lines), func(i int) bool { return int(lines[i]) > offset }) - 1

		idx := sort.Search(len(pr.Scopes), func(i int) bool { return pr.Scopes[i].Start > line }) - 1
		if idx >= 0 && line <= pr.Scopes[idx].End {
			return funcKey(pr.Scopes[idx].Start, pr.Scopes[idx].End)
		}

		return ""
	}

	for _, r := range pr.Regions {
		if r.Kind == RegionScript {
			writes = collectCollectionExpression(r.Text, r.Offset, functionAt, writes)

			continue
		}

		if r.Kind != RegionTag {
			continue
		}

		writes = collectCollectionTags(r.Text, r.Offset, functionAt, writes)
	}

	return writes
}

func collectCollectionExpression(expression string, offset int, functionAt func(int) string, writes []collectionWrite) []collectionWrite {
	sc := NewScanner(expression)
	sc.interpStrings = true
	previous := TokEOF

	for {
		t := sc.NextSkipComments()
		before := previous

		previous = t.Kind
		if t.Kind == TokEOF {
			break
		}

		// Closure-local collection bindings need their own lexical identities.
		// Withhold collection inference rather than attributing them to the parent.
		if ((t.Kind == TokEquals || t.Kind == TokMinus) && sc.PeekSkipComments().Kind == TokGT) ||
			(t.Kind == TokIdent && identEq(t.Value, "function") && sc.PeekSkipComments().Kind == TokLParen) {
			writes = append(writes, collectionWrite{unknown: true})
		}

		if t.Kind != TokIdent || before == TokDot || before == TokDoubleColon {
			continue
		}

		cursor := *sc
		if collectionMutator(t.Value) && cursor.PeekSkipComments().Kind == TokLParen {
			cursor.NextSkipComments()

			root, element := collectionTarget(&cursor)
			if root != "" && !element {
				writes = append(writes, collectionWrite{offset: offset + t.Offset, target: root, function: functionAt(offset + t.Offset), unknown: true})
			}
		}

		cursor = *sc
		cursor.Restore(ScannerState{pos: t.Offset, line: t.Line})
		root, element := collectionTarget(&cursor)

		local := identEq(t.Value, "var")
		if local {
			root, element = collectionTarget(&cursor)
		}

		if root == "" {
			continue
		}

		if cursor.PeekSkipComments().Kind == TokLParen {
			if base, method, ok := strings.CutLast(root, "."); ok && collectionMemberMutator(method) {
				writes = append(writes, collectionWrite{offset: offset + t.Offset, target: base, function: functionAt(offset + t.Offset), unknown: true})
			}
		}

		operator := cursor.NextSkipComments()
		if operator.Kind == TokPlus || operator.Kind == TokMinus || operator.Kind == TokStar || operator.Kind == TokSlash || operator.Kind == TokAmpersand || operator.Kind == TokPercent || operator.Kind == TokCaret {
			next := cursor.PeekSkipComments().Kind
			if next == TokEquals || next == operator.Kind {
				writes = append(writes, collectionWrite{offset: offset + t.Offset, target: root, element: element, function: functionAt(offset + t.Offset), unknown: true})
			}

			continue
		}

		if operator.Kind != TokEquals || cursor.PeekSkipComments().Kind == TokEquals {
			continue
		}

		writes = append(writes, collectionWrite{offset: offset + t.Offset, target: root, expression: scriptReturnExpression(&cursor), function: functionAt(offset + t.Offset), element: element, local: local || hasPrefixFold(root, "local.")})
	}

	return writes
}

func collectCollectionTags(text string, offset int, functionAt func(int) string, writes []collectionWrite) []collectionWrite {
	for pos := 0; pos < len(text); {
		idx := nextTagStart(text[pos:])
		if idx < 0 {
			break
		}

		idx += pos
		if strings.HasPrefix(text[idx:], "<!---") {
			pos = skipCFMLComment(text, idx)

			continue
		}

		end := tagEndIndex(text[idx:])
		if end < 0 {
			break
		}

		end += idx
		tag := text[idx : end+1]
		pos = end + 1

		name := extractIdent(tag[1:])
		if strings.EqualFold(name, "cfset") {
			body := strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(tag[1+len(name):], ">"), "/"))
			writes = collectCollectionExpression(body, offset+idx+strings.Index(tag, body), functionAt, writes)
		}

		for _, attribute := range collectionOutputAttributes(name) {
			if root, element := collectionRead(getAttr(tag, attribute)); root != "" {
				writes = append(writes, collectionWrite{offset: offset + idx, target: root, element: element, function: functionAt(offset + idx), unknown: true})
			}
		}

		if strings.EqualFold(name, "cfparam") {
			name := getAttr(tag, "name")
			def := strings.Trim(getAttr(tag, "default"), "#")

			if name != "" {
				root, element := collectionRead(name)
				if root != "" {
					writes = append(writes, collectionWrite{offset: offset + idx, target: root, expression: def, element: element, unknown: element, function: functionAt(offset + idx)})
				}
			}
		}
	}

	return writes
}

// Only whole expressions can supply an element contract.
func (pr *ParseResult) collectionValueSource(expression, function string, locals map[string]bool) ReturnSource {
	if !wholeCollectionValue(expression) {
		return ReturnSource{}
	}

	if root, methods := factoryReturnChain(expression); root != "" && pr.resolverSet != nil {
		if comp := pr.resolverSet.Resolve(root); comp != "" {
			return ReturnSource{Component: comp, Methods: methods}
		}
	}

	tp := &tagParser{fileURI: string(pr.URI), inFunc: function, resolvers: pr.Resolvers, resolverSet: pr.resolverSet, funcs: pr.Funcs}
	tp.checkSetRHSStr(expression, "$element", 0)

	refs := tp.componentRefs
	if function != "" {
		refs = tp.funcRefs[function]
	}

	if len(refs) > 0 {
		return ReturnSource{Component: refs[0].Component, Methods: refs[0].ChainRest}
	}

	for i := range tp.pendingCalls {
		c := &tp.pendingCalls[i]
		if c.baseVar != "" {
			ref := firstRefIn(pr.funcRefsMap[function], c.baseVar, c.baseScope)
			if ref == nil && c.baseScope == RefAny && locals[function+"\t"+strings.ToLower(c.baseVar)] {
				return ReturnSource{}
			}

			if ref == nil {
				ref = firstRefIn(pr.ComponentRefs, c.baseVar, c.baseScope)
			}

			if ref != nil {
				return ReturnSource{Component: pr.settledComponent(ref), Methods: append([]string{c.funcName}, c.rest...)}
			}
		} else {
			for i := range pr.Funcs {
				f := &pr.Funcs[i]
				if strings.EqualFold(f.Name, c.funcName) {
					return ReturnSource{Component: pr.URI.Path(), Methods: append([]string{c.funcName}, c.rest...)}
				}
			}
		}
	}

	return ReturnSource{}
}

func (n *collectionNode) addSource(source ReturnSource) {
	if n.sources == nil {
		n.sources = map[string]ReturnSource{}
	}

	key := source.Component + "\t" + strings.Join(source.Methods, "\t")
	if len(n.sources) >= 16 {
		if _, exists := n.sources[key]; !exists {
			n.invalid = true

			return
		}
	}

	n.sources[key] = source
}

func wholeCollectionValue(expression string) bool {
	sc := NewScanner(expression)

	first := sc.PeekSkipComments()
	if first.Kind == TokIdent && identEq(first.Value, "new") {
		sc.NextSkipComments()
	}

	if path, _ := collectionTarget(sc); path == "" {
		return false
	}

	for {
		switch sc.NextSkipComments().Kind {
		case TokEOF:
			return true
		case TokLParen:
			depth := 1
			for depth > 0 {
				switch sc.NextSkipComments().Kind {
				case TokEOF:
					return false
				case TokLParen:
					depth++
				case TokRParen:
					depth--
				default:
				}
			}

			if sc.PeekSkipComments().Kind == TokDot {
				sc.NextSkipComments()

				if sc.NextSkipComments().Kind != TokIdent {
					return false
				}

				if sc.PeekSkipComments().Kind != TokLParen {
					return false
				}
			}
		default:
			return false
		}
	}
}

func collectionMutator(name string) bool {
	var buf foldScratch
	switch string(buf.lowerFold(name)) {
	case "structinsert", "structappend", "structupdate", "arrayappend", "arrayprepend", "arrayset", "arrayinsertat":
		return true
	default:
		return false
	}
}

func collectionMemberMutator(name string) bool {
	var buf foldScratch
	switch string(buf.lowerFold(name)) {
	case "insert", "append", "update", "prepend", "set", "insertat":
		return true
	default:
		return false
	}
}

// Output bindings assign non-element contracts. Their contents are not modeled.
func collectionOutputAttributes(name string) []string {
	var buf foldScratch
	switch string(buf.lowerFold(name)) {
	case "cfquery", "cfstoredproc":
		return []string{"name", "result"}
	case "cfobject", "cfsearch", "cfldap", "cfdbinfo", "cfprocresult", "cfpop", "cfimap", "cfspreadsheet", "cfdocument":
		return []string{"name"}
	case "cfsavecontent", "cfxml", "cfexecute", "cfprocparam":
		return []string{"variable"}
	case "cfwddx":
		return []string{"output"}
	case "cfinvoke":
		return []string{"returnvariable"}
	case "cfhttp":
		return []string{"result", "name"}
	case "cffile", "cfdirectory", "cfimage", "cfzip":
		return []string{"variable", "name", "result"}
	case "cfloop":
		return []string{"index", "item"}
	default:
		return nil
	}
}
