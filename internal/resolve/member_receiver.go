package resolve

import "github.com/cfmleditor/cfmleditor-lsp/internal/parser"

// memberReceiver is the component a member receiver (`a.b`, `prc.x`,
// `args.x`) holds: what the file records for it, then each of the lookups that
// read it from somewhere else, the first to answer winning.
func (r *Resolver) memberReceiver(variable, name string, scope parser.RefScope, line uint32, caller, funcName string, pr *parser.ParseResult, baseDir string, tr *callTrace, ctx lookupCtx) string {
	comp := r.recordReceiver(variable, name, scope, line, caller, pr)
	if comp == "" {
		comp = r.builderMember(variable, line, caller, funcName, pr, baseDir, tr)
	}

	if comp == "" {
		comp = r.assignedFromCall(variable, line, caller, pr, baseDir, tr, ctx)
	}

	// A view's rc member is what its controller left there, and a
	// template's what the file including it holds.
	if comp == "" {
		comp = r.fw1ViewRc(variable, "", pr, tr)
	}

	if comp == "" {
		comp = r.coldboxPrcResponse(variable, pr)
	}

	if comp == "" {
		comp = r.viewArgs(variable, pr, tr)
	}

	if comp == "" {
		comp = r.includerHeld(variable, pr, tr, ctx)
	}

	return comp
}
