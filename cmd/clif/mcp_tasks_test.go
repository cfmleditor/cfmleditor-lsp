package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func repoTestdata(t *testing.T) string {
	t.Helper()

	_, file, _, _ := runtime.Caller(0)

	return filepath.Join(filepath.Dir(file), "..", "..", "testdata")
}

func TestMCPFlagsChooseTheTools(t *testing.T) {
	plain := mcpServer(&mcpFlags{}, nil)
	if plain.Unresolved == nil || plain.FindRefs == nil || plain.Explain == nil {
		t.Error("a plain server lacks a task tool")
	}

	if plain.Lint != nil {
		t.Error("lint offered without --allow-lint")
	}

	if mcpServer(&mcpFlags{allowLint: true}, nil).Lint == nil {
		t.Error("--allow-lint did not offer lint")
	}

	mapOnly := mcpServer(&mcpFlags{mapOnly: true, db: "x"}, nil)
	if mapOnly.Unresolved != nil || mapOnly.FindRefs != nil || mapOnly.Explain != nil || mapOnly.Lint != nil {
		t.Error("--map-only offered a tool that reads source")
	}
}

func TestAMissingPathIsAnErrorNotAnEmptyAnswer(t *testing.T) {
	if _, err := mcpUnresolved([]string{filepath.Join(t.TempDir(), "nope")}, false, 10); err == nil {
		t.Error("unresolved over a missing path answered; it would read as no findings")
	}

	if _, err := mcpFindRefs("f", []string{filepath.Join(t.TempDir(), "nope")}, 10); err == nil {
		t.Error("refs over a missing path answered")
	}
}

// TestMCPLinesAreOneBased: the parser numbers lines from zero, and explain_call,
// editors and the CLI's text from one. Each line reported must hold its call.
func TestMCPLinesAreOneBased(t *testing.T) {
	td := repoTestdata(t)

	res, err := mcpUnresolved([]string{td}, false, 1000)
	if err != nil {
		t.Fatal(err)
	}

	m, _ := res.(map[string]any)

	calls, _ := m["calls"].([]unresolvedCall)
	if len(calls) == 0 {
		t.Fatal("no unresolved calls in testdata; the check below would pass vacuously")
	}

	// Every one, not any: a neighbouring line can hold the same name.
	for _, c := range calls {
		if c.Unchecked > 0 {
			continue // the entry for a broken extends chain sits on the extends line
		}

		name := c.Call[strings.LastIndex(c.Call, ".")+1:] + "("
		if !strings.Contains(lineOf(t, c.File, int(c.Line)), name) {
			t.Errorf("%s:%d does not hold %s; lines are off by one", c.File, c.Line, name)
		}
	}

	refsRes, err := mcpFindRefs("GetData", []string{filepath.Join(td, "refs")}, 10)
	if err != nil {
		t.Fatal(err)
	}

	rm, _ := refsRes.(map[string]any)
	refs, _ := rm["references"].([]reference)

	if len(refs) == 0 {
		t.Fatal("no references to GetData in testdata/refs")
	}

	// The call text is the whole line, and the line above GetData's call is
	// GetData's declaration, so a name check would pass off by one.
	for _, r := range refs {
		if got := strings.TrimSpace(lineOf(t, r.File, int(r.Line))); got != strings.TrimSpace(r.Call) {
			t.Errorf("%s:%d is %q, want the reference %q; lines are off by one", r.File, r.Line, got, r.Call)
		}
	}
}

func lineOf(t *testing.T, file string, line int) string {
	t.Helper()

	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(string(data), "\n")
	if line < 1 || line > len(lines) {
		return ""
	}

	return lines[line-1]
}
