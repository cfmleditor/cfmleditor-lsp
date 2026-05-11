package cache

import (
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

const testURI = uri.URI("file:///test.cfc")

func TestGetFunc_Hit(t *testing.T) {
	c := New()
	c.PutFunc(testURI, "init", 100, []protocol.CompletionItem{{Label: "x"}})
	got := c.GetFunc(testURI, "init", 100)
	if len(got) != 1 || got[0].Label != "x" {
		t.Errorf("expected hit, got %v", got)
	}
}

func TestGetFunc_MissHash(t *testing.T) {
	c := New()
	c.PutFunc(testURI, "init", 100, []protocol.CompletionItem{{Label: "x"}})
	if c.GetFunc(testURI, "init", 999) != nil {
		t.Error("expected miss on different hash")
	}
}

func TestGetFile_Hit(t *testing.T) {
	c := New()
	c.PutFile(testURI, []protocol.CompletionItem{{Label: "y"}})
	got := c.GetFile(testURI)
	if len(got) != 1 || got[0].Label != "y" {
		t.Errorf("expected hit, got %v", got)
	}
}

func TestInvalidate(t *testing.T) {
	c := New()
	c.PutFunc(testURI, "init", 1, []protocol.CompletionItem{{Label: "a"}})
	c.PutFile(testURI, []protocol.CompletionItem{{Label: "b"}})
	c.Invalidate(testURI)
	if c.GetFunc(testURI, "init", 1) != nil || c.GetFile(testURI) != nil {
		t.Error("expected all cleared")
	}
}

func TestFuncCacheIndependent(t *testing.T) {
	c := New()
	c.PutFunc(testURI, "init", 1, []protocol.CompletionItem{{Label: "a"}})
	c.PutFunc(testURI, "save", 2, []protocol.CompletionItem{{Label: "b"}})
	// Overwrite init with new hash
	c.PutFunc(testURI, "init", 99, []protocol.CompletionItem{{Label: "c"}})
	// save unchanged
	if got := c.GetFunc(testURI, "save", 2); len(got) != 1 || got[0].Label != "b" {
		t.Errorf("save should be unchanged, got %v", got)
	}
	if got := c.GetFunc(testURI, "init", 99); len(got) != 1 || got[0].Label != "c" {
		t.Errorf("init should be updated, got %v", got)
	}
}

func TestHashScope(t *testing.T) {
	h1 := HashScope("a\nb\nc", 0, 2)
	h2 := HashScope("a\nX\nc", 0, 2)
	if h1 == h2 {
		t.Error("different content should have different hash")
	}
}
