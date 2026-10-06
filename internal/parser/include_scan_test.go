package parser

import (
	"maps"
	"math/rand/v2"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The expressions the include scanners replaced, and the scan built on them as
// it was, kept as the reference the scanners must agree with.
var (
	refIncludeTagAt       = regexp.MustCompile(`^(?i)<cfinclude\b[^>]*?\btemplate\s*=\s*["']([^"'#]+)["']`)
	refIncludeScriptAt    = regexp.MustCompile(`^(?i)(?:cf)?include\s*\(?\s*(?:template\s*=\s*)?["']([^"'#]+)["']`)
	refDirectoryListing   = regexp.MustCompile(`(?is)<cfdirectory\b[^>]*>`)
	refListingAttr        = regexp.MustCompile(`(?i)\b([a-z]+)\s*=\s*["']([^"']*)["']`)
	refListingDir         = regexp.MustCompile(`(?i)^#\s*getDirectoryFromPath\s*\(\s*getCurrentTemplatePath\s*\(\s*\)\s*\)\s*#([\w./-]+?)/?$`)
	refIncludeFromListing = regexp.MustCompile(`(?i)<cfinclude\b[^>]*?\btemplate\s*=\s*["']([\w./-]*)#\s*([\w$]+)\.name\s*#["']`)
)

func includeSitesRegexp(content string) []IncludeSite {
	var (
		out      []IncludeSite
		comments [][2]int
		scanned  bool
	)

	for i := indexFold(content, "include"); i >= 0; i = indexFoldFrom(content, "include", i+len("include")) {
		start, re := includeFormAtRegexp(content, i)
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

		out = append(out, IncludeSite{Path: p, Offset: start})
	}

	return append(out, directoryIncludesRegexp(content)...)
}

func includeFormAtRegexp(content string, i int) (int, *regexp.Regexp) {
	start := i
	if i >= 2 && strings.EqualFold(content[i-2:i], "cf") {
		start = i - 2
	}

	if start >= 1 && content[start-1] == '<' {
		if start == i {
			return 0, nil // "<include" is not a CFML tag
		}

		return start - 1, refIncludeTagAt
	}

	if start >= 1 {
		if c := content[start-1]; c == '.' || c == '_' || isAlnum(c) {
			return 0, nil
		}
	}

	return start, refIncludeScriptAt
}

func directoryIncludesRegexp(content string) []IncludeSite {
	if indexFold(content, "cfdirectory") < 0 {
		return nil
	}

	comments := tagCommentSpans(content)
	listed := map[string]string{} // query name → directory, lowercased name

	for _, at := range refDirectoryListing.FindAllStringIndex(content, -1) {
		if inSpan(comments, at[0]) {
			continue
		}

		attrs := map[string]string{}

		for _, m := range refListingAttr.FindAllStringSubmatch(content[at[0]:at[1]], -1) {
			attrs[strings.ToLower(m[1])] = m[2]
		}

		dir := refListingDir.FindStringSubmatch(attrs["directory"])
		if !strings.EqualFold(attrs["action"], "list") || dir == nil || attrs["name"] == "" ||
			strings.EqualFold(attrs["recurse"], "true") || strings.EqualFold(attrs["recurse"], "yes") ||
			!strings.EqualFold(attrs["filter"], "*.cfm") || strings.Contains(dir[1], "..") {
			continue
		}

		listed[strings.ToLower(attrs["name"])] = dir[1]
	}

	var out []IncludeSite

	for _, at := range refIncludeFromListing.FindAllStringSubmatchIndex(content, -1) {
		if inSpan(comments, at[0]) {
			continue
		}

		m := []string{"", content[at[2]:at[3]], content[at[4]:at[5]]}

		dir, ok := listed[strings.ToLower(m[2])]
		if !ok || !strings.EqualFold(strings.TrimSuffix(m[1], "/"), dir) {
			continue
		}

		out = append(out, IncludeSite{Path: dir + "/*.cfm", Offset: at[0]})
	}

	return out
}

// tagCommentSpans, for the reference scan, returns the outermost <!--- … --->
// comments, which nest, as [start, end) offsets in source order. An unclosed
// one runs to the end.
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

var includeSamples = map[string]string{
	"tag":                     `<cfinclude template="a.cfm">`,
	"tag upper":               `<CFINCLUDE TEMPLATE = 'b.cfml' >`,
	"tag later attr":          `<cfinclude x="1" template="c.cfm">`,
	"tag dynamic":             `<cfinclude template="#x#.cfm">`,
	"tag dynamic then static": `<cfinclude template="#x#" template="d.cfm">`,
	"tag no boundary":         `<cfincludex template="e.cfm">`,
	"tag attr word":           `<cfinclude mytemplate="f.cfm">`,
	"tag after end":           `<cfinclude a="1"> template="g.cfm"`,
	"script":                  `include "h.cfm";`,
	"script paren":            `cfinclude( template = "i.cfm" );`,
	"script template no eq":   `include template "j.cfm";`,
	"script mixed quotes":     `include "k.cfm';`,
	"member":                  `arr.include("l.cfm")`,
	"ident tail":              `myinclude "m.cfm"`,
	"comment":                 `<!--- <cfinclude template="n.cfm"> --->`,
	"url":                     `<cfinclude template="http://x/o.cfm">`,
	"not cfml":                `<cfinclude template="p.js">`,
	"spaces in path":          `<cfinclude template=" q.cfm ">`,
	"open overlapping close":  `<!--- a <!---> <cfinclude template="r.cfm"> ---> <cfinclude template="s.cfm">`,
	"nested comments":         `<!--- <!--- x ---> <cfinclude template="t.cfm"> ---> <cfinclude template="u.cfm">`,
	"listing": `<cfdirectory action="list" directory="#getDirectoryFromPath(getCurrentTemplatePath())#dbUpdates" name="rsUpdates" filter="*.cfm">` +
		`<cfloop query="rsUpdates"><cfinclude template="dbUpdates/#rsUpdates.name#"></cfloop>`,
	"listing spaced": `<CFDIRECTORY ACTION='list' DIRECTORY='# getDirectoryFromPath ( getCurrentTemplatePath ( ) ) #up/' NAME='q' FILTER='*.cfm'>` +
		`<cfinclude template='up/# q.NAME #'>`,
	"listing slash only": `<cfdirectory action="list" directory="#getDirectoryFromPath(getCurrentTemplatePath())#/" name="q" filter="*.cfm"><cfinclude template="/#q.name#">`,
	"listing recursive":  `<cfdirectory action="list" recurse="yes" directory="#getDirectoryFromPath(getCurrentTemplatePath())#a" name="q" filter="*.cfm"><cfinclude template="a/#q.name#">`,
	"listing nested open": `<cfdirectory <cfdirectory action="list" directory="#getDirectoryFromPath(getCurrentTemplatePath())#a" name="q" filter="*.cfm">` +
		`<cfinclude template="a/#q.name#">`,
	"listing unclosed": `<cfdirectory action="list" directory="#getDirectoryFromPath(getCurrentTemplatePath())#a" name="q" filter="*.cfm"`,
	"listing commented": `<!--- <cfdirectory action="list" directory="#getDirectoryFromPath(getCurrentTemplatePath())#a" name="q" filter="*.cfm"> --->` +
		`<cfinclude template="a/#q.name#">`,
}

func TestIncludeScannersMatchTheirExpressions(t *testing.T) {
	for name, src := range includeSamples {
		if got, want := IncludeSites(src), includeSitesRegexp(src); !slices.Equal(got, want) {
			t.Errorf("%s: scanners %v, expressions %v", name, got, want)
		}
	}

	// Splice fragments of the samples together at random, so the scanners
	// meet every form cut short, run together and nested in the others.
	var pieces []string

	for _, src := range includeSamples {
		for i := 0; i < len(src); i += 3 {
			pieces = append(pieces, src[i:min(len(src), i+1+i%11)])
		}
	}

	pieces = append(pieces, " ", "\t", "\n", "\v", "=", "'", `"`, "#", ">", "<", "(", ")", ".", "/", "-", "$", "_", "<!---", "--->",
		"include", "cfinclude", "<cfinclude", "<!--->", "<!--", "-->", "template", "<cfdirectory", "name", ".name", "q", "list", "*.cfm", "a.cfm",
		"getDirectoryFromPath", "getCurrentTemplatePath", "directory", "filter", "action")
	slices.Sort(pieces)

	rng := rand.New(rand.NewPCG(3, 4))

	for range 100000 {
		var b strings.Builder
		for range rng.IntN(24) {
			b.WriteString(pieces[rng.IntN(len(pieces))])
		}

		src := b.String()
		if got, want := IncludeSites(src), includeSitesRegexp(src); !slices.Equal(got, want) {
			t.Fatalf("%q: scanners %v, expressions %v", src, got, want)
		}
	}
}

func TestListingScannersMatchTheirExpressions(t *testing.T) {
	rng := rand.New(rand.NewPCG(5, 6))
	pieces := []string{"#", " ", "\t", "getDirectoryFromPath", "GETCURRENTTEMPLATEPATH", "getCurrentTemplatePath", "(", ")", "/", "a", ".", "-", "_", "x y", "#", "", "1"}
	attrPieces := []string{" ", "a", "Name", "=", "'", `"`, "x", "1", "_", "-", ">", "action", "list", "\n", "é"}

	for range 100000 {
		var dir, tag strings.Builder
		for range rng.IntN(12) {
			dir.WriteString(pieces[rng.IntN(len(pieces))])
		}

		for range rng.IntN(16) {
			tag.WriteString(attrPieces[rng.IntN(len(attrPieces))])
		}

		gotDir, ok := listingDir(dir.String())

		want := refListingDir.FindStringSubmatch(dir.String())
		if ok != (want != nil) || ok && gotDir != want[1] {
			t.Fatalf("listingDir(%q) = %q, %v; expression %v", dir.String(), gotDir, ok, want)
		}

		wantAttrs := map[string]string{}
		for _, m := range refListingAttr.FindAllStringSubmatch(tag.String(), -1) {
			wantAttrs[strings.ToLower(m[1])] = m[2]
		}

		if got := listingAttrs(tag.String()); !maps.Equal(got, wantAttrs) {
			t.Fatalf("listingAttrs(%q) = %v; expression %v", tag.String(), got, wantAttrs)
		}
	}
}
