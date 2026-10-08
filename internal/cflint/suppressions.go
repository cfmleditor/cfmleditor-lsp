package cflint

import (
	"cmp"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/cfmleditor/clif/internal/knownissues"
)

// The two suppression forms CFLint honours, with its own code-list rule
// (com.cflint.CFLint: CFLINTIGNORE_PATTERN and CFLINT_LINE_IGNORE_PATTERN): a
// list of word characters and commas, so "@CFLintIgnore A, B" suppresses A
// only — the space ends the list — and is counted as A only.
var (
	ignoreTag  = regexp.MustCompile(`@CFLintIgnore\s+([\w,]+)`)
	lineIgnore = regexp.MustCompile(`//\s*cflint\s+ignore:([\w,]+)`)
)

// Suppressions counts the suppression comments in a set of files.
type Suppressions struct {
	Total int            `json:"total"`
	Files int            `json:"files"`
	Rules map[string]int `json:"byRule"`
}

// CountSuppressions counts @CFLintIgnore tags and `// cflint ignore:` comments
// in files, by rule: each code a comment names counts once. Files counts the
// files holding at least one. It reads the source as a grep would, so a tag
// inside a string literal counts too; nothing in CFML puts one there.
func CountSuppressions(files []string) (*Suppressions, error) {
	s := &Suppressions{Rules: map[string]int{}}

	for _, f := range files {
		data, err := os.ReadFile(f) //nolint:gosec // a source file being counted
		if err != nil {
			return nil, err
		}

		n := s.add(string(data))
		if n > 0 {
			s.Files++
		}
	}

	return s, nil
}

// add counts the suppressions in one file's content, returning how many.
func (s *Suppressions) add(content string) int {
	n := 0

	for _, re := range []*regexp.Regexp{ignoreTag, lineIgnore} {
		for _, m := range re.FindAllStringSubmatch(content, -1) {
			for code := range strings.SplitSeq(m[1], ",") {
				if code = strings.TrimSpace(code); code != "" {
					s.Rules[code]++
					s.Total++
					n++
				}
			}
		}
	}

	return n
}

// RuleCount is one rule's count.
type RuleCount struct {
	Code  string
	Count int
}

// ByCount is the rules, most suppressed first, then by code.
func (s *Suppressions) ByCount() []RuleCount {
	out := make([]RuleCount, 0, len(s.Rules))
	for code, n := range s.Rules {
		out = append(out, RuleCount{Code: code, Count: n})
	}

	slices.SortFunc(out, func(a, b RuleCount) int {
		return cmp.Or(cmp.Compare(b.Count, a.Count), cmp.Compare(a.Code, b.Code))
	})

	return out
}

// ReadSuppressionBaseline reads a baseline WriteSuppressionBaseline wrote.
func ReadSuppressionBaseline(path string) (*Suppressions, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the baseline file the caller named
	if err != nil {
		return nil, err
	}

	var s Suppressions
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("%s is not a suppression baseline: %w", path, err)
	}

	if s.Rules == nil {
		s.Rules = map[string]int{}
	}

	return &s, nil
}

// WriteSuppressionBaseline writes s as a baseline: JSON with the rules in code
// order, so a change to it reads as a one-line diff in review.
func WriteSuppressionBaseline(path string, s *Suppressions) error {
	out, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, append(out, '\n'), 0o644) //nolint:gosec // a baseline committed to the repository, read by everyone
}

// Change is how one count moved against the baseline. Code is empty for the
// total.
type Change struct {
	Code      string
	Was, Is   int
	IsNewRule bool
}

// Compare lists what rose and what fell between baseline and s: the total
// first, then each rule by code.
func Compare(baseline, s *Suppressions) (rose, fell []Change) {
	if s.Total != baseline.Total {
		c := Change{Was: baseline.Total, Is: s.Total}
		if s.Total > baseline.Total {
			rose = append(rose, c)
		} else {
			fell = append(fell, c)
		}
	}

	codes := slices.Sorted(maps.Keys(s.Rules))
	for code := range baseline.Rules {
		if _, ok := s.Rules[code]; !ok {
			codes = append(codes, code)
		}
	}

	slices.Sort(codes)

	for _, code := range slices.Compact(codes) {
		was, had := baseline.Rules[code]
		is := s.Rules[code]

		switch {
		case is > was:
			rose = append(rose, Change{Code: code, Was: was, Is: is, IsNewRule: !had})
		case is < was:
			fell = append(fell, Change{Code: code, Was: was, Is: is})
		}
	}

	return rose, fell
}

// baselineKey matches a finding to a baseline entry: the file, the rule and
// the message, not the line, so an edit above a finding does not make it new.
type baselineKey struct {
	file, code, message string
}

// Subtract removes from the run every issue the baseline already lists and
// returns how many it removed, and the baseline entries it found nothing for:
// findings since fixed, which a regenerated baseline would drop.
//
// A finding matches an entry with the same file, rule and message, wherever it
// now is, since an edit above it moves it. Each entry matches one finding, so
// when a file has more findings of one kind than the baseline lists, the extra
// are new. Which of them is new is decided by line: each entry pairs with the
// nearest finding, the closest pairs first, so a finding added in the middle
// of a file is the one reported rather than one an edit pushed down. A
// baseline records no source text, so when an insertion moves an old finding
// exactly as far as a new one lands, the two cannot be told apart; the count
// is right either way.
func (run *Run) Subtract(baseline []knownissues.Entry) (matched int, stale []knownissues.Entry) {
	entries := map[baselineKey][]int{}

	for i := range baseline {
		e := &baseline[i]
		k := baselineKey{filepath.Clean(e.Path), e.Code, e.Message}
		entries[k] = append(entries[k], i)
	}

	issues := map[baselineKey][]int{}

	for i := range run.Issues {
		loc := firstLocation(&run.Issues[i].Issue)
		k := baselineKey{filepath.Clean(loc.File), run.Issues[i].ID, loc.Message}
		issues[k] = append(issues[k], i)
	}

	known := make([]bool, len(run.Issues))
	used := make([]bool, len(baseline))

	for k, idx := range issues {
		matched += pairNearest(run.Issues, idx, baseline, entries[k], known, used)
	}

	kept := run.Issues[:0]

	for i := range run.Issues {
		if !known[i] {
			kept = append(kept, run.Issues[i])
		}
	}

	run.Issues = kept

	for i := range baseline {
		if !used[i] {
			stale = append(stale, baseline[i])
		}
	}

	slices.SortFunc(stale, func(a, b knownissues.Entry) int {
		return cmp.Or(cmp.Compare(a.Path, b.Path), cmp.Compare(a.Line, b.Line))
	})

	return matched, stale
}

// pairNearest matches the issues idx to the baseline entries ents, all with
// one key, closest lines first, marking each pair in known and used, and
// returns how many pairs it made.
func pairNearest(issues []RunIssue, idx []int, baseline []knownissues.Entry, ents []int, known, used []bool) int {
	n := 0

	for {
		bestIssue, bestEntry, bestDist := -1, -1, 0

		for _, i := range idx {
			if known[i] {
				continue
			}

			// CFLint's lines are 1-based; a known-issues entry's are 0-based.
			line := firstLocation(&issues[i].Issue).Line - 1

			for _, e := range ents {
				if used[e] {
					continue
				}

				if d := abs(baseline[e].Line - line); bestIssue < 0 || d < bestDist {
					bestIssue, bestEntry, bestDist = i, e, d
				}
			}
		}

		if bestIssue < 0 {
			return n
		}

		known[bestIssue], used[bestEntry] = true, true
		n++
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}

	return n
}
