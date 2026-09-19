package server

import (
	"path/filepath"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
)

// TestCodeMapOutputStaysInsideTheWorkspace. The command takes a path from the
// editor, which means it takes a path from whatever asked the editor to run it.
// Keeping the write inside the workspace means the worst a bad argument does is
// leave a file the user can see and delete.
func TestCodeMapOutputStaysInsideTheWorkspace(t *testing.T) {
	s := &Server{}
	root := t.TempDir()

	ok := []struct{ in, wantSuffix string }{
		{"", filepath.Join(".cfmleditor", "codemap.html")},
		{"map.html", "map.html"},
		{"reports/map.html", filepath.Join("reports", "map.html")},
	}

	for _, c := range ok {
		got, err := s.codeMapOutputPath(codeMapRequest{Out: c.in, Format: "html"}, root)
		if err != nil {
			t.Errorf("out=%q: %v", c.in, err)

			continue
		}

		if !strings.HasSuffix(got, c.wantSuffix) {
			t.Errorf("out=%q gave %q, want it to end with %q", c.in, got, c.wantSuffix)
		}

		if !strings.HasPrefix(got, root) {
			t.Errorf("out=%q escaped the workspace: %q", c.in, got)
		}
	}

	for _, bad := range []string{"../escape.html", "../../etc/passwd", "/tmp/anywhere.html"} {
		if got, err := s.codeMapOutputPath(codeMapRequest{Out: bad}, root); err == nil {
			t.Errorf("out=%q was accepted and resolved to %q", bad, got)
		}
	}
}

// TestCodeMapRequestDefaults. The command is meant to be bound to a menu item, so
// it has to work invoked with nothing — and with a partial or malformed options
// object, which is what an editor extension sends while it is being written.
func TestCodeMapRequestDefaults(t *testing.T) {
	cases := []struct {
		name   string
		params []protocol.LSPAny
		want   codeMapRequest
	}{
		{"no arguments", nil, codeMapRequest{Level: "function", Format: "html"}},
		{"empty object", []protocol.LSPAny{[]byte(`{}`)}, codeMapRequest{Level: "function", Format: "html"}},
		{"malformed", []protocol.LSPAny{[]byte(`"not an object"`)}, codeMapRequest{Level: "function", Format: "html"}},
		{"partial", []protocol.LSPAny{[]byte(`{"level":"call"}`)}, codeMapRequest{Level: "call", Format: "html"}},
		{
			"full",
			[]protocol.LSPAny{[]byte(`{"level":"package","format":"dot","under":"pkg","live":true,"open":true,"from":["a","","b"]}`)},
			codeMapRequest{Level: "package", Format: "dot", Under: "pkg", Live: true, Open: true, From: []string{"a", "b"}},
		},
		// An empty string must not override a default, or a client sending
		// {"format":""} silently gets no report at all.
		{"empty strings", []protocol.LSPAny{[]byte(`{"level":"","format":""}`)}, codeMapRequest{Level: "function", Format: "html"}},
	}

	for _, c := range cases {
		got := parseCodeMapRequest(c.params)
		if got.Level != c.want.Level || got.Format != c.want.Format || got.Under != c.want.Under ||
			got.Live != c.want.Live || got.Open != c.want.Open || len(got.From) != len(c.want.From) {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
}

// TestCodeMapExtMatchesFormat, so a default output path is not written with the
// wrong extension and opened by the wrong application.
func TestCodeMapExtMatchesFormat(t *testing.T) {
	want := map[string]string{
		"html": ".html", "json": ".json", "dot": ".dot",
		"mermaid": ".md", "text": ".txt", "": ".html", "nonsense": ".html",
	}

	for format, ext := range want {
		if got := codeMapExt(format); got != ext {
			t.Errorf("codeMapExt(%q) = %q, want %q", format, got, ext)
		}
	}
}

// TestResolveRouteWithoutAConvention answers rather than errors. An editor
// command that calls this in a workspace with no routes block should be able to
// tell the user why nothing happened, and a JSON-RPC error is not something a
// command handler can put in front of someone.
func TestResolveRouteWithoutAConvention(t *testing.T) {
	s := &Server{}

	got, err := s.handleResolveRoute([]protocol.LSPAny{[]byte(`"a.b.c"`)})
	if err != nil {
		t.Fatalf("handleResolveRoute: %v", err)
	}

	out, _ := got.(map[string]any)
	if out["reason"] == nil {
		t.Errorf("no reason given for an empty result: %v", out)
	}

	targets, _ := out["targets"].([]any)
	if len(targets) != 0 {
		t.Errorf("targets from an unconfigured workspace: %v", targets)
	}
}

// TestResolveRouteRequiresARoute: an empty argument list is a caller bug and has
// to say so, but an empty string is a user who has not typed anything yet.
func TestResolveRouteRequiresARoute(t *testing.T) {
	s := &Server{}

	if _, err := s.handleResolveRoute(nil); err == nil {
		t.Error("no arguments did not error")
	}

	got, err := s.handleResolveRoute([]protocol.LSPAny{[]byte(`""`)})
	if err != nil || got != nil {
		t.Errorf("an empty route gave (%v, %v), want (nil, nil)", got, err)
	}
}
