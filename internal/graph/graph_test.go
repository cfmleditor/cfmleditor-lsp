package graph

import (
	"strings"
	"testing"
)

func TestMermaid_DefaultDirection(t *testing.T) {
	g := &Graph{Edges: []Edge{{From: "a", To: "b"}}}

	out := g.Mermaid()
	if got := "graph LR"; out[:len(got)] != got {
		t.Errorf("expected default direction LR, got start of output: %q", out)
	}
}

func TestMermaid_ExplicitDirection(t *testing.T) {
	g := &Graph{Direction: "TD", Edges: []Edge{{From: "a", To: "b"}}}

	out := g.Mermaid()
	if got := "graph TD"; out[:len(got)] != got {
		t.Errorf("expected explicit direction TD preserved, got start of output: %q", out)
	}
}

func TestMermaid_DashedVsSolidArrow(t *testing.T) {
	solid := (&Graph{Edges: []Edge{{From: "a", To: "b"}}}).Mermaid()
	if !strings.Contains(solid, "-->") {
		t.Errorf("expected solid arrow --> for non-dashed edge, got %q", solid)
	}

	dashed := (&Graph{Edges: []Edge{{From: "a", To: "b", Dashed: true}}}).Mermaid()
	if !strings.Contains(dashed, "-.->") {
		t.Errorf("expected dashed arrow -.-> for dashed edge, got %q", dashed)
	}
}

func TestMermaid_DedupesIdenticalEdges(t *testing.T) {
	g := &Graph{Edges: []Edge{
		{From: "a", To: "b"},
		{From: "a", To: "b"},
		{From: "a", To: "c"},
	}}

	out := g.Mermaid()
	if got := strings.Count(out, "-->"); got != 2 {
		t.Errorf("expected 2 unique edges rendered, got %d arrows in: %q", got, out)
	}
}

func TestDOT_DirectionTranslation(t *testing.T) {
	cases := []struct {
		direction string
		want      string
	}{
		{"", "rankdir=LR;"},
		{"LR", "rankdir=LR;"},
		{"TD", "rankdir=TB;"},
		{"TB", "rankdir=TB;"},
	}

	for _, c := range cases {
		out := (&Graph{Direction: c.direction, Edges: []Edge{{From: "a", To: "b"}}}).DOT()
		if !strings.Contains(out, c.want) {
			t.Errorf("direction %q: expected %q in output, got %q", c.direction, c.want, out)
		}
	}
}

func TestDOT_DashedStyle(t *testing.T) {
	out := (&Graph{Edges: []Edge{{From: "a", To: "b", Dashed: true}}}).DOT()
	if !strings.Contains(out, "[style=dashed]") {
		t.Errorf("expected dashed style annotation, got %q", out)
	}
}

func TestDOT_DedupesIdenticalEdges(t *testing.T) {
	g := &Graph{Edges: []Edge{
		{From: "a", To: "b"},
		{From: "a", To: "b"},
	}}

	out := g.DOT()
	if got := strings.Count(out, "->"); got != 1 {
		t.Errorf("expected 1 unique edge rendered, got %d in: %q", got, out)
	}
}

func TestEscapeMermaid(t *testing.T) {
	cases := map[string]string{
		`say "hi"`: `say #quot;hi#quot;`,
		`a[b]`:     `a#lsqb;b#rsqb;`,
		`pkg:sub`:  `pkg#colon;sub`,
		`a-b`:      `a#45;b`,
		`f(x)`:     `f#40;x#41;`,
		`{x}`:      `#123;x#125;`,
		`a;b`:      `a#59;b`,
		`a<b>c`:    `a#lt;b#gt;c`,
		`plain`:    `plain`,
	}

	for input, want := range cases {
		if got := escapeMermaid(input); got != want {
			t.Errorf("escapeMermaid(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestEscapeDOT(t *testing.T) {
	cases := map[string]string{
		`say "hi"`:       `say \"hi\"`,
		`back\slash`:     `back\\slash`,
		`both\and"quote`: `both\\and\"quote`,
		`plain`:          `plain`,
	}

	for input, want := range cases {
		if got := escapeDOT(input); got != want {
			t.Errorf("escapeDOT(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNodeID(t *testing.T) {
	cases := map[string]string{
		"app.models.User": "app_models_User",
		"a/b/c":           "a_b_c",
		"pkg:sub":         "pkg_sub",
		"my node":         "my_node",
		"a-b-c":           "a_b_c",
		"plain":           "plain",
	}

	for input, want := range cases {
		if got := nodeID(input); got != want {
			t.Errorf("nodeID(%q) = %q, want %q", input, got, want)
		}
	}
}
