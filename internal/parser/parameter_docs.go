package parser

import "strings"

// applyParameterDocs recognizes both JSDoc and CFML's argument metadata.
// doc_generic describes a component only when its value is a dotted path
// and the declaration is generic; array element types are not receivers.
func applyParameterDocs(comment string, args []Argument) {
	applyJSDocParams(comment, args)

	for line := range strings.SplitSeq(comment, "\n") {
		_, annotation, ok := strings.Cut(line, "@")
		if !ok {
			continue
		}

		end := strings.IndexAny(annotation, " \t\r")
		if end < 0 {
			continue
		}

		name, kind, ok := strings.Cut(annotation[:end], ".")
		if !ok || !strings.EqualFold(kind, "doc_generic") {
			continue
		}

		value := strings.TrimSpace(annotation[end:])
		if end := strings.IndexAny(value, " \t\r"); end >= 0 {
			value = value[:end]
		}

		if !isComponentType(value) {
			continue
		}

		for i := range args {
			a := &args[i]
			if strings.EqualFold(a.Name, name) && (a.Type == "" || strings.EqualFold(a.Type, "any") || strings.EqualFold(a.Type, "struct")) {
				a.Type = value
			}
		}
	}
}
