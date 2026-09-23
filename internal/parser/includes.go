package parser

import (
	"path"
	"regexp"
	"slices"
	"strings"
)

// includeTagAt matches <cfinclude template="…"> starting at the '<'. A path
// holding a # is dynamic and has no static answer, so the class excludes it
// rather than a caller having to.
var includeTagAt = regexp.MustCompile(`^(?i)<cfinclude\b[^>]*?\btemplate\s*=\s*["']([^"'#]+)["']`)

// includeScriptAt matches the script forms starting at the keyword: include
// "x.cfm", include template="x.cfm" and cfinclude(template="x.cfm").
var includeScriptAt = regexp.MustCompile(`^(?i)(?:cf)?include\s*\(?\s*(?:template\s*=\s*)?["']([^"'#]+)["']`)

// includeWindow bounds how far past the keyword a match may reach. A tag with
// its attributes or a script include with its path is far shorter; the bound
// only has to stop a stray keyword costing a scan to the end of the file.
const includeWindow = 512

// ExtractIncludes returns the static paths a file cfincludes, in source order
// and without duplicates, exactly as written.
//
// An included file runs in its includer's variables scope, so a function one
// declares is callable unqualified from the other. That is the relationship the
// resolver needs, and it is why this is separate from ExtractLinks: a link is
// anything that names a file — href, action, <cfmodule template> — and a
// cfmodule runs in a scope of its own, so reading one as an include would
// resolve calls through a file they can never reach.
//
// Only a path naming a CFML file counts, which is also what keeps the script
// form off prose that happens to say `include "…"`. An include inside a
// <!--- ---> comment is skipped, since a commented-out include is not a live one.
//
// It runs on every file the index takes, so it looks for the keyword and
// matches only there. The first version ran two case-insensitive patterns over
// the whole file and copied it to blank its comments, which on tassweb took the
// index pass from 1.5s to 5.5s.
func ExtractIncludes(content string) []string {
	var (
		out      []string
		comments [][2]int
		scanned  bool
	)

	for i := indexFold(content, "include"); i >= 0; i = indexFoldFrom(content, "include", i+len("include")) {
		start, re := includeFormAt(content, i)
		if re == nil {
			continue
		}

		m := re.FindStringSubmatchIndex(content[start:min(len(content), start+includeWindow)])
		if m == nil {
			continue
		}

		p := strings.TrimSpace(content[start+m[2] : start+m[3]])
		if p == "" || strings.Contains(p, "://") || !isIncludable(p) {
			continue
		}

		if !scanned {
			comments = tagCommentSpans(content)
			scanned = true
		}

		if inSpan(comments, start) {
			continue
		}

		if !slices.ContainsFunc(out, func(s string) bool { return strings.EqualFold(s, p) }) {
			out = append(out, p)
		}
	}

	return out
}

// includeFormAt decides which form the "include" found at i begins, and where
// that form starts: "<cfinclude" at the '<', "cfinclude" or "include" at the
// keyword. It returns a nil pattern for an occurrence that is neither — the
// tail of a longer identifier or a member call such as arr.include(…).
func includeFormAt(content string, i int) (int, *regexp.Regexp) {
	start := i
	if i >= 2 && strings.EqualFold(content[i-2:i], "cf") {
		start = i - 2
	}

	if start >= 1 && content[start-1] == '<' {
		if start == i {
			return 0, nil // "<include" is not a CFML tag
		}

		return start - 1, includeTagAt
	}

	if start >= 1 {
		if c := content[start-1]; c == '.' || c == '_' || isAlnum(c) {
			return 0, nil
		}
	}

	return start, includeScriptAt
}

func isAlnum(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// isIncludable reports whether p names a template cfinclude can run. It is not
// cfpath.IsCFMLFile, which imports this package.
func isIncludable(p string) bool {
	ext := strings.ToLower(path.Ext(p))

	return ext == ".cfm" || ext == ".cfml"
}

// tagCommentSpans returns the outermost <!--- … ---> comments, which nest, as
// [start, end) offsets in source order. An unclosed one runs to the end.
func tagCommentSpans(s string) [][2]int {
	var spans [][2]int

	for i := 0; ; {
		open := strings.Index(s[i:], "<!---")
		if open < 0 {
			return spans
		}

		open += i
		depth := 1
		j := open + len("<!---")

		for depth > 0 {
			nextOpen := strings.Index(s[j:], "<!---")
			nextClose := strings.Index(s[j:], "--->")

			if nextClose < 0 {
				return append(spans, [2]int{open, len(s)})
			}

			if nextOpen >= 0 && nextOpen < nextClose {
				depth++
				j += nextOpen + len("<!---")

				continue
			}

			depth--
			j += nextClose + len("--->")
		}

		spans = append(spans, [2]int{open, j})
		i = j
	}
}

// inSpan reports whether pos falls inside one of spans, which are sorted and
// do not overlap.
func inSpan(spans [][2]int, pos int) bool {
	k, _ := slices.BinarySearchFunc(spans, pos, func(sp [2]int, p int) int {
		switch {
		case sp[1] <= p:
			return -1
		case sp[0] > p:
			return 1
		default:
			return 0
		}
	})

	return k < len(spans) && spans[k][0] <= pos && pos < spans[k][1]
}
