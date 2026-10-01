package parser

import (
	"maps"
	"strings"
)

type memberDependency struct {
	receiver string
	methods  []string
}

func (pr *ParseResult) memberWriteDependency(write collectionWrite) (string, []string) {
	if write.unknown {
		return "", nil
	}

	return pr.memberDependencySource(write.expression)
}

// Read the whole record receiver. The scalar fallback must not borrow a
// different binding with the same final segment.
func (pr *ParseResult) memberDependencySource(expression string) (string, []string) {
	if receiver, element := collectionRead(expression); receiver != "" && !element {
		if _, _, record := MemberReceiverName(receiver); record {
			return receiver, nil
		}
	}

	root, methods := factoryReturnChain(expression)
	if root == "" || pr.resolverSet != nil && pr.resolverSet.Resolve(root) != "" {
		return "", nil
	}

	before, _, _ := strings.Cut(root, "(")

	receiver, method, ok := strings.CutLast(strings.TrimSpace(before), ".")
	if !ok {
		return "", nil
	}

	if _, _, record := MemberReceiverName(receiver); !record {
		return "", nil
	}

	return receiver, append([]string{method}, methods...)
}

// A concrete write grounds a dependency cycle. Every producer and method
// result must agree, including self-updates; unknown or conflicting writes
// invalidate their consumers. Work is bounded independently of file size.
func (pr *ParseResult) settleMemberDependencies(nodes map[string]*collectionNode, dependencies map[string][]memberDependency) map[string]string {
	components := map[string]string{}
	reverse := map[string][]string{}

	for identity, node := range nodes {
		components[identity] = pr.memberSourceComponent(node)
		if len(node.sources) > 0 && components[identity] == "" {
			node.invalid = true
		}

		for _, dep := range dependencies[identity] {
			reverse[dep.receiver] = append(reverse[dep.receiver], identity)
		}
	}

	if len(dependencies) == 0 {
		return components
	}

	for range 16 {
		changed := false
		current := maps.Clone(components)

		for identity, deps := range dependencies {
			node := nodes[identity]
			if node.invalid {
				continue
			}

			for _, dep := range deps {
				component := current[dep.receiver]
				if component == "" {
					continue
				}

				component = pr.memberDependencyComponent(component, dep.methods)
				if component == "" || components[identity] != "" && !strings.EqualFold(components[identity], component) {
					node.invalid = true

					break
				}

				if components[identity] == "" {
					components[identity] = component
					changed = true
				}
			}
		}

		if !changed {
			break
		}
	}

	queue := []string{}

	for identity, node := range nodes {
		if !pr.memberDependenciesAgree(identity, dependencies[identity], nodes, components) {
			node.invalid = true
		}

		if node.invalid || components[identity] == "" {
			node.invalid = true

			queue = append(queue, identity)
		}
	}

	for len(queue) > 0 {
		identity := queue[0]
		queue = queue[1:]

		components[identity] = ""
		for _, consumer := range reverse[identity] {
			if !nodes[consumer].invalid {
				nodes[consumer].invalid = true
				queue = append(queue, consumer)
			}
		}
	}

	return components
}

func (pr *ParseResult) memberDependenciesAgree(identity string, dependencies []memberDependency, nodes map[string]*collectionNode, components map[string]string) bool {
	for _, dep := range dependencies {
		producer := nodes[dep.receiver]
		if producer == nil || producer.invalid || components[dep.receiver] == "" {
			return false
		}

		component := pr.memberDependencyComponent(components[dep.receiver], dep.methods)
		if component == "" || !strings.EqualFold(component, components[identity]) {
			return false
		}
	}

	return true
}

func (pr *ParseResult) memberDependencyComponent(component string, methods []string) string {
	if pr.FuncLookup != nil {
		component = pr.walkChainRest(component, methods)
	} else {
		component = dynamicIfTyped(component, methods)
	}

	if strings.HasPrefix(component, "$") {
		return ""
	}

	return component
}
