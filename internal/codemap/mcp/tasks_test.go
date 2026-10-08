//go:build !wasip1

package mcp_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cfmleditor/clif/internal/codemap/mcp"
)

func listTools(t *testing.T, s *mcp.Server) []string {
	t.Helper()

	replies := exchange(t, s, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)

	result, _ := replies[0]["result"].(map[string]any)
	list, _ := result["tools"].([]any)

	var names []string

	for _, item := range list {
		tool, _ := item.(map[string]any)
		name, _ := tool["name"].(string)
		names = append(names, name)
	}

	return names
}

func callResult(t *testing.T, s *mcp.Server, call string) (text string, isError bool) {
	t.Helper()

	replies := exchange(t, s, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":`+call+`}`)

	result, _ := replies[0]["result"].(map[string]any)
	content, _ := result["content"].([]any)

	if len(content) == 0 {
		t.Fatalf("no content in %v", replies[0])
	}

	first, _ := content[0].(map[string]any)
	text, _ = first["text"].(string)
	isError, _ = result["isError"].(bool)

	return text, isError
}

// TestAServerWithoutAMapOffersTheTaskTools: the task tools read source and need
// no map, so a server started without one has them, and only them.
func TestAServerWithoutAMapOffersTheTaskTools(t *testing.T) {
	s := &mcp.Server{
		Name: "test", Version: "0",
		Unresolved: func([]string, bool, int) (any, error) { return map[string]any{}, nil },
		FindRefs:   func(string, []string, int) (any, error) { return map[string]any{}, nil },
	}

	names := listTools(t, s)
	if !slices.Equal(names, []string{"find_unresolved_calls", "find_references"}) {
		t.Errorf("tools = %v, want only the two task tools set", names)
	}

	text, isError := callResult(t, s, `{"name":"get_stats","arguments":{}}`)
	if !isError || !strings.Contains(text, "graph --db") {
		t.Errorf("a map tool without a map = %q (isError %v), want an error saying how to build one", text, isError)
	}

	replies := exchange(t, s, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	result, _ := replies[0]["result"].(map[string]any)

	if instructions, _ := result["instructions"].(string); !strings.Contains(instructions, "No code map is loaded") {
		t.Errorf("instructions do not say there is no map: %q", instructions)
	}
}

// TestLintIsOfferedOnlyWhenSet: it starts a Java process, which a server
// pointed at a checkout should not do unless asked.
func TestLintIsOfferedOnlyWhenSet(t *testing.T) {
	s := server(t, true)
	if slices.Contains(listTools(t, s), "lint") {
		t.Error("lint offered with no Lint hook")
	}

	var got []string

	s.Lint = func(paths []string, limit int) (any, error) {
		got = paths

		return mcp.Limited("findings", []int{1, 2, 3}, limit), nil
	}

	if !slices.Contains(listTools(t, s), "lint") {
		t.Error("lint not offered with a Lint hook")
	}

	text, isError := callResult(t, s, `{"name":"lint","arguments":{"paths":["a","b"],"limit":2}}`)
	if isError || !slices.Equal(got, []string{"a", "b"}) {
		t.Fatalf("lint = %q (isError %v), paths %v", text, isError, got)
	}

	for _, want := range []string{`"count": 2`, `"total": 3`, `"truncated": true`} {
		if !strings.Contains(text, want) {
			t.Errorf("lint result lacks %s:\n%s", want, text)
		}
	}
}

func TestTaskToolsNeedTheirArguments(t *testing.T) {
	s := &mcp.Server{
		Name: "test", Version: "0",
		Unresolved: func([]string, bool, int) (any, error) { return nil, nil },
		FindRefs:   func(string, []string, int) (any, error) { return nil, nil },
	}

	for _, call := range []string{
		`{"name":"find_unresolved_calls","arguments":{}}`,
		`{"name":"find_references","arguments":{"paths":["x"]}}`,
		`{"name":"find_references","arguments":{"target":"f"}}`,
	} {
		if text, isError := callResult(t, s, call); !isError || !strings.Contains(text, "missing required argument") {
			t.Errorf("%s = %q (isError %v), want a missing-argument error", call, text, isError)
		}
	}
}

func TestADefaultLimitApplies(t *testing.T) {
	var got int

	s := &mcp.Server{
		Name: "test", Version: "0",
		Unresolved: func(_ []string, _ bool, limit int) (any, error) {
			got = limit

			return map[string]any{}, nil
		},
	}

	callResult(t, s, `{"name":"find_unresolved_calls","arguments":{"paths":["x"]}}`)

	if got <= 0 {
		t.Errorf("limit passed = %d; a whole-workspace scan unbounded lands thousands of entries in the model's context", got)
	}
}
