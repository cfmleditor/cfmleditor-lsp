package resolve

import (
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/uri"
)

// TargetKind classifies how a call site's callee was identified. It is the
// difference between an edge the call graph can trust and one it has to draw
// dashed: only TargetSameFile, TargetExtends, TargetInclude and
// TargetComponent name a real definition.
type TargetKind string

// The kinds a resolved call can land on.
const (
	// TargetNone means the call did not resolve at all. The accompanying reason
	// string from CanResolveCall says why.
	TargetNone TargetKind = ""

	// TargetSameFile is a bare or `this.` call to a function the calling file
	// declares itself.
	TargetSameFile TargetKind = "samefile"

	// TargetExtends is a call satisfied by walking the extends chain.
	TargetExtends TargetKind = "extends"

	// TargetInclude is a bare call to a function declared by a file the
	// calling file shares a variables scope with through cfinclude: its
	// includer, something that includes, or a component one of those extends.
	TargetInclude TargetKind = "include"

	// TargetComponent is a qualified call into a component whose file and
	// definition were both found.
	TargetComponent TargetKind = "component"

	// TargetDynamic is a call accepted without a definition: a `$any` receiver, a
	// noFollow resolver, an onMissingMethod dispatcher, or a function-reference
	// value held in a variable, argument or VARIABLES-scoped property. Component
	// may name where the call goes; URI never does.
	TargetDynamic TargetKind = "dynamic"

	// TargetBuiltin is a method on a built-in function's return value.
	TargetBuiltin TargetKind = "builtin"

	// TargetMember is a known member or Java-interop method, accepted without
	// being found in the component.
	TargetMember TargetKind = "member"
)

// Definite reports whether the target names an actual function definition, and so
// whether URI/FuncName/Line are populated. The dynamic kinds resolve a call
// without ever finding one.
func (k TargetKind) Definite() bool {
	switch k {
	case TargetSameFile, TargetExtends, TargetInclude, TargetComponent:
		return true
	case TargetNone, TargetDynamic, TargetBuiltin, TargetMember:
		return false
	default:
		return false
	}
}

// CallTarget is where a call site resolved to.
type CallTarget struct {
	Kind      TargetKind
	Component string // dot-path of the receiving component, when one was involved
	URI       uri.URI
	FuncName  string
	Line      uint32
}

// ResolveCallTarget runs exactly the resolution CanResolveCall runs and also
// reports where the call landed. The reason string is CanResolveCall's, unchanged:
// empty when the call resolved.
//
// This exists so a caller building a call graph does not have to re-derive the
// callee that CanResolveCall already found and discarded. It is not on the lint
// hot path — it allocates a recorder per call — so CanResolveCall still passes nil
// and pays nothing.
func (r *Resolver) ResolveCallTarget(call *parser.CallSite, pr *parser.ParseResult, baseDir string) (CallTarget, string) {
	tr := &callTrace{}
	reason := r.canResolveCall(call, pr, baseDir, tr)

	return tr.target, reason
}
