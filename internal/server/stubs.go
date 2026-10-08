package server

import (
	"github.com/cfmleditor/clif/internal/frameworkapi"
	"github.com/cfmleditor/clif/internal/parser"
	cfpath "github.com/cfmleditor/clif/internal/path"
	"go.lsp.dev/protocol"
)

// A framework's bundled API stands in for its source when the source is not
// in the workspace (internal/frameworkapi): calls on it resolve, completion
// lists it and hover documents it. Its files exist nowhere an editor can
// open, so no answer that sends the editor to a location may name one.

// withoutStubLocations drops stub locations from a definition-style answer:
// a Location, a *Location, or a list of either. Nothing left is no answer.
func withoutStubLocations(v any) any {
	switch l := v.(type) {
	case protocol.Location:
		if frameworkapi.IsStubURI(string(l.URI)) {
			return nil
		}
	case *protocol.Location:
		if l != nil && frameworkapi.IsStubURI(string(l.URI)) {
			return nil
		}
	case []protocol.Location:
		kept := make([]protocol.Location, 0, len(l))
		for i := range l {
			if !frameworkapi.IsStubURI(string(l[i].URI)) {
				kept = append(kept, l[i])
			}
		}

		switch len(kept) {
		case 0:
			return nil
		case 1:
			return kept[0]
		default:
			return kept
		}
	case []protocol.LocationLink:
		kept := make([]protocol.LocationLink, 0, len(l))
		for i := range l {
			if !frameworkapi.IsStubURI(string(l[i].TargetURI)) {
				kept = append(kept, l[i])
			}
		}

		if len(kept) == 0 {
			return nil
		}

		return kept
	}

	return v
}

// stubDoc is the documentation of a function the bundled API declares, or
// nil for any other.
func stubDoc(def *parser.FunctionDef) *frameworkapi.Doc {
	if def == nil || !frameworkapi.IsStubURI(string(def.URI)) {
		return nil
	}

	d, ok := frameworkapi.DocAt(cfpath.FromURI(string(def.URI)), def.Line)
	if !ok {
		return nil
	}

	return d
}

// stubDocMarkup is stubDoc as completion documentation.
func stubDocMarkup(def *parser.FunctionDef) *protocol.MarkupContent {
	d := stubDoc(def)
	if d == nil {
		return nil
	}

	return &protocol.MarkupContent{Kind: protocol.MarkupKindMarkdown, Value: d.Markdown()}
}
