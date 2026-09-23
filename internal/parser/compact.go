package parser

import "strings"

// CompactDefs returns defs whose strings own their memory.
//
// Every string a parse produces is a slice of the file's source, and a Go
// substring keeps the whole backing array alive. That is right for a parse
// result, which is discarded with the source it came from, and wrong for the
// index, which outlives it: one function name retains the entire file it was
// declared in, so an index of a workspace retains the workspace.
//
// Measured on a real one — 18,245 files, 322MB of source, 85,712 declarations —
// the index held 285MB, 0.9x the source it was built from, for data whose own
// size is a few megabytes.
//
// The unexported returnVar is the reason this lives here rather than in the
// index: it is a substring like the rest, and nothing outside this package can
// clone it. A copy that misses one field frees nothing, because one surviving
// substring pins the file just as well as five.
func CompactDefs(defs []FunctionDef) []FunctionDef {
	if len(defs) == 0 {
		return defs
	}

	out := make([]FunctionDef, len(defs))

	for i := range defs {
		d := defs[i]
		d.Name = strings.Clone(d.Name)
		d.ReturnType = strings.Clone(d.ReturnType)
		d.ReturnComponent = strings.Clone(d.ReturnComponent)
		d.returnVar = strings.Clone(d.returnVar)

		// An empty argument list is dropped rather than kept: the script
		// parser allocates room for four arguments up front, so a function
		// that takes none arrived here holding an empty slice with 224 bytes
		// behind it, and copying d kept that array. On a 5,632-component
		// corpus that was 1.9MB, a sixth of the whole index.
		if len(d.Arguments) == 0 {
			d.Arguments = nil
		} else {
			args := make([]Argument, len(d.Arguments))

			for j := range d.Arguments {
				a := d.Arguments[j]
				a.Name = strings.Clone(a.Name)
				a.Type = strings.Clone(a.Type)
				a.Hint = strings.Clone(a.Hint)
				args[j] = a
			}

			d.Arguments = args
		}

		out[i] = d
	}

	return out
}

// CompactRefs is CompactDefs for component references, for the same reason.
func CompactRefs(refs []ComponentRef) []ComponentRef {
	if len(refs) == 0 {
		return refs
	}

	out := make([]ComponentRef, len(refs))

	for i := range refs {
		r := refs[i]
		r.Variable = strings.Clone(r.Variable)
		r.Component = strings.Clone(r.Component)
		r.ChainBase = strings.Clone(r.ChainBase)
		r.ChainMethod = strings.Clone(r.ChainMethod)
		r.ChainRest = nil
		out[i] = r
	}

	return out
}
