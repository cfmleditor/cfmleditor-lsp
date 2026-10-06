package parser

import "strings"

// The scanners here read the include and directory-listing forms by hand. They
// were regular expressions, and the include scan runs over every file the
// index takes and over the open document on every edit that changes its
// signatures: a whole-file pattern cost a 65,000-line component 10ms a
// keystroke. Each mirrors the expression it replaced exactly, and
// TestIncludeScannersMatchTheirExpressions holds them to it.

// isRegexpSpace is \s in Go's regexp syntax: no vertical tab.
func isRegexpSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\f' || c == '\r'
}

// isWordByte is \w.
func isWordByte(c byte) bool {
	return isAlnum(c) || c == '_'
}

func skipRegexpSpace(s string, i int) int {
	for i < len(s) && isRegexpSpace(s[i]) {
		i++
	}

	return i
}

// wordStartAt is \b before s[i] when s[i] is a word byte.
func wordStartAt(s string, i int) bool {
	return i == 0 || !isWordByte(s[i-1])
}

// wordEndAt is \b after a word ending before s[i].
func wordEndAt(s string, i int) bool {
	return i >= len(s) || !isWordByte(s[i])
}

func isQuote(c byte) bool { return c == '"' || c == '\'' }

// tagAttrValues calls value for each place, from i up to the tag's first '>',
// that `\bname\s*=\s*` stands, with the offset just past it, until value
// reports a match. It is the `[^>]*?\bname\s*=\s*` of a tag pattern: the
// earliest occurrence whose remainder matches wins, as a lazy repeat's would.
func tagAttrValues(s string, i int, name string, value func(j int) bool) bool {
	end := strings.IndexByte(s[i:], '>')
	if end < 0 {
		end = len(s)
	} else {
		end += i
	}

	for p := i; p < end; p++ {
		// The attribute name holds no '>', so the name may run past end only
		// as the tail of a lazy repeat would: it starts before the '>'.
		if !hasPrefixFold(s[p:], name) || !wordStartAt(s, p) {
			continue
		}

		j := skipRegexpSpace(s, p+len(name))
		if j >= len(s) || s[j] != '=' {
			continue
		}

		if value(skipRegexpSpace(s, j+1)) {
			return true
		}
	}

	return false
}

// quotedPath reads `["']([^"'#]+)["']` at j: the path's bounds.
func quotedPath(s string, j int) (start, end int, ok bool) {
	if j >= len(s) || !isQuote(s[j]) {
		return 0, 0, false
	}

	k := j + 1
	for k < len(s) && !isQuote(s[k]) && s[k] != '#' {
		k++
	}

	if k == j+1 || k >= len(s) || !isQuote(s[k]) {
		return 0, 0, false
	}

	return j + 1, k, true
}

// includeTagAt reads `^(?i)<cfinclude\b[^>]*?\btemplate\s*=\s*["']([^"'#]+)["']`
// and returns the path's bounds.
func includeTagAt(s string) (start, end int, ok bool) {
	const open = "<cfinclude"
	if !hasPrefixFold(s, open) || !wordEndAt(s, len(open)) {
		return 0, 0, false
	}

	found := tagAttrValues(s, len(open), "template", func(j int) bool {
		start, end, ok = quotedPath(s, j)

		return ok
	})

	return start, end, found
}

// includeScriptAt reads
// `^(?i)(?:cf)?include\s*\(?\s*(?:template\s*=\s*)?["']([^"'#]+)["']`.
func includeScriptAt(s string) (start, end int, ok bool) {
	i := 0
	if hasPrefixFold(s, "cf") {
		i = 2
	}

	if !hasPrefixFold(s[i:], "include") {
		return 0, 0, false
	}

	i = skipRegexpSpace(s, i+len("include"))
	if i < len(s) && s[i] == '(' {
		i++
	}

	i = skipRegexpSpace(s, i)

	// The optional template= is taken only when it is whole: a "template"
	// without its '=' leaves the quote test to fail on its 't', which is
	// where the pattern's other branch fails too.
	if hasPrefixFold(s[i:], "template") {
		if j := skipRegexpSpace(s, i+len("template")); j < len(s) && s[j] == '=' {
			i = skipRegexpSpace(s, j+1)
		}
	}

	return quotedPath(s, i)
}

// directoryTagEnd reads `^(?is)<cfdirectory\b[^>]*>` and returns the offset
// past the '>'.
func directoryTagEnd(s string) (int, bool) {
	const open = "<cfdirectory"
	if !hasPrefixFold(s, open) || !wordEndAt(s, len(open)) {
		return 0, false
	}

	end := strings.IndexByte(s[len(open):], '>')
	if end < 0 {
		return 0, false
	}

	return len(open) + end + 1, true
}

// listingAttrs reads every `(?i)\b([a-z]+)\s*=\s*["']([^"']*)["']` in a tag, in
// order and without overlap, as name lowercased → value; a later name wins.
func listingAttrs(s string) map[string]string {
	attrs := map[string]string{}

	for i := 0; i < len(s); {
		if !isASCIILetter(s[i]) || !wordStartAt(s, i) {
			i++

			continue
		}

		j := i
		for j < len(s) && isASCIILetter(s[j]) {
			j++
		}

		name := s[i:j]

		k := skipRegexpSpace(s, j)
		if k >= len(s) || s[k] != '=' {
			i = j

			continue
		}

		k = skipRegexpSpace(s, k+1)
		if k >= len(s) || !isQuote(s[k]) {
			i = j

			continue
		}

		v := k + 1
		for v < len(s) && !isQuote(s[v]) {
			v++
		}

		if v >= len(s) {
			i = j

			continue
		}

		attrs[strings.ToLower(name)] = s[k+1 : v]
		i = v + 1
	}

	return attrs
}

func isASCIILetter(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// listingDir reads
// `(?i)^#\s*getDirectoryFromPath\s*\(\s*getCurrentTemplatePath\s*\(\s*\)\s*\)\s*#([\w./-]+?)/?$`
// and returns the directory it names.
func listingDir(s string) (string, bool) {
	i := 0

	for _, part := range []string{"#", "getDirectoryFromPath", "(", "getCurrentTemplatePath", "(", ")", ")", "#"} {
		if part != "#" || i > 0 {
			i = skipRegexpSpace(s, i)
		}

		if !hasPrefixFold(s[i:], part) {
			return "", false
		}

		i += len(part)
	}

	rest := s[i:]
	if rest == "" {
		return "", false
	}

	for j := range len(rest) {
		if c := rest[j]; !isWordByte(c) && c != '.' && c != '/' && c != '-' {
			return "", false
		}
	}

	// The lazy capture leaves one trailing '/' to the optional one, unless
	// that would leave the capture empty.
	if len(rest) > 1 && rest[len(rest)-1] == '/' {
		rest = rest[:len(rest)-1]
	}

	return rest, true
}

// listingIncludeAt reads
// `^(?i)<cfinclude\b[^>]*?\btemplate\s*=\s*["']([\w./-]*)#\s*([\w$]+)\.name\s*#["']`
// and returns the prefix, the query name, and the offset past the match.
func listingIncludeAt(s string) (prefix, query string, end int, ok bool) {
	const open = "<cfinclude"
	if !hasPrefixFold(s, open) || !wordEndAt(s, len(open)) {
		return "", "", 0, false
	}

	ok = tagAttrValues(s, len(open), "template", func(j int) bool {
		if j >= len(s) || !isQuote(s[j]) {
			return false
		}

		k := j + 1
		for k < len(s) && (isWordByte(s[k]) || s[k] == '.' || s[k] == '/' || s[k] == '-') {
			k++
		}

		if k >= len(s) || s[k] != '#' {
			return false
		}

		q := skipRegexpSpace(s, k+1)

		r := q
		for r < len(s) && (isWordByte(s[r]) || s[r] == '$') {
			r++
		}

		if r == q || !hasPrefixFold(s[r:], ".name") {
			return false
		}

		e := skipRegexpSpace(s, r+len(".name"))
		if e+1 >= len(s) || s[e] != '#' || !isQuote(s[e+1]) {
			return false
		}

		prefix, query, end = s[j+1:k], s[q:r], e+2

		return true
	})

	return prefix, query, end, ok
}
