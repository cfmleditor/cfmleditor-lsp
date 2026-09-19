package codemap

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// TestCompactRoundTripsEveryNode pins the encoding the viewer depends on. The
// compact form drops a node's id when it can be derived, which is the saving that
// took a real project's report from 25MB to 4MB — and also the way to silently
// mislabel every node if derivedID and FuncID ever disagree. So every node is
// decoded back the way the viewer decodes it and compared to what went in.
func TestCompactRoundTripsEveryNode(t *testing.T) {
	m := &Map{
		Root: "/tmp/x", Level: LevelFunction,
		Nodes: []Node{
			{ID: FuncID("a/b.cfc", "getthing"), Kind: KindFunction, Name: "getThing", File: "a/b.cfc", Line: 12, Access: "public", Island: 1, Reachable: true},
			{ID: FileID("a/b.cfc"), Kind: KindFile, Name: "b.cfc", File: "a/b.cfc", Island: 1, Entry: true},
			{ID: ExternalID("x.y.z"), Kind: KindExternal, Name: "x.y.z", Island: 2},
			{ID: PackageID("a"), Kind: KindPackage, Name: "a", File: "a", Island: 1, Abstract: true},
		},
		Edges: []Edge{
			{From: FileID("a/b.cfc"), To: FuncID("a/b.cfc", "getthing"), Kind: EdgeContains, Count: 1},
			{From: FuncID("a/b.cfc", "getthing"), To: ExternalID("x.y.z"), Kind: EdgeCalls, Count: 3, Dynamic: true},
		},
	}

	c := m.compact("t")

	if len(c.N) != len(m.Nodes) {
		t.Fatalf("compact dropped nodes: %d in, %d out", len(m.Nodes), len(c.N))
	}

	for i, row := range c.N {
		got := decodeID(c, row)
		if want := m.Nodes[i].ID; got != want {
			t.Errorf("node %d id round-tripped as %q, want %q", i, got, want)
		}
	}

	// contains is dropped on purpose; nothing else may be.
	if len(c.E) != 1 {
		t.Fatalf("expected 1 edge after dropping contains, got %d", len(c.E))
	}

	if from, to := decodeID(c, c.N[c.E[0][0]]), decodeID(c, c.N[c.E[0][1]]); from != m.Edges[1].From || to != m.Edges[1].To {
		t.Errorf("edge round-tripped as %s -> %s, want %s -> %s", from, to, m.Edges[1].From, m.Edges[1].To)
	}

	if c.E[0][4] != 1 {
		t.Error("the dynamic flag did not survive the round trip")
	}
}

// decodeID mirrors the viewer's reconstruction exactly. If it has to change, so
// does the JavaScript.
func decodeID(c *compactMap, row []int) string {
	str := func(i int) string {
		if i < 0 || i >= len(c.Strings) {
			return ""
		}

		return c.Strings[i]
	}

	if id := row[8]; id >= 0 {
		return str(id)
	}

	kind := c.Kinds[row[0]]
	name, file := str(row[1]), str(row[2])

	if kind == string(KindFunction) {
		return file + "::" + strings.ToLower(name)
	}

	return file
}

// TestCompactFlagsMatchTheViewer states the bitmask in full. The viewer has the
// same four constants, and a mismatch does not fail anything — it just draws entry
// points as dead code.
func TestCompactFlagsMatchTheViewer(t *testing.T) {
	want := map[string]int{"entry": 1, "reachable": 2, "root": 4, "abstract": 8, "boundary": 16}

	got := map[string]int{
		"entry": flagEntry, "reachable": flagReachable, "root": flagRoot,
		"abstract": flagAbstract, "boundary": flagBoundary,
	}
	for name, v := range want {
		if got[name] != v {
			t.Errorf("flag %s = %d, want %d — the viewer's FLAG object must be changed to match", name, got[name], v)
		}
	}

	if !strings.Contains(viewerHTML, `const FLAG = { entry: 1, reachable: 2, root: 4, abstract: 8, boundary: 16 };`) {
		t.Error("the viewer's FLAG constants no longer match this file's; decoding will mislabel nodes")
	}
}

// TestViewerIsSelfContained checks the two things that make a generated report a
// single file that works offline: no network reference, and the embedded bundle.
func TestViewerIsSelfContained(t *testing.T) {
	var buf bytes.Buffer

	m := &Map{Root: "/tmp/x", Level: LevelFunction}
	if err := m.WriteHTML(&buf, "t"); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}

	page := buf.String()

	for _, marker := range []string{"__CODEMAP_DATA__", "__D3_BUNDLE__", "__CODEMAP_TITLE__"} {
		if strings.Contains(page, marker) {
			t.Errorf("placeholder %s survived into the output", marker)
		}
	}

	// Namespace URIs inside the bundle are not fetches, so this looks for the
	// attributes that actually load something.
	for _, remote := range []string{`src="http`, `src="//`, `href="http`, `href="//`, "cdn.jsdelivr", "cdnjs.cloudflare", "unpkg.com"} {
		if strings.Contains(page, remote) {
			t.Errorf("the generated page references %q; a report must open with no network", remote)
		}
	}

	if strings.Contains(page, "<script src=") {
		t.Error("the generated page loads an external script; the D3 bundle must be inlined")
	}

	if !strings.Contains(page, "forceSimulation") {
		t.Error("the D3 bundle does not appear to be embedded")
	}
}

// TestEmbeddedBundleCannotCloseTheScriptElement is a build-time guard rather than
// a runtime escape. The bundle is inlined into a <script>, so a "</script"
// anywhere in it would end the element early and turn the rest of the page into
// markup. Escaping it is not an option — "<\/" is not valid JavaScript outside a
// string — so the only safe answer is to notice at build time.
func TestEmbeddedBundleCannotCloseTheScriptElement(t *testing.T) {
	if strings.Contains(strings.ToLower(d3Bundle), "</script") {
		t.Fatal("the vendored D3 bundle contains \"</script\"; inlining it would break every generated report. " +
			"Re-bundle with make update-d3, or serve the bundle as a separate file.")
	}
}

// TestPayloadIsInertInsideAScript checks the same hazard for the data, which does
// come from user content: a component path or file name containing "</script>"
// would otherwise end the element.
func TestPayloadIsInertInsideAScript(t *testing.T) {
	m := &Map{
		Root:  "/tmp/x",
		Level: LevelFunction,
		Nodes: []Node{{ID: "x", Kind: KindExternal, Name: `</script><img src=x onerror=alert(1)>`}},
	}

	var buf bytes.Buffer
	if err := m.WriteHTML(&buf, "t"); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}

	if strings.Contains(buf.String(), "</script><img") {
		t.Fatal("a node name closed the script element; escapeJSONForScript is not doing its job")
	}

	// And it must still decode to the original text.
	start := strings.Index(buf.String(), `id="codemap-data">`) + len(`id="codemap-data">`)
	end := strings.Index(buf.String()[start:], "</script>") + start

	var decoded compactMap
	if err := json.Unmarshal([]byte(buf.String()[start:end]), &decoded); err != nil {
		t.Fatalf("the escaped payload is no longer valid JSON: %v", err)
	}

	want := `</script><img src=x onerror=alert(1)>`
	if !slices.Contains(decoded.Strings, want) {
		t.Errorf("escaping changed the data; the string table is %q", decoded.Strings)
	}
}

// TestEveryEdgeKindIsEncodable. The compact encoder maps an edge kind to its index
// in a list, and a kind missing from that list comes back -1 — which the viewer
// decodes as the *first* kind, so a route edge silently became a call. That is the
// one distinction the route work exists to keep, and nothing about it fails
// loudly: the picture just quietly tells you something untrue.
func TestEveryEdgeKindIsEncodable(t *testing.T) {
	all := []EdgeKind{EdgeCalls, EdgeInstantiates, EdgeExtends, EdgeIncludes, EdgeRoute, EdgeContains}

	m := &Map{Root: "/w", Level: LevelFunction}

	for i, kind := range all {
		from := Node{ID: fmt.Sprintf("a%d.cfc", i), Kind: KindFile, Name: "a", File: fmt.Sprintf("a%d.cfc", i)}
		to := Node{ID: fmt.Sprintf("b%d.cfc", i), Kind: KindFile, Name: "b", File: fmt.Sprintf("b%d.cfc", i)}
		m.Nodes = append(m.Nodes, from, to)
		m.Edges = append(m.Edges, Edge{From: from.ID, To: to.ID, Kind: kind, Count: 1})
	}

	c := m.compact("t")

	for _, row := range c.E {
		if row[2] < 0 || row[2] >= len(c.Edges) {
			t.Fatalf("an edge encoded with kind index %d, outside the %d-entry kind list; "+
				"add the missing EdgeKind to compact()'s edgeKinds", row[2], len(c.Edges))
		}
	}

	// EdgeContains is dropped on purpose, so one fewer edge than kinds.
	if len(c.E) != len(all)-1 {
		t.Errorf("encoded %d edges from %d kinds; expected all but contains", len(c.E), len(all))
	}

	// And the viewer must offer a filter for each encoded kind, or an edge it
	// decodes is invisible with no way to switch it on.
	for _, kind := range c.Edges {
		if !strings.Contains(viewerHTML, `"`+kind+`"`) {
			t.Errorf("edge kind %q is encoded but the viewer's KINDS list does not mention it", kind)
		}
	}
}
