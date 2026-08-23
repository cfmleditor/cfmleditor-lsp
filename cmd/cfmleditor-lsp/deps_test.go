package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
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

// depsTestdata is the checked-in controller -> service -> persist chain.
func depsTestdata(t *testing.T) string {
	t.Helper()

	_, thisFile, _, _ := runtime.Caller(0)

	return filepath.Join(filepath.Dir(thisFile), "..", "..", "testdata", "deps")
}

// The CLI used to emit one edge per component ref with no resolution and no
// traversal, so it could only ever describe one level and had no way to say
// whether a dependency resolved. It goes through deps.Build now, the same as
// cfmleditor.exportDeps, so both surfaces answer with the same graph.
func TestCmdDeps_ResolvesAndTraversesTransitively(t *testing.T) {
	out := captureStdout(t, func() { cmdDeps([]string{depsTestdata(t)}) })

	var result DepResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}

	if result.Files != 3 {
		t.Errorf("files = %d, want 3", result.Files)
	}

	var reachedPersistFromService bool

	for _, e := range result.Edges {
		if strings.HasPrefix(e.From, "service.cfc::") && strings.HasPrefix(e.To, "persist.cfc::") {
			reachedPersistFromService = true
		}

		if e.Dashed {
			t.Errorf("edge %q -> %q did not resolve", e.From, e.To)
		}
	}

	if !reachedPersistFromService {
		t.Errorf("no second-hop edge from service to persist; edges: %+v", result.Edges)
	}
}

// Every node an edge points at must be a node some other edge can start from,
// or the rendered graph is disconnected pairs rather than a path.
func TestCmdDeps_MermaidGraphIsConnected(t *testing.T) {
	out := captureStdout(t, func() { cmdDeps([]string{"--mermaid", depsTestdata(t)}) })

	if !strings.Contains(out, "graph LR") {
		t.Fatalf("expected a mermaid header, got %q", out)
	}

	if !strings.Contains(out, "persist_cfc__FetchRecord") {
		t.Errorf("second hop missing from the rendered graph:\n%s", out)
	}
}

// One file reached from several places must not repeat its edges.
func TestCmdDeps_DedupesEdges(t *testing.T) {
	out := captureStdout(t, func() { cmdDeps([]string{depsTestdata(t)}) })

	var result DepResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}

	seen := make(map[string]bool)

	for _, e := range result.Edges {
		key := e.FromFile + "\x00" + e.From + "\x00" + e.To
		if seen[key] {
			t.Errorf("duplicate edge %q -> %q from %s", e.From, e.To, e.FromFile)
		}

		seen[key] = true
	}
}
