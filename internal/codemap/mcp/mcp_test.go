//go:build !wasip1

package mcp_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/codemap"
	"github.com/cfmleditor/cfmleditor-lsp/internal/codemap/mcp"
	"github.com/cfmleditor/cfmleditor-lsp/internal/codemap/store"
)

func server(t *testing.T, withExplain bool) *mcp.Server {
	t.Helper()

	db, err := store.Open(filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	t.Cleanup(func() { _ = db.Close() })

	m := &codemap.Map{
		Root: "/w", Level: codemap.LevelFunction,
		Nodes: []codemap.Node{
			{ID: "page.cfm", Kind: codemap.KindFile, Name: "page.cfm", File: "page.cfm", Entry: true},
			{ID: "svc.cfc::getuser", Kind: codemap.KindFunction, Name: "getUser", File: "svc.cfc", Line: 3},
		},
		Edges: []codemap.Edge{{From: "page.cfm", To: "svc.cfc::getuser", Kind: codemap.EdgeCalls, Count: 2}},
		Stats: codemap.Stats{Files: 2, Functions: 1, CallSites: 3, Resolved: 1},
	}
	m.Annotate()

	if err := db.Save(m); err != nil {
		t.Fatalf("Save: %v", err)
	}

	s := &mcp.Server{Store: db, Name: "test", Version: "0"}
	if withExplain {
		s.Explain = func(file string, _ int, _ string) (string, error) {
			return "traced " + file, nil
		}
	}

	return s
}

// exchange runs a session and returns the responses, in order.
func exchange(t *testing.T, s *mcp.Server, requests ...string) []map[string]any {
	t.Helper()

	var out bytes.Buffer
	if err := s.Serve(strings.NewReader(strings.Join(requests, "\n")+"\n"), &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	var replies []map[string]any

	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if line == "" {
			continue
		}

		var r map[string]any
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("response is not JSON: %q: %v", line, err)
		}

		replies = append(replies, r)
	}

	return replies
}

// TestNotificationsGetNoReply is not a nicety. A client that sent
// notifications/initialized and got a response back would see an unsolicited
// message with a null id, and several clients drop the connection over it.
func TestNotificationsGetNoReply(t *testing.T) {
	replies := exchange(t, server(t, false),
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":1}}`,
		`{"jsonrpc":"2.0","id":7,"method":"ping"}`)

	if len(replies) != 1 {
		t.Fatalf("two notifications and one request produced %d replies, want 1: %v", len(replies), replies)
	}

	if replies[0]["id"] != float64(7) {
		t.Errorf("the one reply is for id %v, want 7", replies[0]["id"])
	}
}

// TestUnknownToolIsAToolErrorNotAProtocolError. MCP specifies this, and the reason
// is practical: a protocol error is the transport's problem and clients abandon
// the conversation, where a tool error is information the model can act on.
func TestUnknownToolIsAToolErrorNotAProtocolError(t *testing.T) {
	replies := exchange(t, server(t, false),
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"nope","arguments":{}}}`)

	if len(replies) != 1 {
		t.Fatalf("want one reply, got %d", len(replies))
	}

	if _, isProtocolError := replies[0]["error"]; isProtocolError {
		t.Fatal("an unknown tool produced a JSON-RPC error; it must be a tool result with isError set")
	}

	result, _ := replies[0]["result"].(map[string]any)
	if result["isError"] != true {
		t.Errorf("unknown tool did not set isError: %v", result)
	}
}

// TestUnknownMethodIsAProtocolError is the flip side: an unknown *method* is a
// protocol-level failure and must be reported as one.
func TestUnknownMethodIsAProtocolError(t *testing.T) {
	replies := exchange(t, server(t, false), `{"jsonrpc":"2.0","id":1,"method":"resources/list"}`)

	errObj, ok := replies[0]["error"].(map[string]any)
	if !ok {
		t.Fatalf("unknown method did not produce an error: %v", replies[0])
	}

	if errObj["code"] != float64(-32601) {
		t.Errorf("error code = %v, want -32601 (method not found)", errObj["code"])
	}
}

// TestMalformedJSONDoesNotKillTheSession: one bad line from a client must not end
// the conversation, or a single encoding slip takes down the whole session.
func TestMalformedJSONDoesNotKillTheSession(t *testing.T) {
	replies := exchange(t, server(t, false),
		`{"jsonrpc":"2.0",,,`,
		`{"jsonrpc":"2.0","id":2,"method":"ping"}`)

	if len(replies) != 2 {
		t.Fatalf("want a parse error then a pong, got %d replies: %v", len(replies), replies)
	}

	if _, ok := replies[0]["error"]; !ok {
		t.Error("the malformed line did not produce an error response")
	}

	if replies[1]["id"] != float64(2) {
		t.Error("the session did not survive the malformed line")
	}
}

// TestExplainToolIsOnlyAdvertisedWhenItWorks. A tool that is listed and always
// fails is worse than one that is absent: the model spends a turn discovering it.
func TestExplainToolIsOnlyAdvertisedWhenItWorks(t *testing.T) {
	for _, withExplain := range []bool{false, true} {
		replies := exchange(t, server(t, withExplain), `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)

		names := toolNames(t, replies[0])
		if got := names["explain_call"]; got != withExplain {
			t.Errorf("with Explain=%v, explain_call advertised=%v", withExplain, got)
		}

		for _, always := range []string{"search_symbols", "get_callers", "get_callees", "find_path", "list_islands", "list_orphans", "get_stats", "get_symbol"} {
			if !names[always] {
				t.Errorf("%s is not advertised", always)
			}
		}
	}
}

// TestEveryToolIsMarkedReadOnly. The server has no write path by construction, and
// the annotation is what tells a client it does not need to ask before calling.
func TestEveryToolIsMarkedReadOnly(t *testing.T) {
	replies := exchange(t, server(t, true), `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)

	result, _ := replies[0]["result"].(map[string]any)
	tools, _ := result["tools"].([]any)

	if len(tools) == 0 {
		t.Fatal("no tools advertised")
	}

	for _, raw := range tools {
		tool, _ := raw.(map[string]any)

		ann, ok := tool["annotations"].(map[string]any)
		if !ok || ann["readOnlyHint"] != true {
			t.Errorf("tool %v is not marked readOnlyHint", tool["name"])
		}
	}
}

// TestToolResultsCarryStructuredContent so a client can use the data without
// re-parsing the human-readable text block.
func TestToolResultsCarryStructuredContent(t *testing.T) {
	replies := exchange(t, server(t, false),
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_callers","arguments":{"id":"svc.cfc::getuser"}}}`)

	result, _ := replies[0]["result"].(map[string]any)
	if result["isError"] == true {
		t.Fatalf("get_callers failed: %v", result)
	}

	structured, ok := result["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("no structuredContent in %v", result)
	}

	if structured["count"] != float64(1) {
		t.Errorf("count = %v, want 1", structured["count"])
	}
}

// TestMissingRequiredArgumentIsReported rather than silently returning everything.
func TestMissingRequiredArgumentIsReported(t *testing.T) {
	replies := exchange(t, server(t, false),
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_symbol","arguments":{}}}`)

	result, _ := replies[0]["result"].(map[string]any)
	if result["isError"] != true {
		t.Fatalf("get_symbol with no id did not report an error: %v", result)
	}
}

func toolNames(t *testing.T, reply map[string]any) map[string]bool {
	t.Helper()

	result, ok := reply["result"].(map[string]any)
	if !ok {
		t.Fatalf("no result in %v", reply)
	}

	tools, _ := result["tools"].([]any)
	names := make(map[string]bool, len(tools))

	for _, raw := range tools {
		tool, _ := raw.(map[string]any)
		name, _ := tool["name"].(string)
		names[name] = true
	}

	return names
}
