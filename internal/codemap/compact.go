package codemap

import "strings"

// compactMap is the wire form the HTML viewer receives. It exists because the
// obvious encoding does not survive contact with a real project: a function-level
// map of an 11,700-file codebase is 60,000 nodes and 105,000 edges, and writing
// each one as a JSON object with named keys and full string ids produced a 25MB
// page. The same graph in this form is a few megabytes.
//
// Three things account for the difference, all of them repetition:
//
//   - Every string is interned. A file path is repeated once per function the file
//     declares, and a name like "init" or "getValue" hundreds of times across the
//     workspace.
//   - An edge names its endpoints by node position, not by id. Node ids are long
//     ("packages/tass/core/user.cfc::checkpermslist") and each appears once as a
//     node and many times as an endpoint; two small integers replace two long
//     strings 105,000 times over.
//   - A node's id is not stored when it can be derived. A function's id is its
//     file, "::", and its lower-cased name; a file's id is its path. Only the
//     external nodes, which have no file, need one written out.
//
// The `contains` edges are dropped entirely: they are the file→function hierarchy,
// the viewer discards them on load, and they are nearly half of all edges. Every
// answer they carried is already baked into each node's file and island.
type compactMap struct {
	Title string `json:"title"`
	Root  string `json:"root"`
	Level Level  `json:"level"`
	Stats Stats  `json:"stats"`
	// HideUtility opens the report with the utility toggle off. It is a starting
	// position, not a filter — the nodes are all present.
	HideUtility bool `json:"hideUtility,omitempty"`

	Strings []string `json:"s"`
	Kinds   []string `json:"nk"`
	Edges   []string `json:"ek"`

	// Nodes is [kind, name, file, line, access, component, island, flags, id],
	// where name/file/access/component/id are indexes into Strings (-1 for absent)
	// and id is -1 when it can be derived. Flags is a bitmask: 1 entry,
	// 2 reachable, 4 root, 8 abstract.
	N [][]int `json:"n"`

	// E is [from, to, kind, count, dynamic], from/to being indexes into N.
	E [][]int `json:"e"`
}

// Node flag bits in the compact encoding. The viewer has the same list; a change
// here without one there silently mislabels entry points as dead code.
const (
	flagEntry = 1 << iota
	flagReachable
	flagRoot
	flagAbstract
	flagBoundary
	flagUtility
)

type interner struct {
	index map[string]int
	list  []string
}

func newInterner() *interner {
	return &interner{index: make(map[string]int)}
}

// put returns the table index for s, or -1 for the empty string so the decoder can
// tell "absent" from "the empty string" without a sentinel entry.
func (in *interner) put(s string) int {
	if s == "" {
		return -1
	}

	if i, ok := in.index[s]; ok {
		return i
	}

	i := len(in.list)
	in.index[s] = i
	in.list = append(in.list, s)

	return i
}

func (m *Map) compact(title string) *compactMap {
	nodeKinds := []string{string(KindFunction), string(KindFile), string(KindExternal), string(KindPackage)}
	// Every EdgeKind the encoder can emit must be listed, or its index comes back
	// -1 and the viewer decodes the edge as the first kind instead — route edges
	// silently became calls, which is the one distinction the map exists to keep.
	// TestEveryEdgeKindIsEncodable pins the list against the constants.
	edgeKinds := []string{
		string(EdgeCalls), string(EdgeInstantiates), string(EdgeExtends),
		string(EdgeIncludes), string(EdgeRoute),
	}

	nodeKindIdx := indexOf(nodeKinds)
	edgeKindIdx := indexOf(edgeKinds)

	in := newInterner()
	out := &compactMap{
		Title: title, Root: m.Root, Level: m.Level, Stats: m.Stats,
		Kinds: nodeKinds, Edges: edgeKinds,
		N: make([][]int, 0, len(m.Nodes)),
		E: make([][]int, 0, len(m.Edges)),
	}

	pos := make(map[string]int, len(m.Nodes))

	for i := range m.Nodes {
		n := &m.Nodes[i]
		pos[n.ID] = len(out.N)

		flags := 0
		if n.Entry {
			flags |= flagEntry
		}

		if n.Reachable {
			flags |= flagReachable
		}

		if n.Root {
			flags |= flagRoot
		}

		if n.Abstract {
			flags |= flagAbstract
		}

		if n.Boundary {
			flags |= flagBoundary
		}

		if n.Utility {
			flags |= flagUtility
		}

		id := -1
		if n.ID != derivedID(n) {
			id = in.put(n.ID)
		}

		out.N = append(out.N, []int{
			nodeKindIdx(string(n.Kind)),
			in.put(n.Name),
			in.put(n.File),
			int(n.Line),
			in.put(n.Access),
			in.put(n.Component),
			n.Island,
			flags,
			id,
		})
	}

	for i := range m.Edges {
		e := &m.Edges[i]
		if e.Kind == EdgeContains {
			continue
		}

		from, okFrom := pos[e.From]
		to, okTo := pos[e.To]

		if !okFrom || !okTo {
			continue
		}

		dyn := 0
		if e.Dynamic {
			dyn = 1
		}

		out.E = append(out.E, []int{from, to, edgeKindIdx(string(e.Kind)), e.Count, dyn})
	}

	out.Strings = in.list

	return out
}

// derivedID is the id the decoder will reconstruct when none is written. It must
// stay in step with FuncID and FileID — a node whose real id differs from this is
// written out in full, so a mismatch costs bytes rather than correctness.
func derivedID(n *Node) string {
	if n.Kind == KindFunction {
		return FuncID(n.File, strings.ToLower(n.Name))
	}

	return n.File
}

func indexOf(values []string) func(string) int {
	idx := make(map[string]int, len(values))
	for i, v := range values {
		idx[v] = i
	}

	return func(v string) int {
		if i, ok := idx[v]; ok {
			return i
		}

		return -1
	}
}
