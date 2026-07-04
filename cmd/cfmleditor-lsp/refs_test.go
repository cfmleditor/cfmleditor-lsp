package main

import (
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/refs"
)

func TestPrintMermaidRefs_ResolvedVsUnresolvedStyle(t *testing.T) {
	out := captureStdout(t, func() {
		printMermaidRefs("app.models.User", []refs.Entry{
			{File: "/proj/a.cfc", Function: "getUser", Resolved: true},
			{File: "/proj/b.cfc", Resolved: false},
		})
	})

	if !strings.Contains(out, "app_models_User[app.models.User]") {
		t.Errorf("expected target node rendered, got %q", out)
	}

	if !strings.Contains(out, "a.cfc::getUser") {
		t.Errorf("expected resolved ref to include function name, got %q", out)
	}

	if !strings.Contains(out, "ref0[a.cfc::getUser] --> app_models_User") {
		t.Errorf("expected solid arrow for resolved ref, got %q", out)
	}

	if !strings.Contains(out, "ref1[b.cfc] -.-> app_models_User") {
		t.Errorf("expected dashed arrow and no function suffix for unresolved ref, got %q", out)
	}
}
