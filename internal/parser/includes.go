package parser

import (
	"path"
	"slices"
	"strings"
)

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
	var out []string

	for _, site := range IncludeSites(content) {
		if !slices.ContainsFunc(out, func(s string) bool { return strings.EqualFold(s, site.Path) }) {
			out = append(out, site.Path)
		}
	}

	return out
}

// IncludeSite is one include statement: the path as ExtractIncludes reports
// it, and the byte offset the statement starts at.
type IncludeSite struct {
	Path   string
	Offset int
}

// IncludeSites is every include ExtractIncludes reads, one per statement and
// with where it is, so a template can be read as its includer sees it at
// that line: a template included inside a function sees the function's locals.
// The script and tag forms come first, in source order, then the directory
// listings' globs.
func IncludeSites(content string) []IncludeSite {
	var (
		out      []IncludeSite
		comments [][2]int
		listed   map[string]string // query name → directory, lowercased name
		dirEnd   int               // where the last listing tag ended
	)

	// One walk over the text finds the three things the scan needs: each
	// include keyword, each <cfdirectory, and each <!--- comment, which is
	// stepped over whole as soon as it is met, so nothing inside one is ever
	// read. It was three searches over the whole text, and on a 65,000-line
	// component they ran on every edit that changed a signature.
	for i := 0; i < len(content); {
		// A tight loop to the next byte that can start anything, then the
		// byte after it tested inline before any prefix is compared: an 'i'
		// starts one identifier in three, and a '<' every tag.
		for i < len(content) && !includeScanByte[content[i]] {
			i++
		}

		if i+1 >= len(content) {
			break
		}

		c := content[i]
		if !includeScanNext(c, content[i+1]) {
			i++

			continue
		}

		if c == '<' {
			if strings.HasPrefix(content[i:], "<!---") {
				end := commentEnd(content, i)
				comments = append(comments, [2]int{i, end})
				i = end

				continue
			}

			if i >= dirEnd {
				end, dir, name, ok := listingAt(content, i)
				dirEnd = max(dirEnd, end)

				if ok {
					listed = withListing(listed, name, dir)
				}
			}

			i++

			continue
		}

		// c is 'i' or 'I'.
		if !hasPrefixFold(content[i:], "include") {
			i++

			continue
		}

		if site, ok := includeSiteAt(content, i); ok {
			out = append(out, site)
		}

		i += len("include")
	}

	if len(listed) > 0 {
		out = append(out, listingIncludes(content, listed, comments)...)
	}

	return out
}

// includeScanByte marks the bytes IncludeSites stops at: the '<' that opens a
// comment or a <cfdirectory, and the first letter of an include keyword.
var includeScanByte = [256]bool{'<': true, 'i': true, 'I': true}

// includeScanNext reports whether next can follow c in what IncludeSites
// looks for: "<!---", "<cfdirectory" or "include".
func includeScanNext(c, next byte) bool {
	if c == '<' {
		return next == '!' || next|0x20 == 'c'
	}

	return next|0x20 == 'n'
}

// listingAt reads a <cfdirectory> tag at i, returning where it ends — a tag
// that begins inside it is read as part of it, as a whole-file match would —
// or 0 when none starts there, and the directory and query name when it is a
// listing listingIncludes can follow.
func listingAt(content string, i int) (end int, dir, name string, ok bool) {
	n, found := directoryTagEnd(content[i:])
	if !found {
		return 0, "", "", false
	}

	dir, name, ok = qualifyingListing(content[i : i+n])

	return i + n, dir, name, ok
}

// withListing records a listing, making the map on first use: most files have
// none.
func withListing(listed map[string]string, name, dir string) map[string]string {
	if listed == nil {
		listed = map[string]string{}
	}

	listed[name] = dir

	return listed
}

// includeSiteAt reads the include whose keyword starts at i, if it is one.
func includeSiteAt(content string, i int) (IncludeSite, bool) {
	start, form := includeFormAt(content, i)
	if form == nil {
		return IncludeSite{}, false
	}

	from, to, ok := form(content[start:min(len(content), start+includeWindow)])
	if !ok {
		return IncludeSite{}, false
	}

	p := strings.TrimSpace(content[start+from : start+to])
	if glob, ok := computedNameGlob(p); ok {
		p = glob
	}

	if p == "" || strings.Contains(p, "://") || !isIncludable(p) {
		return IncludeSite{}, false
	}

	return IncludeSite{Path: p, Offset: start}, true
}

// includeFormAt decides which form the "include" found at i begins, and where
// that form starts: "<cfinclude" at the '<', "cfinclude" or "include" at the
// keyword. It returns a nil reader for an occurrence that is neither — the
// tail of a longer identifier or a member call such as arr.include(…).
func includeFormAt(content string, i int) (int, func(string) (int, int, bool)) {
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

// commentEnd is the offset past the ---> that closes the <!--- at open,
// counting the comments nested in it; the end of s when it is never closed.
func commentEnd(s string, open int) int {
	depth := 1
	j := open + len("<!---")

	for depth > 0 {
		nextClose := strings.Index(s[j:], "--->")
		if nextClose < 0 {
			return len(s)
		}

		// A nested open matters only if it starts before the close, so the
		// search for one stops there — it may still run into the close, as
		// in "<!--->". Unbounded, it read on to the next comment in the file,
		// a second pass over the text for every comment in it.
		nextOpen := strings.Index(s[j:min(len(s), j+nextClose+len("<!---")-1)], "<!---")

		if nextOpen >= 0 && nextOpen < nextClose {
			depth++
			j += nextOpen + len("<!---")

			continue
		}

		depth--
		j += nextClose + len("--->")
	}

	return j
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

// qualifyingListing reads one <cfdirectory> tag and reports the directory
// and lowercased query name of a listing listingIncludes can follow: a
// literal directory beside the listing file, not recursive, and filtered to
// templates. Anything computed is not a static answer.
func qualifyingListing(tag string) (dir, name string, ok bool) {
	attrs := listingAttrs(tag)

	dir, ok = listingDir(attrs["directory"])
	if !strings.EqualFold(attrs["action"], "list") || !ok || attrs["name"] == "" ||
		strings.EqualFold(attrs["recurse"], "true") || strings.EqualFold(attrs["recurse"], "yes") ||
		!strings.EqualFold(attrs["filter"], "*.cfm") || strings.Contains(dir, "..") {
		return "", "", false
	}

	return dir, strings.ToLower(attrs["name"]), true
}

// listingIncludes is the glob includes of a file that lists a directory of
// templates beside itself and includes each one it finds. Mura applies its
// database updates this way: configBean lists dbUpdates/*.cfm and includes
// every one, so each runs in configBean's variables scope.
//
// The include is a glob, `sub/*.cfm`, which the resolver expands. listed is
// the qualifying listings by query name, and comments the file's comments.
func listingIncludes(content string, listed map[string]string, comments [][2]int) []IncludeSite {
	var out []IncludeSite

	const include = "<cfinclude"

	end := 0

	for i := indexFold(content, include); i >= 0; i = indexFoldFrom(content, include, i+len(include)) {
		if i < end {
			continue
		}

		prefix, query, n, ok := listingIncludeAt(content[i:])
		if !ok {
			continue
		}

		end = i + n

		if inSpan(comments, i) {
			continue
		}

		dir, ok := listed[strings.ToLower(query)]
		if !ok || !strings.EqualFold(strings.TrimSuffix(prefix, "/"), dir) {
			continue
		}

		out = append(out, IncludeSite{Path: dir + "/*.cfm", Offset: i})
	}

	return out
}

// computedNameGlob reads an include whose file name is computed and whose
// directory is not — `#current.action#.cfm`, `layouts/#docFormat#.cfm` — as
// every template in that directory, the glob a directory listing gives. It
// is how a dispatcher picks a page by name: Lucee's admin web.cfm includes
// `#url.action#.cfm`, so every page beside it runs there and calls the
// helpers web_functions.cfm declares. The name must be one #...# span and
// nothing else, the extension .cfm, and the directory relative and literal:
// a mapping, a computed directory or a name with a literal part is not read.
func computedNameGlob(p string) (string, bool) {
	dir, name := path.Split(p)
	if len(name) < len("#x#.cfm") || !strings.EqualFold(name[len(name)-len(".cfm"):], ".cfm") {
		return "", false
	}

	stem := name[:len(name)-len(".cfm")]
	if stem[0] != '#' || stem[len(stem)-1] != '#' || strings.Count(stem, "#") != 2 ||
		strings.ContainsAny(dir, "#*\\") || strings.HasPrefix(dir, "/") {
		return "", false
	}

	if dir == "" {
		return "./*.cfm", true
	}

	return dir + "*.cfm", true
}
