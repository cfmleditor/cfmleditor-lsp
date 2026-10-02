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

	return append(out, directoryIncludes(content)...)
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

// directoryListing is a <cfdirectory action="list"> of a directory beside the
// listing file: `directory="#getDirectoryFromPath(getCurrentTemplatePath())#sub"`.
var directoryListing = regexp.MustCompile(`(?is)<cfdirectory\b[^>]*>`)

var (
	listingAttr = regexp.MustCompile(`(?i)\b([a-z]+)\s*=\s*["']([^"']*)["']`)
	listingDir  = regexp.MustCompile(`(?i)^#\s*getDirectoryFromPath\s*\(\s*getCurrentTemplatePath\s*\(\s*\)\s*\)\s*#([\w./-]+?)/?$`)
)

// includeFromListing is <cfinclude template="sub/#q.name#">, the include of a
// file a listing named q found.
var includeFromListing = regexp.MustCompile(`(?i)<cfinclude\b[^>]*?\btemplate\s*=\s*["']([\w./-]*)#\s*([\w$]+)\.name\s*#["']`)

// directoryIncludes is the glob includes of a file that lists a directory of
// templates beside itself and includes each one it finds. Mura applies its
// database updates this way: configBean lists dbUpdates/*.cfm and includes
// every one, so each runs in configBean's variables scope.
//
// The include is a glob, `sub/*.cfm`, which the resolver expands. It is only
// recorded when the listing is literal, not recursive, filtered to templates,
// and names the directory the include's prefix does; anything computed is not
// a static answer.
func directoryIncludes(content string) []string {
	if indexFold(content, "cfdirectory") < 0 {
		return nil
	}

	comments := tagCommentSpans(content)
	listed := map[string]string{} // query name → directory, lowercased name

	for _, at := range directoryListing.FindAllStringIndex(content, -1) {
		if inSpan(comments, at[0]) {
			continue
		}

		attrs := map[string]string{}

		for _, m := range listingAttr.FindAllStringSubmatch(content[at[0]:at[1]], -1) {
			attrs[strings.ToLower(m[1])] = m[2]
		}

		dir := listingDir.FindStringSubmatch(attrs["directory"])
		if !strings.EqualFold(attrs["action"], "list") || dir == nil || attrs["name"] == "" ||
			strings.EqualFold(attrs["recurse"], "true") || strings.EqualFold(attrs["recurse"], "yes") ||
			!strings.EqualFold(attrs["filter"], "*.cfm") || strings.Contains(dir[1], "..") {
			continue
		}

		listed[strings.ToLower(attrs["name"])] = dir[1]
	}

	var out []string

	for _, at := range includeFromListing.FindAllStringSubmatchIndex(content, -1) {
		if inSpan(comments, at[0]) {
			continue
		}

		m := []string{"", content[at[2]:at[3]], content[at[4]:at[5]]}

		dir, ok := listed[strings.ToLower(m[2])]
		if !ok || !strings.EqualFold(strings.TrimSuffix(m[1], "/"), dir) {
			continue
		}

		glob := dir + "/*.cfm"
		if !slices.ContainsFunc(out, func(s string) bool { return strings.EqualFold(s, glob) }) {
			out = append(out, glob)
		}
	}

	return out
}
