package parser

import (
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/conv"
)

// MemberReceiverName preserves a record's path. A scoped variable and an
// unrelated record member with the same final name are different bindings.
func MemberReceiverName(variable string) (string, RefScope, bool) {
	scope, name, ok := strings.Cut(variable, ".")
	if !ok {
		return "", RefAny, false
	}

	switch {
	case strings.EqualFold(scope, "variables"):
		return name, RefVariables, strings.Contains(name, ".")
	case strings.EqualFold(scope, "this"):
		return name, RefThis, strings.Contains(name, ".")
	case strings.EqualFold(scope, "local"), strings.EqualFold(scope, "arguments"):
		return name, RefAny, strings.Contains(name, ".")
	case strings.EqualFold(scope, "application"), strings.EqualFold(scope, "request"), strings.EqualFold(scope, "session"), strings.EqualFold(scope, "server"):
		return variable, RefAny, strings.Contains(name, ".")
	default:
		return variable, RefAny, true
	}
}

// Explicit writes type one member, never its whole container. Conflicting or
// unknown writes and parent replacements withhold the member's component.
func (pr *ParseResult) applyMemberBindings() {
	if !pr.hasMemberBinding(pr.Content) {
		return
	}

	writes := pr.collectionWrites()
	locals, _, key := pr.collectionKeys(writes)
	nodes := map[string]*collectionNode{}
	identities := map[string]collectionWrite{}
	dependencies := map[string][]memberDependency{}

	for _, w := range writes {
		if w.element {
			continue
		}

		if _, _, ok := MemberReceiverName(w.target); !ok {
			continue
		}

		identity := key(w.target, w.function)

		n := nodes[identity]
		if n == nil {
			n = &collectionNode{}
			nodes[identity] = n
			identities[identity] = w
		}

		if receiver, methods := pr.memberWriteDependency(w); receiver != "" {
			dependencies[identity] = append(dependencies[identity], memberDependency{key(receiver, w.function), methods})

			continue
		}

		source := pr.memberValueSource(w.expression, w.function, locals)
		if w.unknown || source.Component == "" || strings.HasPrefix(source.Component, "$") {
			n.invalid = true

			continue
		}

		n.addSource(source)
	}

	descendants := memberDescendants(nodes)

	for _, w := range writes {
		if w.target == "" && w.unknown {
			for _, n := range nodes {
				n.invalid = true
			}

			break
		}

		if !w.element && emptyCollection(w.expression) {
			continue
		}

		if source, indexed := collectionRead(w.expression); source != "" && !indexed {
			invalidate(descendants[key(source, w.function)])
		}

		parent := key(w.target, w.function)
		if w.element && (strings.EqualFold(w.target, "variables") || strings.EqualFold(w.target, "this")) {
			parent = strings.ToLower(w.target)
		}

		invalidate(descendants[parent])
	}

	components := pr.settleMemberDependencies(nodes, dependencies)

	for identity := range nodes {
		w := identities[identity]
		name, scope, _ := MemberReceiverName(w.target)
		component := components[identity]

		ref := ComponentRef{Variable: strings.Clone(name), Component: component, URI: pr.URI, Line: conv.Uint32(pr.lineAt(w.offset)), This: scope == RefThis}
		if strings.Contains(identity, "\t") {
			if pr.funcRefsMap == nil {
				pr.funcRefsMap = map[string][]ComponentRef{}
			}

			pr.funcRefsMap[w.function] = append(pr.funcRefsMap[w.function], ref)
		} else {
			pr.ComponentRefs = append(pr.ComponentRefs, ref)
		}
	}
}

// memberDescendants files each node under every proper prefix of its identity
// that ends before a dot, so the members a write replaces are one lookup away.
// Every write used to test every node's identity against its own key, which
// made a component holding many members quadratic in them.
func memberDescendants(nodes map[string]*collectionNode) map[string][]*collectionNode {
	descendants := map[string][]*collectionNode{}

	for identity, n := range nodes {
		for i := range len(identity) {
			if identity[i] == '.' {
				descendants[identity[:i]] = append(descendants[identity[:i]], n)
			}
		}
	}

	return descendants
}

func invalidate(nodes []*collectionNode) {
	for _, n := range nodes {
		n.invalid = true
	}
}

func (pr *ParseResult) memberSourceComponent(node *collectionNode) string {
	if node.invalid {
		return ""
	}

	answer := ""

	for _, source := range node.sources {
		component := source.Component
		if pr.FuncLookup != nil {
			component = pr.walkChainRest(component, source.Methods)
		} else {
			component = dynamicIfTyped(component, source.Methods)
		}

		if component == "" || strings.HasPrefix(component, "$") || answer != "" && !strings.EqualFold(answer, component) {
			return ""
		}

		answer = component
	}

	return answer
}

func (pr *ParseResult) hasMemberBinding(content string) bool {
	for offset := strings.IndexByte(content, '='); offset >= 0; {
		if pr.memberAssignmentCandidate(content, offset) {
			return true
		}

		next := strings.IndexByte(content[offset+1:], '=')
		if next < 0 {
			return false
		}

		offset += next + 1
	}

	return false
}

func (pr *ParseResult) memberAssignmentCandidate(content string, offset int) bool {
	if offset == 0 || content[offset-1] == '=' || offset+1 < len(content) && content[offset+1] == '=' {
		return false
	}

	end := offset
	for end > 0 && (content[end-1] == ' ' || content[end-1] == '\t') {
		end--
	}

	start := end
	for start > 0 && (isIdentPart(content[start-1]) || content[start-1] == '.') {
		start--
	}

	if _, _, ok := MemberReceiverName(content[start:end]); !ok {
		return false
	}

	cursor := NewScanner(content[offset+1:])
	expression := mappingSourceExpression(cursor)

	first := NewScanner(expression).PeekSkipComments()
	if first.Kind == TokIdent && (identEq(first.Value, "new") || identEq(first.Value, "createObject")) {
		return true
	}

	if pr.FuncLookup != nil || len(pr.Resolvers) > 0 {
		return true
	}

	root, element := collectionRead(expression)
	if root == "" || element {
		return false
	}

	name := StripReceiverScope(root)
	for _, refs := range pr.funcRefsMap {
		if concreteMemberRef(refs, name) {
			return true
		}
	}

	return concreteMemberRef(pr.ComponentRefs, name)
}

func concreteMemberRef(refs []ComponentRef, name string) bool {
	ref := firstRefNamed(refs, name)

	return ref != nil && ref.Component != "" && !strings.HasPrefix(ref.Component, "$")
}

func (pr *ParseResult) memberValueSource(expression, function string, locals map[string]bool) ReturnSource {
	// The legacy scalar-chain parser strips a receiver's last dot segment.
	// Do not let a record read acquire the type of an unrelated scalar.
	if before, _, ok := strings.Cut(expression, "("); ok {
		if receiver, _, ok := strings.CutLast(strings.TrimSpace(before), "."); ok {
			if _, _, record := MemberReceiverName(receiver); record {
				root, _ := factoryReturnChain(expression)
				if pr.resolverSet == nil || pr.resolverSet.Resolve(root) == "" {
					return ReturnSource{}
				}
			}
		}
	}

	if source := pr.collectionValueSource(expression, function, locals); source.Component != "" {
		return source
	}

	root, element := collectionRead(expression)
	if _, _, record := MemberReceiverName(root); record {
		return ReturnSource{}
	}

	if root == "" || element {
		return ReturnSource{}
	}

	name := StripReceiverScope(root)
	scope := ReceiverRefScope(root)

	ref := firstRefIn(pr.funcRefsMap[function], name, scope)
	if scope != RefAny {
		ref = firstRefIn(pr.ComponentRefs, name, scope)
	}

	if ref == nil && (hasPrefixFold(root, "arguments.") || hasPrefixFold(root, "local.") || locals[function+"\t"+strings.ToLower(name)]) {
		return ReturnSource{}
	}

	if ref == nil {
		ref = firstRefIn(pr.ComponentRefs, name, scope)
	}

	if ref == nil || ref.VisibleTo != 0 {
		return ReturnSource{}
	}

	return ReturnSource{Component: pr.settledComponent(ref)}
}
