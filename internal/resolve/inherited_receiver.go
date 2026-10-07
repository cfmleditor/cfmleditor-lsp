package resolve

import "github.com/cfmleditor/cfmleditor-lsp/internal/parser"

// inheritedReceiver is what a receiver the file never types holds through its
// extends chain: a ColdBox test base's mock of the class the test names,
// asked first because the base's own ref is the $any that
// createMock( annotations.model ) gives, then a ComponentRef in a parent
// (variables.$assert assigned in a base spec).
func (r *Resolver) inheritedReceiver(variable, lookupVar string, pr *parser.ParseResult, baseDir string, tr *callTrace) string {
	if comp := r.coldboxTestSubject(variable, pr); comp != "" {
		tr.addf("resolved %q to %q: the class the test's ColdBox base mocks from its attribute", variable, comp)

		return comp
	}

	if r.fileExtends(pr) == "" {
		return ""
	}

	tr.addf("no ref found in this file — checking extends chain (%s) for a ComponentRef", r.fileExtends(pr))

	comp := ""

	r.walkExtendsRefs(r.fileExtends(pr), baseDir, lookupVar, func(ref *parser.ComponentRef, parent string) bool {
		comp = ref.Component

		tr.addf("resolved %q to %q via ComponentRef in parent %s", variable, comp, parent)

		return comp != ""
	})

	return comp
}
