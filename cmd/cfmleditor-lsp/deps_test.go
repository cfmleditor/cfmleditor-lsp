package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout redirects os.Stdout for the duration of fn and returns everything written to it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	orig := os.Stdout
	os.Stdout = w

	defer func() { os.Stdout = orig }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}

	return buf.String()
}

func TestPrintMermaidDeps_BasicEdge(t *testing.T) {
	out := captureStdout(t, func() {
		printMermaidDeps(DepResult{Edges: []DepEdge{
			{FromFile: "/proj/a.cfc", ToComponent: "app.models.User"},
		}})
	})

	if !strings.Contains(out, "graph LR") {
		t.Errorf("expected mermaid header, got %q", out)
	}

	if !strings.Contains(out, "a_cfc[a.cfc] --> app_models_User[app.models.User]") {
		t.Errorf("expected rendered edge, got %q", out)
	}
}

func TestPrintMermaidDeps_IncludesFunctionNameInFromLabel(t *testing.T) {
	out := captureStdout(t, func() {
		printMermaidDeps(DepResult{Edges: []DepEdge{
			{FromFile: "/proj/a.cfc", FromFunction: "getUser", ToComponent: "app.models.User"},
		}})
	})

	if !strings.Contains(out, "a.cfc::getUser") {
		t.Errorf("expected from-label to include function name, got %q", out)
	}
}

func TestPrintMermaidDeps_DedupesIdenticalEdges(t *testing.T) {
	out := captureStdout(t, func() {
		printMermaidDeps(DepResult{Edges: []DepEdge{
			{FromFile: "/proj/a.cfc", ToComponent: "app.models.User"},
			{FromFile: "/proj/a.cfc", ToComponent: "app.models.User"},
		}})
	})

	if got := strings.Count(out, "-->"); got != 1 {
		t.Errorf("expected 1 deduped edge, got %d in %q", got, out)
	}
}
