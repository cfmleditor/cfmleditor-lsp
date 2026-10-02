package resolve

import (
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
)

type producerNode struct {
	kind               string
	target, expression string
	body, alternative  []producerNode
}

type producerMethod struct {
	parameters []string
	defaults   map[string]string
	body       []producerNode
	sensitive  bool
	wanted     map[string]bool
}

// These source plans carry no inferred types. Calls interpret them with their
// own arguments and application context; the byte-checked lexical cache owns
// the plans and refreshes them when a method changes.
func producerMethods(content string) map[string]*producerMethod {
	methods := map[string]*producerMethod{}
	tags := []producerTag{}

	for _, region := range parser.ClassifyRegions(content) {
		if region.Kind == parser.RegionScript {
			scriptProducerMethods(region.Text, methods)
			tags = append(tags, producerTag{name: "cfscript", body: region.Text})
		}

		if region.Kind == parser.RegionTag {
			tags = append(tags, producerTags(region.Text)...)
		}
	}

	tagProducerMethods(tags, methods)

	return methods
}

func producerTokens(source string) []parser.Token {
	scanner := parser.NewScanner(source)

	tokens := []parser.Token{}
	for len(tokens) <= 4096 {
		tok := scanner.NextSkipComments()
		if tok.Kind == parser.TokEOF {
			return tokens
		}

		tokens = append(tokens, tok)
	}

	return nil
}

func producerText(tokens []parser.Token) string {
	values := make([]string, len(tokens))
	for i, t := range tokens {
		values[i] = t.Value
	}

	return strings.Join(values, " ")
}

func scriptProducerMethods(source string, methods map[string]*producerMethod) {
	tokens := producerTokens(source)
	// Whole-file scanning finds function boundaries; individual method plans
	// remain bounded even in large classes.
	if tokens == nil {
		tokens = nil

		scanner := parser.NewScanner(source)
		for {
			tok := scanner.NextSkipComments()
			if tok.Kind == parser.TokEOF {
				break
			}

			tokens = append(tokens, tok)
		}
	}

	for i := 0; i+2 < len(tokens); i++ {
		if !strings.EqualFold(tokens[i].Value, "function") || tokens[i+1].Kind != parser.TokIdent || tokens[i+2].Kind != parser.TokLParen {
			continue
		}

		name := strings.ToLower(tokens[i+1].Value)

		end := producerGroupEnd(tokens, i+2, parser.TokLParen, parser.TokRParen)
		if end < 0 {
			return
		}

		params := producerSplit(tokens[i+3 : end])

		start := end + 1
		for start < len(tokens) && tokens[start].Kind != parser.TokLBrace && tokens[start].Kind != parser.TokSemicolon {
			start++
		}

		if start >= len(tokens) || tokens[start].Kind != parser.TokLBrace {
			continue
		}

		finish := producerGroupEnd(tokens, start, parser.TokLBrace, parser.TokRBrace)
		if finish < 0 {
			return
		}

		if finish-start > 4096 {
			i = finish

			continue
		}

		method := &producerMethod{defaults: map[string]string{}}

		for _, param := range params {
			if len(param) == 0 {
				continue
			}

			eq := -1

			for j, t := range param {
				if t.Kind == parser.TokEquals {
					eq = j

					break
				}
			}

			header := param
			if eq >= 0 {
				header = param[:eq]
			}

			if len(header) == 0 {
				continue
			}

			key := strings.ToLower(header[len(header)-1].Value)

			method.parameters = append(method.parameters, key)
			if eq >= 0 {
				method.defaults[key] = producerText(param[eq+1:])
			}
		}

		p := producerScript{tokens: tokens[start+1 : finish]}

		method.body = p.block()
		if p.valid() {
			method.sensitive = (producerSensitive(method.body) || producerCallBinding(method.body)) && !producerReturnsThis(method.body)
			method.wanted = producerWanted(method.body)
			methods[name] = method
		} else if producerSensitive([]producerNode{{kind: "if", expression: producerText(tokens[start+1 : finish])}}) {
			method.body = []producerNode{{kind: "unsafe", expression: producerText(tokens[start+1 : finish])}}
			method.sensitive = true
			methods[name] = method
		}

		i = finish
	}
}

func producerGroupEnd(tokens []parser.Token, start int, open, closing parser.TokenKind) int {
	depth := 0

	for i := start; i < len(tokens); i++ {
		if tokens[i].Kind == open {
			depth++
		}

		if tokens[i].Kind == closing {
			depth--
			if depth == 0 {
				return i
			}
		}
	}

	return -1
}

func producerSplit(tokens []parser.Token) [][]parser.Token {
	result := [][]parser.Token{}
	start, depth := 0, 0

	for i, t := range tokens {
		switch t.Kind {
		case parser.TokLParen, parser.TokLBrace, parser.TokLBracket:
			depth++
		case parser.TokRParen, parser.TokRBrace, parser.TokRBracket:
			depth--
		default:
		}

		if depth == 0 && t.Kind == parser.TokComma {
			result = append(result, tokens[start:i])
			start = i + 1
		}
	}

	return append(result, tokens[start:])
}

type producerScript struct {
	tokens   []parser.Token
	position int
	failed   bool
}

func (p *producerScript) valid() bool { return !p.failed && p.position == len(p.tokens) }

// takeParens consumes a parenthesised group and returns what it holds.
func (p *producerScript) takeParens() []parser.Token {
	if p.position >= len(p.tokens) || p.tokens[p.position].Kind != parser.TokLParen {
		p.failed = true

		return nil
	}

	end := producerGroupEnd(p.tokens, p.position, parser.TokLParen, parser.TokRParen)
	if end < 0 {
		p.failed = true
		p.position = len(p.tokens)

		return nil
	}

	group := p.tokens[p.position+1 : end]
	p.position = end + 1

	return group
}

func (p *producerScript) block() []producerNode {
	nodes := []producerNode{}
	for !p.failed && p.position < len(p.tokens) && p.tokens[p.position].Kind != parser.TokRBrace {
		nodes = append(nodes, p.statement())
	}

	return nodes
}

func (p *producerScript) statement() producerNode {
	tok := p.tokens[p.position]
	p.position++

	if tok.Kind == parser.TokSemicolon {
		return producerNode{kind: "noop"}
	}

	if tok.Kind == parser.TokLBrace {
		body := p.block()
		if p.position >= len(p.tokens) {
			p.failed = true

			return producerNode{}
		}

		p.position++

		return producerNode{kind: "block", body: body}
	}

	switch strings.ToLower(tok.Value) {
	case "if":
		condition := producerText(p.takeParens())
		if p.position >= len(p.tokens) {
			p.failed = true

			return producerNode{}
		}

		body := []producerNode{p.statement()}

		var alternative []producerNode

		if p.position < len(p.tokens) && strings.EqualFold(p.tokens[p.position].Value, "else") {
			p.position++
			if p.position >= len(p.tokens) {
				p.failed = true

				return producerNode{}
			}

			alternative = []producerNode{p.statement()}
		}

		return producerNode{kind: "if", expression: condition, body: body, alternative: alternative}
	case "for", "while":
		header := p.takeParens()
		target := ""

		if len(header) > 0 && strings.EqualFold(header[0].Value, "var") {
			header = header[1:]
		}

		if len(header) > 0 && header[0].Kind == parser.TokIdent {
			target = strings.ToLower(header[0].Value)
		}

		if p.position >= len(p.tokens) {
			p.failed = true

			return producerNode{}
		}

		body := []producerNode{p.statement()}

		if producerConditionMutation(header) {
			return producerNode{kind: "unsafe"}
		}

		return producerNode{kind: "loop", target: target, body: body}
	case "try":
		if p.position >= len(p.tokens) {
			p.failed = true

			return producerNode{}
		}

		body := []producerNode{p.statement()}
		if p.position >= len(p.tokens) || !strings.EqualFold(p.tokens[p.position].Value, "catch") {
			p.failed = true

			return producerNode{}
		}

		p.position++
		p.takeParens()

		if p.position >= len(p.tokens) {
			p.failed = true

			return producerNode{}
		}

		catches := [][]producerNode{{p.statement()}}

		// Every further catch clause is another handler the same failure can
		// reach. Read as one, the second was an expression statement that ran
		// on into the statement after it, so ColdBox's execute() lost its
		// `requestContext = getRequestContext();` and kept a stale value.
		for p.position < len(p.tokens) && strings.EqualFold(p.tokens[p.position].Value, "catch") {
			p.position++
			p.takeParens()

			if p.position >= len(p.tokens) {
				p.failed = true

				return producerNode{}
			}

			catches = append(catches, []producerNode{p.statement()})
		}

		if p.position < len(p.tokens) && strings.EqualFold(p.tokens[p.position].Value, "finally") {
			p.failed = true
		}

		return producerNode{kind: "try", body: body, alternative: producerHandlers(catches)}
	case "function", "switch", "do", "finally", "break", "continue":
		p.failed = true

		return producerNode{}
	}

	return p.expressionStatement()
}

// producerStatementBreak reports whether a line break between prev and next
// ends a statement. CFScript does not require the semicolon, and ColdBox's
// request() omits it throughout; read as one statement, its body lost its
// return. The break is a new line whose first token is a word that is not an
// operator, after a token that can end an expression. A line starting with
// `.`, an operator or an operator word continues the statement, so a chain
// written one call per line stays one.
func producerStatementBreak(prev, next parser.Token) bool {
	if next.Line <= prev.Line || next.Kind != parser.TokIdent || producerOperatorWord(next.Value) {
		return false
	}

	switch prev.Kind {
	case parser.TokString, parser.TokNumber, parser.TokRParen, parser.TokRBracket:
		return true
	case parser.TokIdent:
		// A word that wants what follows it does not end an expression:
		// `return` with its value on the next line, `var`, `new`, an operator.
		return !producerOperatorWord(prev.Value) && !producerLeadingKeyword(prev.Value)
	default:
		return false
	}
}

func producerOperatorWord(word string) bool {
	switch strings.ToLower(word) {
	case "and", "or", "not", "xor", "eqv", "imp", "eq", "neq", "is", "gt", "lt", "gte", "lte", "ge", "le", "contains", "mod":
		return true
	default:
		return false
	}
}

func producerLeadingKeyword(word string) bool {
	switch strings.ToLower(word) {
	case "return", "var", "new", "throw", "in", "case", "else":
		return true
	default:
		return false
	}
}

// producerAugmentsReturn reports whether the plan writes a member onto a
// variable it returns: `requestContext.getRenderedContent = …; return
// requestContext;`. The value returned then has members its component does not
// declare, and typed as the plain component every call to one would be
// reported missing. ColdBox's execute() adds three test helpers this way.
func producerAugmentsReturn(nodes []producerNode) bool {
	returned := map[string]bool{}

	var collect func([]producerNode)

	collect = func(nodes []producerNode) {
		for i := range nodes {
			if nodes[i].kind == "return" {
				if path := producerPath(producerUnwrap(producerTokens(nodes[i].expression))); path != "" {
					returned[normalizeProducerPath(path)] = true
				}
			}

			collect(nodes[i].body)
			collect(nodes[i].alternative)
		}
	}
	collect(nodes)

	var writes func([]producerNode) bool

	writes = func(nodes []producerNode) bool {
		for i := range nodes {
			if nodes[i].kind == "set" {
				if root, _, member := strings.Cut(normalizeProducerPath(nodes[i].target), "."); member && returned[root] {
					return true
				}
			}

			if writes(nodes[i].body) || writes(nodes[i].alternative) {
				return true
			}
		}

		return false
	}

	return writes(nodes)
}

// producerHandlers is a try's catch clauses as one alternative: each starts
// from the state the failure left, and which one runs is not in the source,
// so they are the branches of an if whose condition is unknown.
func producerHandlers(catches [][]producerNode) []producerNode {
	alternative := catches[len(catches)-1]
	for i := len(catches) - 2; i >= 0; i-- {
		alternative = []producerNode{{kind: "if", body: catches[i], alternative: alternative}}
	}

	return alternative
}

func (p *producerScript) expressionStatement() producerNode {
	start := p.position - 1
	tok := p.tokens[start]
	depth := 0

	for p.position < len(p.tokens) {
		t := p.tokens[p.position]
		if depth == 0 && (t.Kind == parser.TokSemicolon || t.Kind == parser.TokRBrace) {
			break
		}

		if depth == 0 && p.position > start+1 && producerStatementBreak(p.tokens[p.position-1], t) {
			break
		}

		switch t.Kind {
		case parser.TokLParen, parser.TokLBracket, parser.TokLBrace:
			depth++
		case parser.TokRParen, parser.TokRBracket, parser.TokRBrace:
			depth--
		default:
		}

		p.position++
	}

	expression := p.tokens[start:p.position]
	if p.position < len(p.tokens) && p.tokens[p.position].Kind == parser.TokSemicolon {
		p.position++
	}

	if strings.EqualFold(tok.Value, "return") {
		return producerNode{kind: "return", expression: producerText(expression[1:])}
	}

	if strings.EqualFold(tok.Value, "throw") || strings.EqualFold(tok.Value, "abort") {
		return producerNode{kind: "stop"}
	}

	return producerAssignment(expression)
}

func producerAssignment(tokens []parser.Token) producerNode {
	local := len(tokens) > 0 && strings.EqualFold(tokens[0].Value, "var")
	if local {
		tokens = tokens[1:]
	}

	for i, t := range tokens {
		if t.Kind == parser.TokLParen {
			break
		}

		if t.Kind == parser.TokEquals && i > 0 {
			if i+1 < len(tokens) && tokens[i+1].Kind == parser.TokEquals {
				break
			}

			target := producerPath(tokens[:i])
			if target != "" {
				if local {
					target = "local." + target
				}

				return producerNode{kind: "set", target: target, expression: producerText(tokens[i+1:])}
			}

			return producerNode{kind: "unsafe"}
		}
	}

	for i, t := range tokens {
		if t.Kind == parser.TokPlus || t.Kind == parser.TokMinus {
			if i+1 < len(tokens) && tokens[i+1].Kind == t.Kind {
				return producerNode{kind: "unsafe"}
			}
		}
	}

	return producerNode{kind: "effect", expression: producerText(tokens)}
}

func producerPath(tokens []parser.Token) string {
	if len(tokens) == 0 || len(tokens)%2 == 0 {
		return ""
	}

	for i, t := range tokens {
		if i%2 == 0 && t.Kind != parser.TokIdent || i%2 == 1 && t.Kind != parser.TokDot {
			return ""
		}
	}

	return strings.ToLower(strings.ReplaceAll(producerText(tokens), " ", ""))
}

func producerSensitive(nodes []producerNode) bool {
	for i := range nodes {
		n := &nodes[i]
		if n.kind == "if" || n.kind == "unsafe" {
			tokens := producerTokens(n.expression)
			for j, t := range tokens {
				if j+1 < len(tokens) && tokens[j+1].Kind == parser.TokLParen && (strings.EqualFold(t.Value, "isObject") || strings.EqualFold(t.Value, "structKeyExists") && j+3 < len(tokens) && strings.EqualFold(tokens[j+2].Value, "arguments") && tokens[j+3].Kind == parser.TokComma) {
					return true
				}
			}
		}

		if producerSensitive(n.body) || producerSensitive(n.alternative) {
			return true
		}
	}

	return false
}

type producerTag struct{ name, body string }

func producerTags(source string) []producerTag {
	tags := []producerTag{}

	for i := 0; i < len(source); {
		if strings.HasPrefix(source[i:], "<!---") {
			end := strings.Index(source[i+5:], "--->")
			if end < 0 {
				return nil
			}

			i += 5 + end + 4

			continue
		}

		if source[i] != '<' || (!strings.HasPrefix(strings.ToLower(source[i:min(i+4, len(source))]), "<cf") && !strings.HasPrefix(strings.ToLower(source[i:min(i+5, len(source))]), "</cf")) {
			i++

			continue
		}

		start := i
		i++
		quote := byte(0)

	tagEnd:
		for i < len(source) {
			ch := source[i]
			switch {
			case quote != 0:
				if ch == quote {
					if i+1 < len(source) && source[i+1] == quote {
						i += 2

						continue
					}

					quote = 0
				}
			case ch == '\'' || ch == '"':
				quote = ch
			case ch == '>':
				break tagEnd
			}

			i++
		}

		if i >= len(source) {
			return nil
		}

		text := strings.TrimSpace(source[start+1 : i])
		i++

		end := 0
		for end < len(text) && text[end] != ' ' && text[end] != '\t' && text[end] != '\r' && text[end] != '\n' {
			end++
		}

		name := strings.ToLower(text[:end])
		if !strings.HasPrefix(strings.TrimPrefix(name, "/"), "cf") {
			continue
		}

		tags = append(tags, producerTag{name: name, body: strings.TrimSpace(strings.TrimSuffix(text[end:], "/"))})
	}

	return tags
}

func producerAttr(body, key string) (string, bool) {
	tokens := producerTokens(body)
	for i := 0; i+2 < len(tokens); i++ {
		if !strings.EqualFold(tokens[i].Value, key) || tokens[i+1].Kind != parser.TokEquals {
			continue
		}

		t := tokens[i+2]
		if t.Kind == parser.TokString && len(t.Value) >= 2 {
			quote := t.Value[:1]

			return strings.ReplaceAll(t.Value[1:len(t.Value)-1], quote+quote, quote), true
		}

		if t.Kind == parser.TokIdent || t.Kind == parser.TokNumber {
			return t.Value, true
		}
	}

	return "", false
}

func tagProducerMethods(tags []producerTag, methods map[string]*producerMethod) {
	for i := 0; i < len(tags); i++ {
		if tags[i].name != "cffunction" {
			continue
		}

		name, ok := producerAttr(tags[i].body, "name")
		if !ok {
			continue
		}

		end := i + 1
		for end < len(tags) && tags[end].name != "/cffunction" {
			if tags[end].name == "cffunction" {
				break
			}

			end++
		}

		if end >= len(tags) || tags[end].name != "/cffunction" {
			continue
		}

		if end-i > 4096 {
			i = end

			continue
		}

		method := &producerMethod{defaults: map[string]string{}}

		for _, tag := range tags[i+1 : end] {
			if tag.name != "cfargument" {
				continue
			}

			arg, found := producerAttr(tag.body, "name")
			if !found {
				continue
			}

			arg = strings.ToLower(arg)
			method.parameters = append(method.parameters, arg)

			if def, found := producerAttr(tag.body, "default"); found {
				// A computed default is present even though its value is unknown.
				method.defaults[arg] = "'" + strings.ReplaceAll(def, "'", "''") + "'"
			}
		}

		p := producerTagParser{tags: tags[i+1 : end]}

		method.body = p.block()
		if !p.failed && p.position == len(p.tags) {
			method.sensitive = (producerSensitive(method.body) || producerCallBinding(method.body)) && !producerReturnsThis(method.body)
			method.wanted = producerWanted(method.body)
			methods[strings.ToLower(name)] = method
		} else {
			for _, tag := range tags[i+1 : end] {
				if (tag.name == "cfif" || tag.name == "cfscript") && producerSensitive([]producerNode{{kind: "if", expression: tag.body}}) {
					method.body = []producerNode{{kind: "unsafe", expression: tag.body}}
					method.sensitive = true
					methods[strings.ToLower(name)] = method

					break
				}
			}
		}

		i = end
	}
}

type producerTagParser struct {
	tags     []producerTag
	position int
	failed   bool
}

func (p *producerTagParser) block() []producerNode {
	nodes := []producerNode{}

	for !p.failed && p.position < len(p.tags) {
		tag := p.tags[p.position]
		if strings.HasPrefix(tag.name, "/") || tag.name == "cfelse" || tag.name == "cfelseif" || tag.name == "cfcatch" {
			break
		}

		p.position++

		switch tag.name {
		case "cfscript":
			tokens := producerTokens(tag.body)
			if tokens == nil {
				nodes = append(nodes, producerNode{kind: "unsafe", expression: tag.body})

				continue
			}

			script := producerScript{tokens: tokens}

			body := script.block()
			if !script.valid() {
				nodes = append(nodes, producerNode{kind: "unsafe", expression: tag.body})

				continue
			}

			nodes = append(nodes, producerNode{kind: "block", body: body})
		case "cfsetting":
			if strings.ContainsAny(tag.body, "#(") {
				nodes = append(nodes, producerNode{kind: "unsafe"})
			}
		case "cfargument", "cfqueryparam", "cfthrow", "cfabort", "cfexit":
			if tag.name == "cfthrow" || tag.name == "cfabort" || tag.name == "cfexit" {
				nodes = append(nodes, producerNode{kind: "stop"})
			}
		case "cfset":
			nodes = append(nodes, producerAssignment(producerTokens(tag.body)))
		case "cfreturn":
			nodes = append(nodes, producerNode{kind: "return", expression: tag.body})
		case "cfif":
			nodes = append(nodes, p.branch(tag.body))
		case "cfloop":
			body := p.block()
			p.close("/cfloop")

			target, _ := producerAttr(tag.body, "index")
			if target == "" {
				target, _ = producerAttr(tag.body, "item")
			}

			nodes = append(nodes, producerNode{kind: "loop", target: strings.ToLower(target), body: body})
		case "cftry":
			body := p.block()

			var alternate []producerNode

			if p.position < len(p.tags) && p.tags[p.position].name == "cfcatch" {
				p.position++
				alternate = p.block()
				p.close("/cfcatch")
			}

			p.close("/cftry")

			nodes = append(nodes, producerNode{kind: "try", body: body, alternative: alternate})
		case "cfquery":
			body := p.block()
			p.close("/cfquery")

			target, _ := producerAttr(tag.body, "name")
			result, _ := producerAttr(tag.body, "result")
			attrs, _ := producerAttr(tag.body, "attributeCollection")

			if strings.Contains(target+result, "#") {
				p.failed = true
			}

			nodes = append(nodes, producerNode{kind: "query", target: target, expression: strings.Trim(attrs, "#"), body: body, alternative: []producerNode{{target: result}}})
		case "cflock", "cftransaction", "cfsavecontent":
			body := p.block()
			p.close("/" + tag.name)

			target, _ := producerAttr(tag.body, "name")
			if tag.name == "cfsavecontent" {
				target, _ = producerAttr(tag.body, "variable")
			}

			if target != "" {
				nodes = append(nodes, producerNode{kind: "set", target: strings.ToLower(target), expression: "''"})
			}

			nodes = append(nodes, producerNode{kind: "block", body: body})
		case "cfparam":
			target, _ := producerAttr(tag.body, "name")
			nodes = append(nodes, producerNode{kind: "param", target: strings.ToLower(target)})
		default:
			p.failed = true
		}
	}

	return nodes
}

func (p *producerTagParser) close(name string) {
	if p.position >= len(p.tags) || p.tags[p.position].name != name {
		p.failed = true

		return
	}

	p.position++
}

func (p *producerTagParser) branch(condition string) producerNode {
	body := p.block()

	var alternate []producerNode

	if p.position < len(p.tags) {
		switch p.tags[p.position].name {
		case "cfelse":
			p.position++
			alternate = p.block()
			p.close("/cfif")
		case "cfelseif":
			next := p.tags[p.position].body
			p.position++
			alternate = []producerNode{p.branch(next)}
		default:
			p.close("/cfif")
		}
	} else {
		p.failed = true
	}

	return producerNode{kind: "if", expression: condition, body: body, alternative: alternate}
}

// producerUnwrap is tokens without the parentheses around the whole of them:
// `return( this );` is how jsonSerializer, and many a CFML style guide, writes
// `return this;`.
func producerUnwrap(tokens []parser.Token) []parser.Token {
	for len(tokens) > 2 && tokens[0].Kind == parser.TokLParen && producerGroupEnd(tokens, 0, parser.TokLParen, parser.TokRParen) == len(tokens)-1 {
		tokens = tokens[1 : len(tokens)-1]
	}

	return tokens
}

func producerReturnsThis(nodes []producerNode) bool {
	found := false

	for i := range nodes {
		node := &nodes[i]
		if node.kind == "return" {
			if producerPath(producerUnwrap(producerTokens(node.expression))) != "this" {
				return false
			}

			found = true
		}

		for _, body := range [][]producerNode{node.body, node.alternative} {
			if producerHasReturns(body) {
				if !producerReturnsThis(body) {
					return false
				}

				found = true
			}
		}
	}

	return found
}

func producerHasReturns(nodes []producerNode) bool {
	for i := range nodes {
		n := &nodes[i]
		if n.kind == "return" || producerHasReturns(n.body) || producerHasReturns(n.alternative) {
			return true
		}
	}

	return false
}

func producerCallText(tokens []parser.Token) string {
	var result strings.Builder

	for i, t := range tokens {
		if i > 0 && t.Kind != parser.TokDot && t.Kind != parser.TokLParen && tokens[i-1].Kind != parser.TokDot {
			result.WriteByte(' ')
		}

		result.WriteString(t.Value)
	}

	return result.String()
}

// A bounded backward slice avoids evaluating bindings that cannot affect the
// returned identity or a guard/output name. Missing dependencies stay unknown.
func producerWanted(nodes []producerNode) map[string]bool {
	wanted := map[string]bool{}

	var seed func([]producerNode)

	seed = func(nodes []producerNode) {
		for i := range nodes {
			node := &nodes[i]
			if node.kind == "query" || node.kind == "return" || node.kind == "if" {
				producerReadPaths(node.expression, wanted)
			}

			seed(node.body)
			seed(node.alternative)
		}
	}
	seed(nodes)

	var visit func([]producerNode)

	visit = func(nodes []producerNode) {
		for i := range nodes {
			node := &nodes[i]
			if node.kind == "set" && producerWantedTarget(node.target, wanted) {
				producerReadPaths(node.expression, wanted)
			}

			visit(node.body)
			visit(node.alternative)
		}
	}

	for range 16 {
		count := len(wanted)

		visit(nodes)

		if len(wanted) == count {
			break
		}
	}

	return wanted
}

func producerReadPaths(expression string, wanted map[string]bool) {
	tokens := producerTokens(expression)
	for i := 0; i < len(tokens); i++ {
		if tokens[i].Kind != parser.TokIdent {
			continue
		}

		start := i
		for i+2 < len(tokens) && tokens[i+1].Kind == parser.TokDot && tokens[i+2].Kind == parser.TokIdent {
			i += 2
		}

		path := producerPath(tokens[start : i+1])
		if path != "" {
			wanted[normalizeProducerPath(path)] = true
		}
	}
}

func producerWantedTarget(target string, wanted map[string]bool) bool {
	target = normalizeProducerPath(target)
	for path := range wanted {
		if path == target || strings.HasPrefix(path, target+".") || strings.HasPrefix(target, path+".") {
			return true
		}

		if strings.TrimPrefix(target, "arguments.") == path || strings.TrimPrefix(target, "variables.") == path {
			return true
		}
	}

	return false
}

// A call-derived scalar return must not inherit the parser's closed-file
// receiver guess. Collection contracts remain handled by their uniform writers.
func producerCallBinding(nodes []producerNode) bool {
	wanted := map[string]bool{}

	var seed func([]producerNode)

	seed = func(nodes []producerNode) {
		for i := range nodes {
			node := &nodes[i]
			if node.kind == "return" {
				path := producerPath(producerTokens(node.expression))
				if path != "" && path != "this" {
					wanted[normalizeProducerPath(path)] = true
				}
			}

			seed(node.body)
			seed(node.alternative)
		}
	}
	seed(nodes)

	var expand func([]producerNode)

	expand = func(nodes []producerNode) {
		for i := range nodes {
			node := &nodes[i]
			if node.kind == "set" && producerWantedTarget(node.target, wanted) {
				producerReadPaths(node.expression, wanted)
			}

			expand(node.body)
			expand(node.alternative)
		}
	}
	stable := false

	for range 16 {
		count := len(wanted)

		expand(nodes)

		if len(wanted) == count {
			stable = true

			break
		}
	}

	if !stable {
		return true
	}

	var visit func([]producerNode) bool

	visit = func(nodes []producerNode) bool {
		for i := range nodes {
			node := &nodes[i]
			if node.kind == "set" && producerWantedTarget(node.target, wanted) && strings.Contains(node.expression, "(") && !strings.HasPrefix(strings.ToLower(strings.TrimSpace(node.expression)), "new ") {
				return true
			}

			if visit(node.body) || visit(node.alternative) {
				return true
			}
		}

		return false
	}

	return visit(nodes)
}

// Scalar-call specialization replaces component guesses, not the established
// builtin or dynamic contracts. Object guards still need per-call proof.
func producerNeedsSpecialization(method *producerMethod, fd *parser.FunctionDef) bool {
	if method == nil || !method.sensitive {
		return false
	}

	return producerSensitive(method.body) || fd.ReturnComponent != "$any" && !strings.HasPrefix(fd.ReturnComponent, "$builtin.")
}
