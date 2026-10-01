package parser

import (
	"strings"
)

// A DI setter is a public setName(Name) method. Declared and documented types
// outrank name-based injection. The lookup is supplied only for managed files.
func applySetterArgumentTypes(name, access string, args []Argument, lookup func(string) string) {
	if lookup == nil || len(args) != 1 || len(name) <= 3 || !strings.EqualFold(name[:3], "set") || (access != "" && !strings.EqualFold(access, "public")) {
		return
	}

	arg := &args[0]
	if !strings.EqualFold(name[3:], arg.Name) {
		return
	}

	switch strings.ToLower(arg.Type) {
	case "", "any", "component", "object":
		if comp := lookup(arg.Name); comp != "" {
			arg.Component = comp
		}
	default:
		// Primitive and explicit component declarations retain their contract.
	}
}

// Only the whole arguments.name value carries the argument's component type.
// Members, concatenations and calls must not acquire the dependency's type.
func wholeArgumentComponent(chain, inFunc string, funcs []FunctionDef) string {
	scope, name, ok := strings.Cut(chain, ".")
	if !ok || !strings.EqualFold(scope, "arguments") || inFunc == "" || len(funcs) == 0 {
		return ""
	}

	for _, arg := range funcs[len(funcs)-1].Arguments {
		if strings.EqualFold(arg.Name, name) {
			return argumentComponentType(&arg)
		}
	}

	return ""
}

// Keep declared signatures suitable for hover and completion, while using an
// inferred dependency's resolved file for member checks and field assignments.
func argumentComponentType(arg *Argument) string {
	if arg.Component != "" {
		return arg.Component
	}

	if isComponentType(arg.Type) {
		return arg.Type
	}

	return ""
}

// DI/1 injects every known init argument by name, including transients. Keep
// declared/documented contracts and leave missing or overridden values unknown.
func applyConstructorArgumentTypes(name, access string, args []Argument, lookup func(string) string) {
	if lookup == nil || !strings.EqualFold(name, "init") || access != "" && !strings.EqualFold(access, "public") {
		return
	}

	for i := range args {
		arg := &args[i]
		if arg.Component != "" {
			continue
		}

		if arg.Type != "" && !strings.EqualFold(arg.Type, "any") && !strings.EqualFold(arg.Type, "component") && !strings.EqualFold(arg.Type, "object") {
			continue
		}

		arg.Component = lookup(arg.Name)
	}
}
