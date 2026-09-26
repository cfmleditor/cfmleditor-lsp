package parser

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"
)

// The tag scanner names the same set of letters twice: once in the fast skip
// that decides which tags are worth reading at all, and once in the switch that
// decides what to do with one. They are a hand-maintained parallel list, and
// each way of disagreeing is silent.
//
// A letter in the skip with no case is a tag scanned, line-numbered and then
// dropped — pure cost, no behaviour. A case whose letter the skip rejects is
// unreachable, so the feature it implements simply does not work: that is
// exactly how <cfloop index="x"> came to declare nothing, since 'l' was absent
// from the skip while nothing in the switch looked wrong.
func TestTagDispatchMatchesTheFastSkip(t *testing.T) {
	src := readTagParserSource(t)

	skip := fastSkipLetters(t, src)
	cases := dispatchCaseLetters(t, src)

	if len(skip) == 0 || len(cases) == 0 {
		t.Fatalf("could not read the letters: skip=%v cases=%v", skip, cases)
	}

	for _, l := range skip {
		// '/' admits closing tags, which are handled before the switch.
		if l == "/" {
			continue
		}

		if !contains(cases, l) {
			t.Errorf("fast skip admits tags starting cf%s but the switch has no case for it — those tags are scanned and dropped", l)
		}
	}

	for _, l := range cases {
		if !contains(skip, l) {
			t.Errorf("the switch has a case for cf%s but the fast skip rejects it — that case is unreachable", l)
		}
	}
}

func readTagParserSource(t *testing.T) string {
	t.Helper()

	_, file, _, _ := runtime.Caller(0)

	data, err := os.ReadFile(filepath.Join(filepath.Dir(file), "tag_parser.go"))
	if err != nil {
		t.Fatalf("reading tag_parser.go: %v", err)
	}

	return string(data)
}

var (
	skipLineRe = regexp.MustCompile(`if ch != '.'(?: && ch != '.')* \{`)
	letterRe   = regexp.MustCompile(`'(.)'`)
	caseRe     = regexp.MustCompile(`(?m)^\t{3}case '(.)':$`)
)

func fastSkipLetters(t *testing.T, src string) []string {
	t.Helper()

	line := skipLineRe.FindString(src)
	if line == "" {
		t.Fatal("could not find the fast-skip condition; update this test if the scan changed shape")
	}

	var out []string

	for _, m := range letterRe.FindAllStringSubmatch(line, -1) {
		out = append(out, m[1])
	}

	sort.Strings(out)

	return out
}

func dispatchCaseLetters(t *testing.T, src string) []string {
	t.Helper()

	idx := strings.Index(src, "switch ch {")
	if idx < 0 {
		t.Fatal("could not find the dispatch switch; update this test if the scan changed shape")
	}

	var out []string

	for _, m := range caseRe.FindAllStringSubmatch(src[idx:], -1) {
		out = append(out, m[1])
	}

	sort.Strings(out)

	return out
}

func contains(haystack []string, needle string) bool {
	return slices.Contains(haystack, needle)
}
