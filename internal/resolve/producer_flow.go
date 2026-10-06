package resolve

import (
	"maps"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
)

// Values retain object identity separately from primitive, absent and unknown
// alternatives. A concrete receiver requires every surviving return to agree.
type producerValue struct {
	components                                []string
	primitive, unknown, absent, presenceKnown bool
	literal                                   string
	literalKnown                              bool
	fields                                    map[string]producerValue
}

func producerUnknown() producerValue { return producerValue{unknown: true} }
func producerMerge(a, b producerValue) producerValue {
	presenceKnown := a.presenceKnown && b.presenceKnown && a.absent == b.absent

	absent := a.absent && b.absent
	if a.unknown || b.unknown {
		return producerValue{unknown: true, presenceKnown: presenceKnown, absent: absent}
	}

	result := producerValue{literal: a.literal, literalKnown: a.literalKnown && b.literalKnown && a.literal == b.literal, primitive: a.primitive || b.primitive, components: slices.Clone(a.components), presenceKnown: presenceKnown, absent: absent}
	for _, comp := range b.components {
		if !slices.Contains(result.components, comp) {
			result.components = append(result.components, comp)
		}
	}

	if len(result.components) > 8 {
		return producerUnknown()
	}

	return result
}

func (v producerValue) component() string {
	if v.unknown || v.primitive || len(v.components) != 1 {
		return ""
	}

	return v.components[0]
}

type producerEnvironment map[string]producerValue

func mergeProducerEnvironments(a, b producerEnvironment) producerEnvironment {
	result := maps.Clone(a)
	for key, value := range b {
		before, ok := a[key]
		if !ok {
			before = producerUnknown()
		}

		result[key] = producerMerge(before, value)
	}

	for key := range a {
		if _, ok := b[key]; !ok {
			result[key] = producerUnknown()
		}
	}

	return result
}

type producerEvaluation struct {
	resolver          *Resolver
	fd                *parser.FunctionDef
	receiver, baseDir string
	depth             int
	budget            *int
	wanted            map[string]bool
	argumentsKnown    bool
	callerArguments   bool
}
type producerResult struct {
	environment producerEnvironment
	returns     []producerValue
	stopped     bool
	observed    producerEnvironment
}

func (r *Resolver) producerExpressionReturn(fd *parser.FunctionDef, expression, baseDir, component string) string {
	tokens := producerTokens(expression)
	open := -1

	for i, t := range tokens {
		if t.Kind == parser.TokLParen {
			open = i

			break
		}
	}

	if open < 0 || producerGroupEnd(tokens, open, parser.TokLParen, parser.TokRParen) != len(tokens)-1 {
		return ""
	}

	budget := 1024

	return r.producerExpressionValue(fd, expression, baseDir, component, 0, &budget).component()
}

func (r *Resolver) producerExpressionValue(fd *parser.FunctionDef, expression, baseDir, component string, depth int, budget *int) producerValue {
	tokens := producerTokens(expression)
	open := -1

	for i, t := range tokens {
		if t.Kind == parser.TokLParen {
			open = i

			break
		}
	}

	if open < 0 || producerGroupEnd(tokens, open, parser.TokLParen, parser.TokRParen) != len(tokens)-1 {
		return producerUnknown()
	}

	if r.producerIgnoresArguments(fd) {
		return producerUnknown()
	}

	eval := producerEvaluation{resolver: r, fd: fd, receiver: component, baseDir: baseDir, budget: budget, depth: depth, callerArguments: true}

	arguments, known := eval.arguments(tokens[open+1:len(tokens)-1], nil)
	if !known {
		return producerUnknown()
	}

	return eval.call(fd, component, arguments, true)
}

func (e *producerEvaluation) call(fd *parser.FunctionDef, receiver string, arguments map[string]producerValue, specific bool) producerValue {
	if fd == nil || e.depth > 8 || *e.budget <= 0 {
		return producerUnknown()
	}

	*e.budget--

	if specific && receiver != "" {
		source := e.resolver.wheelsSource(fd.URI.Path()).methods[strings.ToLower(fd.Name)]
		if source.selfDefaultParam != "" && e.selfModeSelected(fd, source.selfDefaultParam, arguments) {
			return e.concrete(receiver, e.baseDir)
		}
	}

	switch strings.ToLower(fd.ReturnType) {
	case "", "any", "object", "component":
	default:
		if cfmlTypes[strings.ToLower(fd.ReturnType)] {
			return producerValue{primitive: true}
		}

		if ret := e.resolver.ReturnComponentOf(fd); ret != "" {
			if self := e.resolver.selfTyped(receiver, e.baseDir, fd, ret); self != "" {
				ret = self
			}

			return e.concrete(ret, filepath.Dir(fd.URI.Path()))
		}

		return producerUnknown()
	}

	method := e.resolver.producerFor(fd)
	// Existing concrete/declared contracts take priority unless the source
	// contains an object guard, whose supplied-object path needs specialization.
	if !producerNeedsSpecialization(method, fd) {
		if fd.ReturnComponent != "" && fd.ReturnComponent != "$any" || len(fd.ReturnSources) > 0 || fd.DocReturn != "" {
			if ret := e.resolver.returnComponentOf(fd, e.depth+1, e.budget); ret != "" && ret != "$any" {
				if self := e.resolver.selfTyped(receiver, e.baseDir, fd, ret); self != "" {
					ret = self
				}

				return e.concrete(ret, filepath.Dir(fd.URI.Path()))
			}
		}
	}

	if method == nil || producerAugmentsReturn(method.body) {
		return producerUnknown()
	}

	local := *e
	local.fd = fd
	local.callerArguments = false
	local.argumentsKnown = specific
	local.wanted = method.wanted

	local.receiver = receiver
	if receiver == "" && producerReturnsThis(method.body) {
		local.receiver = fd.URI.Path()
	}

	local.depth++
	env := local.bindArguments(fd, method, arguments, specific)

	result := local.execute(method.body, env)
	if !result.stopped || len(result.returns) == 0 {
		return producerUnknown()
	}

	value := result.returns[0]
	for _, ret := range result.returns[1:] {
		value = producerMerge(value, ret)
	}

	return value
}

func (e *producerEvaluation) selfModeSelected(fd *parser.FunctionDef, name string, arguments map[string]producerValue) bool {
	value, supplied := arguments[name]
	if !supplied {
		for i := range fd.Arguments {
			if strings.EqualFold(fd.Arguments[i].Name, name) {
				value, supplied = arguments[producerPosition(i)]

				break
			}
		}
	}

	return !supplied || value.literalKnown && strings.EqualFold(value.literal, "self")
}

// producerFor is fd's source plan. The plan is read from the file while fd
// comes from the index, which follows an unsaved buffer in the editor; a
// plan whose parameters are not fd's describes another version of the
// function, and binding arguments by position through it would misplace them.
func (r *Resolver) producerFor(fd *parser.FunctionDef) *producerMethod {
	method := r.wheelsSource(fd.URI.Path()).producers.get(strings.ToLower(fd.Name))
	if method == nil || len(method.parameters) != len(fd.Arguments) {
		return nil
	}

	for i, name := range method.parameters {
		if !strings.EqualFold(name, fd.Arguments[i].Name) {
			return nil
		}
	}

	return method
}

func (e *producerEvaluation) concrete(component, baseDir string) producerValue {
	if component == "" || strings.HasPrefix(component, "$") || strings.Contains(component, "|") {
		return producerUnknown()
	}

	if path := e.resolver.ComponentPath(component, baseDir); path != "" {
		component = path
	}

	return producerValue{components: []string{component}}
}

// Interpret only lexical assignments and supported control flow; CFML is never
// executed. Unknown branches join values, and scope escapes discard evidence.
func (e *producerEvaluation) execute(nodes []producerNode, environment producerEnvironment) producerResult {
	result := producerResult{environment: maps.Clone(environment), observed: maps.Clone(environment)}

	for i := range nodes {
		node := &nodes[i]

		if *e.budget <= 0 {
			return producerResult{environment: environment, returns: []producerValue{producerUnknown()}, stopped: true}
		}

		*e.budget--
		env := result.environment

		switch node.kind {
		case "set":
			if !e.assign(node, env) {
				result.returns = append(result.returns, producerUnknown())
				result.stopped = true
			}
		case "return":
			result.returns = append(result.returns, e.expression(node.expression, env))
			result.stopped = true
		case "stop":
			result.stopped = true
		case "unsafe":
			result.returns = append(result.returns, producerUnknown())
			result.stopped = true
		case "query":
			if !e.query(node, env) {
				result.returns = append(result.returns, producerUnknown())
				result.stopped = true
			}

			child := e.execute(node.body, env)
			result.environment = child.environment
			result.returns = append(result.returns, child.returns...)
			result.stopped = result.stopped || child.stopped
		case "param":
			if _, ok := env[node.target]; !ok {
				env[node.target] = producerUnknown()
			}
		case "effect":
			e.effect(node.expression, env)
		case "block":
			child := e.execute(node.body, env)
			result.environment = child.environment
			result.returns = append(result.returns, child.returns...)
			result.stopped = child.stopped
		case "loop":
			child := e.loop(node, env)
			result.environment = child.environment
			result.returns = append(result.returns, child.returns...)
		case "if", "try":
			child := e.branches(node, env)
			result.environment = child.environment
			result.returns = append(result.returns, child.returns...)
			result.stopped = child.stopped

		default:
		}

		result.observed = mergeProducerEnvironments(result.observed, result.environment)
		if result.stopped {
			return result
		}
	}

	return result
}

func normalizeProducerPath(path string) string {
	return strings.TrimPrefix(strings.ToLower(path), "local.")
}

func (e *producerEvaluation) condition(expression string, env producerEnvironment) (yes, no producerEnvironment, choice int) {
	yes, no = maps.Clone(env), maps.Clone(env)

	tokens := producerTokens(expression)
	if producerConditionMutation(tokens) {
		for key := range yes {
			yes[key] = producerUnknown()
			no[key] = producerUnknown()
		}

		return yes, no, 0
	}

	negate := false
	if len(tokens) > 0 && (tokens[0].Kind == parser.TokBang || strings.EqualFold(tokens[0].Value, "not")) {
		negate = true
		tokens = tokens[1:]
	}

	if len(tokens) >= 4 && strings.EqualFold(tokens[0].Value, "len") && tokens[1].Kind == parser.TokLParen && tokens[len(tokens)-1].Kind == parser.TokRParen && e.resolver.ResolveFunc(e.methodOwner(), "len", e.baseDir) == nil {
		value := e.expression(producerText(tokens[2:len(tokens)-1]), env)
		if value.literalKnown {
			choice = -1
			if value.literal != "" {
				choice = 1
			}

			if negate {
				choice = -choice
			}

			return yes, no, choice
		}
	}

	if specialYes, specialNo, specialChoice, ok := e.specialCondition(tokens, env, negate); ok {
		return specialYes, specialNo, specialChoice
	}

	if len(tokens) < 4 || !strings.EqualFold(tokens[0].Value, "isObject") || tokens[1].Kind != parser.TokLParen || tokens[len(tokens)-1].Kind != parser.TokRParen || e.resolver.ResolveFunc(e.methodOwner(), "isObject", e.baseDir) != nil {
		e.effect(expression, yes)
		e.effect(expression, no)

		return yes, no, 0
	}

	path := producerPath(tokens[2 : len(tokens)-1])
	if path == "" {
		return yes, no, 0
	}

	value := e.read(path, env)
	choice = 0

	if !value.unknown {
		if value.primitive && len(value.components) == 0 {
			choice = -1
		}

		if !value.primitive && len(value.components) > 0 {
			choice = 1
		}
	}

	object, primitive := value, producerValue{primitive: true}
	object.primitive = false
	yes[normalizeProducerPath(path)] = object

	no[normalizeProducerPath(path)] = primitive
	if negate {
		return no, yes, -choice
	}

	return yes, no, choice
}

func (e *producerEvaluation) specialCondition(tokens []parser.Token, env producerEnvironment, negate bool) (yes, no producerEnvironment, choice int, ok bool) {
	yes, no = maps.Clone(env), maps.Clone(env)

	if path, matched := e.structKeyCondition(tokens); matched {
		value, known := env[path]
		if e.guardedField(path) {
			value = e.sharedFieldContract(path, false)
			known = !value.unknown
		} else if !known {
			value = producerUnknown()
		}

		choice = 0
		if !known && e.argumentsKnown && strings.HasPrefix(path, "arguments.") {
			choice = -1
		}

		if known && value.presenceKnown {
			if value.absent {
				choice = -1
			} else {
				choice = 1
			}
		}

		if known {
			present := value
			present.presenceKnown = true
			present.absent = false
			yes[path] = present
			no[path] = producerValue{unknown: true, absent: true, presenceKnown: true}
		}

		if negate {
			return no, yes, -choice, true
		}

		return yes, no, choice, true
	}

	if path, matched := e.unaryGuardCondition(tokens, "isSimpleValue"); matched {
		value := e.read(path, env)
		if strings.HasPrefix(path, "variables.") || strings.HasPrefix(path, "this.") {
			value = e.sharedFieldContract(path, true)
		}

		object := value
		object.primitive = false
		primitive := producerValue{primitive: true}
		yes[normalizeProducerPath(path)] = primitive
		no[normalizeProducerPath(path)] = object

		if negate {
			return no, yes, 0, true
		}

		return yes, no, 0, true
	}

	if path, matched := e.unaryGuardCondition(tokens, "isNull"); matched {
		value := e.read(path, env)
		if strings.HasPrefix(path, "variables.") || strings.HasPrefix(path, "this.") {
			value = e.sharedFieldContract(path, false)
		}

		missing := producerValue{unknown: true, absent: true, presenceKnown: true}
		value.presenceKnown = true
		value.absent = false
		yes[normalizeProducerPath(path)] = missing
		no[normalizeProducerPath(path)] = value

		if negate {
			return no, yes, 0, true
		}

		return yes, no, 0, true
	}

	return nil, nil, 0, false
}

func (e *producerEvaluation) unaryGuardCondition(tokens []parser.Token, name string) (string, bool) {
	if len(tokens) < 4 || !strings.EqualFold(tokens[0].Value, name) || tokens[1].Kind != parser.TokLParen || tokens[len(tokens)-1].Kind != parser.TokRParen || e.resolver.ResolveFunc(e.methodOwner(), name, e.baseDir) != nil {
		return "", false
	}

	path := producerPath(tokens[2 : len(tokens)-1])
	if path == "" {
		return "", false
	}

	return normalizeProducerPath(path), true
}

func (e *producerEvaluation) structKeyCondition(tokens []parser.Token) (string, bool) {
	if len(tokens) != 6 || !strings.EqualFold(tokens[0].Value, "structKeyExists") || tokens[1].Kind != parser.TokLParen || tokens[3].Kind != parser.TokComma || tokens[4].Kind != parser.TokString || tokens[5].Kind != parser.TokRParen || e.resolver.ResolveFunc(e.methodOwner(), "structKeyExists", e.baseDir) != nil {
		return "", false
	}

	scope := producerPath(tokens[2:3])

	name := strings.ToLower(strings.Trim(tokens[4].Value, "\"'"))
	if scope == "" || name == "" || strings.Contains(name, ".") {
		return "", false
	}

	return normalizeProducerPath(scope + "." + name), true
}

// sharedFieldContract returns the one component assigned to a guarded shared
// field anywhere in its component. Every explicit write must agree. Primitive
// writes are admitted only for isSimpleValue's sentinel pattern.
func (e *producerEvaluation) sharedFieldContract(path string, allowPrimitive bool) producerValue {
	path = normalizeProducerPath(path)
	if !e.guardedField(path) {
		return producerUnknown()
	}

	source := e.resolver.wheelsSource(e.fd.URI.Path())
	methods := source.producers.all()
	component := ""
	found := false
	valid := true
	request := strings.HasPrefix(path, "request.")
	writes := 0
	write := func(expression string) {
		value := e.expression(expression, producerEnvironment{})
		switch {
		case value.unknown, value.fields != nil, value.primitive && !allowPrimitive, len(value.components) > 1:
			valid = false
		case value.primitive:
			found = true
		case len(value.components) == 1:
			found = true

			if component == "" {
				component = value.components[0]
			} else if component != value.components[0] {
				valid = false
			}
		default:
			valid = false
		}
	}

	var visit func([]producerNode)

	visit = func(nodes []producerNode) {
		for i := range nodes {
			node := &nodes[i]
			// A request field's writes are counted in the text instead.
			if node.kind == "unsafe" && !request && producerMayTouchField(node.expression, path) {
				valid = false
			}

			if node.kind == "set" {
				target := normalizeProducerPath(node.target)
				if target == "variables" || target == "this" {
					valid = false
				} else if !strings.Contains(node.target, ".") {
					target = "variables." + target
				}

				if target == path {
					writes++

					write(node.expression)
				}
			}

			visit(node.body)
			visit(node.alternative)
		}
	}

	for _, method := range methods {
		visit(method.body)
	}

	// Function plans deliberately exclude component-body initialization. It is
	// still an explicit write to the field and must participate in the same
	// contract; otherwise a conflicting startup value would be ignored.
	startup := parser.Parse(e.fd.URI, source.content)
	for _, assignment := range startup.StartupVariableAssignments() {
		if normalizeProducerPath(assignment.Variable) != path {
			continue
		}

		if assignment.Expression == "" {
			valid = false
		} else {
			write(assignment.Expression)
		}
	}

	// Every write of a request field in the file's text must be one the
	// plan read: a statement it could not read may hold another.
	if request && len(requestWriteRe(strings.TrimPrefix(path, "request.")).FindAllStringIndex(source.content, -1)) != writes {
		valid = false
	}

	if !valid || !found || component == "" {
		return producerUnknown()
	}

	return e.concrete(component, e.baseDir)
}

// guardedField reports whether path is a field whose writes the component
// holds: its own variables or this scope, or a request-scope key no other
// file in a batch scan writes. Mura's getCurrentUser() creates
// request.currentUser when it is absent and nothing else ever sets it.
func (e *producerEvaluation) guardedField(path string) bool {
	if strings.HasPrefix(path, "variables.") || strings.HasPrefix(path, "this.") {
		return true
	}

	name, ok := strings.CutPrefix(path, "request.")

	return ok && name != "" && !strings.Contains(name, ".") && e.resolver.onlyFileWritesRequest(name, e.fd.URI.Path())
}

// Unsupported plans cannot supply an all-writes-agree proof. A scope or field
// reference may hide a write, an indexed assignment, or a scope escape. An
// exhausted scan likewise withholds the contract.
func producerMayTouchField(expression, path string) bool {
	tokens := producerTokens(expression)
	if len(tokens) == 0 {
		return true
	}

	scope, field, _ := strings.Cut(path, ".")
	for _, token := range tokens {
		if token.Kind == parser.TokIdent && (strings.EqualFold(token.Value, scope) || strings.EqualFold(token.Value, field)) {
			return true
		}
	}

	return false
}

func (e *producerEvaluation) read(path string, env producerEnvironment) producerValue {
	path = normalizeProducerPath(path)

	if e.callerArguments {
		return producerUnknown()
	}

	if value, ok := env[path]; ok {
		return value
	}

	for parent := path; strings.Contains(parent, "."); {
		parent, _, _ = strings.CutLast(parent, ".")
		if _, replaced := env[parent]; replaced {
			return producerUnknown()
		}
	}

	if path == "this" {
		return e.concrete(e.receiver, e.baseDir)
	}

	if path == "arguments" {
		if !e.argumentsKnown {
			return producerUnknown()
		}

		if _, escaped := env["@arguments"]; escaped {
			return producerUnknown()
		}

		fields := map[string]producerValue{}

		for key, v := range env {
			if strings.HasPrefix(key, "arguments.") && !v.presenceKnown {
				v = producerUnknown()
			}

			if name, ok := strings.CutPrefix(key, "arguments."); ok && !strings.Contains(name, ".") && !v.absent {
				fields[name] = v
			}
		}

		return producerValue{fields: fields}
	}

	if !strings.Contains(path, ".") {
		if value, ok := env["arguments."+path]; ok {
			return value
		}

		path = "variables." + path
		if value, ok := env[path]; ok {
			return value
		}
	}

	if e.resolver.Index == nil {
		return producerUnknown()
	}

	name := strings.TrimPrefix(strings.TrimPrefix(path, "variables."), "this.")
	if name == path {
		return producerUnknown()
	}

	if _, escaped := env["@"+strings.Split(path, ".")[0]]; escaped {
		return producerUnknown()
	}

	var answer producerValue

	found := false

	for _, ref := range e.resolver.Index.RefsForFile(e.fd.URI) {
		if !strings.EqualFold(ref.Variable, name) || ref.This != strings.HasPrefix(path, "this.") {
			continue
		}

		// The index's parse could not look the method up, and the receiver's
		// own type is no statement of what a call on it returns.
		if ref.BaseGuess {
			return producerUnknown()
		}

		value := e.concrete(ref.Component, filepath.Dir(e.fd.URI.Path()))
		if !found {
			answer = value
			found = true
		} else {
			answer = producerMerge(answer, value)
		}
	}

	if !found {
		return producerUnknown()
	}

	return answer
}

func (e *producerEvaluation) expression(expression string, env producerEnvironment) producerValue {
	tokens := producerUnwrap(producerTokens(expression))
	if len(tokens) == 0 {
		return producerUnknown()
	}

	if len(tokens) == 1 && (tokens[0].Kind == parser.TokString && !strings.Contains(tokens[0].Value, "#") || tokens[0].Kind == parser.TokNumber || strings.EqualFold(tokens[0].Value, "true") || strings.EqualFold(tokens[0].Value, "false")) {
		if tokens[0].Kind == parser.TokString {
			value := tokens[0].Value
			quote := value[:1]

			return producerValue{primitive: true, literalKnown: true, literal: strings.ReplaceAll(value[1:len(value)-1], quote+quote, quote)}
		}

		return producerValue{primitive: true}
	}

	if tokens[0].Kind == parser.TokLBrace && producerGroupEnd(tokens, 0, parser.TokLBrace, parser.TokRBrace) == len(tokens)-1 {
		fields := map[string]producerValue{}

		for _, entry := range producerSplit(tokens[1 : len(tokens)-1]) {
			if len(entry) == 0 {
				continue
			}

			if len(entry) < 3 || entry[1].Kind != parser.TokEquals && entry[1].Kind != parser.TokColon || entry[0].Kind != parser.TokIdent && entry[0].Kind != parser.TokString {
				return producerUnknown()
			}

			key := strings.ToLower(strings.Trim(entry[0].Value, "\"'"))
			if strings.Contains(key, "#") {
				return producerUnknown()
			}

			value := e.expression(producerText(entry[2:]), env)
			value.presenceKnown = true
			fields[key] = value
		}

		return producerValue{primitive: true, fields: fields}
	}

	if path := producerPath(tokens); path != "" {
		return e.read(path, env)
	}

	construct := strings.EqualFold(tokens[0].Value, "new")
	if construct {
		tokens = tokens[1:]
	}

	open := -1

	for i, t := range tokens {
		if t.Kind == parser.TokLParen {
			open = i

			break
		}
	}

	if open < 0 {
		return producerUnknown()
	}

	endRoot := producerGroupEnd(tokens, open, parser.TokLParen, parser.TokRParen)
	if endRoot < 0 {
		return producerUnknown()
	}

	before := producerPath(tokens[:open])
	if before == "" {
		return producerUnknown()
	}

	value := e.rootCall(tokens, open, endRoot, before, construct, env)

	position := endRoot + 1
	for position < len(tokens) {
		if value.component() == "" || position+2 >= len(tokens) || tokens[position].Kind != parser.TokDot || tokens[position+1].Kind != parser.TokIdent || tokens[position+2].Kind != parser.TokLParen {
			return producerUnknown()
		}

		method := tokens[position+1].Value

		end := producerGroupEnd(tokens, position+2, parser.TokLParen, parser.TokRParen)
		if end < 0 {
			return producerUnknown()
		}

		if !strings.EqualFold(method, "init") {
			args, known := e.arguments(tokens[position+3:end], env)
			if !known {
				return producerUnknown()
			}

			fd := e.resolver.ResolveFunc(value.component(), method, e.baseDir)
			value = e.call(fd, value.component(), args, true)
		}

		position = end + 1
	}

	return value
}

func (e *producerEvaluation) arguments(tokens []parser.Token, env producerEnvironment) (map[string]producerValue, bool) {
	arguments := map[string]producerValue{}

	pieces := producerSplit(tokens)
	for i, piece := range pieces {
		if len(piece) == 0 {
			if len(tokens) > 0 {
				return nil, false
			}

			continue
		}

		name := ""
		if len(piece) > 2 && piece[0].Kind == parser.TokIdent && (piece[1].Kind == parser.TokEquals || piece[1].Kind == parser.TokColon) {
			name = strings.ToLower(piece[0].Value)
			piece = piece[2:]
		}

		if name == "argumentcollection" {
			if len(pieces) != 1 {
				return nil, false
			}

			value := e.expression(producerText(piece), env)
			if value.fields == nil {
				return nil, false
			}

			for _, field := range value.fields {
				if !field.presenceKnown {
					return nil, false
				}
			}

			return maps.Clone(value.fields), true
		}
		// Positional names are assigned by the callee at dispatch time.
		if name == "" {
			name = producerPosition(i)
		}

		if _, duplicate := arguments[name]; duplicate {
			return nil, false
		}

		value := e.expression(producerText(piece), env)
		if value.fields != nil {
			value = producerUnknown()
		}

		value.presenceKnown = true
		arguments[name] = value
	}

	return arguments, true
}
func producerPosition(position int) string { return "@" + strconv.Itoa(position) }

func (e *producerEvaluation) effect(expression string, env producerEnvironment) {
	tokens := producerTokens(expression)
	if e.argumentMutation(tokens, env) {
		return
	}

	for i, tok := range tokens {
		// A closure writes the enclosing function's locals by reference, and an
		// included template runs in its scope; neither write is visible here.
		hidden := tok.Kind == parser.TokIdent && (strings.EqualFold(tok.Value, "function") || strings.EqualFold(tok.Value, "cfinclude") || i == 0 && strings.EqualFold(tok.Value, "include")) ||
			tok.Kind == parser.TokEquals && i+1 < len(tokens) && tokens[i+1].Kind == parser.TokGT
		if hidden || strings.EqualFold(tok.Value, "evaluate") {
			env["@variables"] = producerUnknown()
			for key := range env {
				env[key] = producerUnknown()
			}

			return
		}
	}

	for i, tok := range tokens {
		if tok.Kind != parser.TokIdent || i == 0 || tokens[i-1].Kind != parser.TokLParen && tokens[i-1].Kind != parser.TokComma && tokens[i-1].Kind != parser.TokEquals {
			continue
		}

		end := i + 1
		for end+1 < len(tokens) && tokens[end].Kind == parser.TokDot && tokens[end+1].Kind == parser.TokIdent {
			end += 2
		}

		if end > i+1 {
			path := producerPath(tokens[i:end])
			if e.read(path, env).fields != nil {
				env[normalizeProducerPath(path)] = producerUnknown()
			}

			continue
		}

		if !slices.Contains([]string{"arguments", "local", "variables", "this"}, strings.ToLower(tok.Value)) {
			if value, ok := env[normalizeProducerPath(tok.Value)]; ok && value.fields != nil {
				env[normalizeProducerPath(tok.Value)] = producerUnknown()
			}

			continue
		}

		if i+1 < len(tokens) && (tokens[i+1].Kind == parser.TokRParen || tokens[i+1].Kind == parser.TokComma) {
			scope := strings.ToLower(tok.Value)

			env["@"+scope] = producerUnknown()
			for key := range env {
				if strings.HasPrefix(key, scope+".") || scope == "local" && !strings.Contains(key, ".") {
					env[key] = producerUnknown()
				}
			}
		}
	}
}

func (e *producerEvaluation) methodOwner() string {
	if e.receiver != "" {
		return e.receiver
	}

	return e.fd.URI.Path()
}

func (e *producerEvaluation) loop(node *producerNode, environment producerEnvironment) producerResult {
	result := producerResult{environment: maps.Clone(environment)}
	for range 16 {
		input := maps.Clone(result.environment)
		if node.target != "" {
			input[normalizeProducerPath(node.target)] = producerUnknown()
		}

		child := e.execute(node.body, input)

		result.returns = append(result.returns, child.returns...)
		if child.stopped {
			return result
		}

		next := mergeProducerEnvironments(result.environment, child.environment)
		if producerEnvironmentsEqual(result.environment, next) {
			return result
		}

		result.environment = next
	}

	return producerResult{environment: environment, returns: []producerValue{producerUnknown()}, stopped: true}
}

func producerEnvironmentsEqual(a, b producerEnvironment) bool {
	if len(a) != len(b) {
		return false
	}

	for key, x := range a {
		y, ok := b[key]
		if !ok || x.unknown != y.unknown || x.primitive != y.primitive || x.absent != y.absent || x.presenceKnown != y.presenceKnown || x.literalKnown != y.literalKnown || x.literal != y.literal || !reflect.DeepEqual(x.fields, y.fields) || len(x.components) != len(y.components) {
			return false
		}

		for _, c := range x.components {
			if !slices.Contains(y.components, c) {
				return false
			}
		}
	}

	return true
}

func (e *producerEvaluation) argumentMutation(tokens []parser.Token, env producerEnvironment) bool {
	if len(tokens) < 5 || tokens[0].Kind != parser.TokIdent || tokens[1].Kind != parser.TokLParen || tokens[len(tokens)-1].Kind != parser.TokRParen {
		return false
	}

	name := strings.ToLower(tokens[0].Value)
	if name != "structappend" && name != "structdelete" {
		return false
	}

	pieces := producerSplit(tokens[2 : len(tokens)-1])
	if len(pieces) < 2 || producerPath(pieces[0]) != "arguments" || e.resolver.ResolveFunc(e.methodOwner(), name, e.baseDir) != nil {
		return false
	}

	if name == "structdelete" {
		key := e.expression(producerText(pieces[1]), env)
		if !key.literalKnown {
			return false
		}

		env["arguments."+strings.ToLower(key.literal)] = producerValue{unknown: true, absent: true, presenceKnown: true}

		return true
	}

	source := e.expression(producerText(pieces[1]), env)
	if source.fields == nil {
		return false
	}

	overwrite := true

	if len(pieces) > 2 {
		if len(pieces[2]) != 1 {
			return false
		}

		if strings.EqualFold(pieces[2][0].Value, "false") {
			overwrite = false
		} else if !strings.EqualFold(pieces[2][0].Value, "true") {
			return false
		}
	}

	for name, value := range source.fields {
		key := "arguments." + name

		before, exists := env[key]
		if !overwrite && exists && !before.absent {
			continue
		}

		value.presenceKnown = true
		env[key] = value
	}

	return true
}

// Query output names can replace the value we are about to return. Attribute
// bags must prove those names even when unrelated database options are unknown.
func (e *producerEvaluation) query(node *producerNode, env producerEnvironment) bool {
	fields := map[string]producerValue{}

	if node.expression != "" {
		attrs := e.expression(node.expression, env)
		if attrs.fields == nil {
			return false
		}

		fields = attrs.fields
	}

	explicit := map[string]string{"name": node.target, "result": node.alternative[0].target}
	for key, literal := range explicit {
		value, exists := fields[key]
		if literal != "" {
			if exists && (!value.literalKnown || value.literal != literal) {
				return false
			}

			value = producerValue{literalKnown: true, literal: literal}
			exists = true
		}

		if !exists {
			continue
		}

		if !value.literalKnown || producerPath(producerTokens(value.literal)) == "" {
			return false
		}

		path := normalizeProducerPath(strings.ToLower(value.literal))
		if !strings.Contains(path, ".") {
			if _, local := env[path]; !local {
				path = "variables." + path
			}
		}

		env[path] = producerValue{primitive: true}
	}

	return true
}

func (e *producerEvaluation) rootCall(tokens []parser.Token, open, end int, before string, construct bool, env producerEnvironment) producerValue {
	if construct {
		dir := filepath.Dir(e.fd.URI.Path())
		if e.callerArguments {
			dir = e.baseDir
		}

		return e.concrete(before, dir)
	}

	if strings.EqualFold(before, "createObject") && e.resolver.ResolveFunc(e.methodOwner(), "createObject", e.baseDir) == nil {
		parts := producerSplit(tokens[open+1 : end])
		if len(parts) == 2 {
			kind := e.expression(producerText(parts[0]), env)

			name := e.expression(producerText(parts[1]), env)
			if kind.literalKnown && strings.EqualFold(kind.literal, "component") && name.literalKnown {
				dir := filepath.Dir(e.fd.URI.Path())
				if e.callerArguments {
					dir = e.baseDir
				}

				return e.concrete(name.literal, dir)
			}
		}

		return producerUnknown()
	}

	component, noFollow, soft := e.resolver.matchResolver(producerCallText(tokens[:end+1]), nil, nil)
	if component != "" && !noFollow && !soft {
		return e.concrete(component, e.baseDir)
	}

	if e.callerArguments {
		return producerUnknown()
	}

	receiver, method, qualified := strings.CutLast(before, ".")

	target := e.receiver
	if target == "" {
		target = e.fd.URI.Path()
	}

	// this.f() and variables.f() call the component's own f(), as a bare
	// f() does: ColdBox's request() is `return this.execute( … )` and get()
	// `return variables.request( … )`.
	switch {
	case !qualified:
		method = before
	case !strings.EqualFold(receiver, "this") && !strings.EqualFold(receiver, "variables"):
		target = e.read(receiver, env).component()
	}

	if target == "" {
		return producerUnknown()
	}

	args, known := e.arguments(tokens[open+1:end], env)
	if !known {
		return producerUnknown()
	}

	return e.call(e.resolver.ResolveFunc(target, method, e.baseDir), target, args, true)
}

func (e *producerEvaluation) bindArguments(fd *parser.FunctionDef, method *producerMethod, arguments map[string]producerValue, specific bool) producerEnvironment {
	env := producerEnvironment{}

	for i, name := range method.parameters {
		value, ok := arguments[name]
		if !ok {
			value, ok = arguments[producerPosition(i)]
		}

		if !ok && specific {
			if def, exists := method.defaults[name]; exists {
				value = e.expression(def, env)
				ok = true
			}
		}

		if !ok {
			value = producerUnknown()

			if i < len(fd.Arguments) {
				arg := fd.Arguments[i]
				if arg.Type != "" && !cfmlTypes[strings.ToLower(arg.Type)] {
					value = e.concrete(arg.Type, filepath.Dir(fd.URI.Path()))
				}
			}
		}

		if ok || specific {
			value.presenceKnown = true
			value.absent = !ok
		}

		env["arguments."+name] = value
	}

	for name, value := range arguments {
		if !strings.HasPrefix(name, "@") {
			value.presenceKnown = true
			env["arguments."+name] = value
		}
	}

	return env
}

func (e *producerEvaluation) branches(node *producerNode, env producerEnvironment) producerResult {
	result := producerResult{environment: env}

	yes, no, choice := e.condition(node.expression, env)
	if node.kind != "if" {
		yes = maps.Clone(env)
		no = maps.Clone(env)
		choice = 0
	}

	a, b := producerResult{environment: env}, producerResult{environment: env}
	if choice >= 0 {
		a = e.execute(node.body, yes)
	}

	if node.kind == "try" {
		no = mergeProducerEnvironments(env, a.observed)
	}

	if choice <= 0 {
		b = e.execute(node.alternative, no)
	}

	result.returns = append(result.returns, a.returns...)

	result.returns = append(result.returns, b.returns...)
	switch {
	case choice > 0:
		result.environment = a.environment
		result.stopped = a.stopped
	case choice < 0:
		result.environment = b.environment
		result.stopped = b.stopped
	case a.stopped && b.stopped:
		result.stopped = true
	case a.stopped:
		result.environment = b.environment
	case b.stopped:
		result.environment = a.environment
	default:
		result.environment = mergeProducerEnvironments(a.environment, b.environment)
	}

	return result
}

func producerConditionMutation(tokens []parser.Token) bool {
	calls := []bool{}

	for i, t := range tokens {
		switch t.Kind {
		case parser.TokLParen:
			calls = append(calls, i > 0 && tokens[i-1].Kind == parser.TokIdent)
		case parser.TokRParen:
			if len(calls) > 0 {
				calls = calls[:len(calls)-1]
			}
		case parser.TokEquals:
			if len(calls) == 0 || !calls[len(calls)-1] || i < 2 || tokens[i-1].Kind != parser.TokIdent || (tokens[i-2].Kind != parser.TokLParen && tokens[i-2].Kind != parser.TokComma) {
				return true
			}
		case parser.TokPlus, parser.TokMinus:
			if i+1 < len(tokens) && tokens[i+1].Kind == t.Kind {
				return true
			}
		default:
		}
	}

	return false
}

func (e *producerEvaluation) assign(node *producerNode, env producerEnvironment) bool {
	sourcePath := producerPath(producerTokens(node.expression))
	if sourcePath == "arguments" || sourcePath == "variables" || sourcePath == "local" || sourcePath != "" && e.read(sourcePath, env).fields != nil {
		return false
	}

	target := normalizeProducerPath(node.target)
	if !strings.Contains(node.target, ".") {
		if _, local := env[target]; !local {
			if _, arg := env["arguments."+target]; arg {
				target = "arguments." + target
			} else {
				target = "variables." + target
			}
		}
	}

	e.effect(node.expression, env)

	for key := range env {
		if strings.HasPrefix(key, target+".") {
			env[key] = producerUnknown()
		}
	}

	for parent := target; strings.Contains(parent, "."); {
		parent, _, _ = strings.CutLast(parent, ".")
		if value, ok := env[parent]; ok && value.fields != nil {
			env[parent] = producerUnknown()
		}
	}

	if producerWantedTarget(node.target, e.wanted) {
		env[target] = e.expression(node.expression, env)
		if strings.HasPrefix(target, "arguments.") {
			value := env[target]
			value.presenceKnown = true
			env[target] = value
		}
	} else {
		env[target] = producerValue{unknown: true, presenceKnown: strings.HasPrefix(target, "arguments.")}
	}

	return true
}

// producerIgnoresArguments reports that call would answer unknown for fd
// whatever arguments it were given: a generic return type, no declared or
// inferred return, no source plan and no self-mode parameter. Evaluating the
// arguments first resolves every call inside them, and a parse asks this
// once per call site with its own argument text, so on a large component
// that evaluation was most of what a return lookup cost.
func (r *Resolver) producerIgnoresArguments(fd *parser.FunctionDef) bool {
	if fd == nil || !fd.URI.IsFile() {
		return false
	}

	switch strings.ToLower(fd.ReturnType) {
	case "", "any", "object", "component":
	default:
		return false
	}

	if fd.ReturnComponent != "" && fd.ReturnComponent != "$any" || len(fd.ReturnSources) > 0 || fd.DocReturn != "" {
		return false
	}

	if r.wheelsSource(fd.URI.Path()).methods[strings.ToLower(fd.Name)].selfDefaultParam != "" {
		return false
	}

	return r.producerFor(fd) == nil
}
