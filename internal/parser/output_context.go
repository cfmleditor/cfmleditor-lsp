package parser

import (
	"regexp"
	"strings"
)

// Where ColdFusion evaluates #...# in the text of a tag file.
//
// Outside the contexts below, a hash in text is a literal character: a CSS
// colour, an href="#", a jQuery "#id" selector. Pairing those across a chunk
// of text swallowed whatever JavaScript, CSS or attribute text sat between two
// of them, and every name( in it became a call. The rules were checked against
// Adobe ColdFusion 2025 with one probe template per context:
//
//	evaluated      <cfoutput> bodies, including HTML attributes and <script>
//	               inside them; <cfquery> and <cfmail> bodies; the body of a
//	               <cffunction output="true|yes">; the attributes of every
//	               <cf...> tag, <cf_...> custom tag, <cfmodule> and
//	               cfimport-prefixed tag (<t:echo v="#x()#">), in any context
//	not evaluated  plain text, HTML attributes and <script> bodies outside
//	               those; <cfsavecontent>, <cfxml> and custom-tag bodies
//	               outside <cfoutput>; the body of a <cffunction> whose
//	               output is unset or false; a template <cfinclude>d from
//	               inside a <cfoutput> (each template is compiled on its own)
//
// <cfmailpart> is taken with <cfmail>, and <cfdocument> and its item and
// section tags are taken as evaluated: the probe server has no PDF service to
// confirm them, and wrongly scanning costs a false positive where wrongly
// skipping would lose a call. <cfcomponent output="true"> is taken the same
// way; the probe could not exercise it, because ColdFusion rejects free
// #...# text in a component body at compile time.
//
// Attributes are handled by the tag walk, not here: this answers only whether
// the text between tags is output.
var outputContextTags = map[string]bool{
	"cfoutput":          true,
	"cfquery":           true,
	"cfmail":            true,
	"cfmailpart":        true,
	"cfdocument":        true,
	"cfdocumentitem":    true,
	"cfdocumentsection": true,
}

// outputContext returns the byte ranges of content whose text ColdFusion
// evaluates, sorted and disjoint, and the tag prefixes content declares with
// <cfimport prefix="...">, lowercased.
//
// It fails open. A range left unclosed at the end of the file runs to the end,
// and a close with nothing open is ignored, so a file the walk misreads is
// scanned more than it should be rather than less.
func outputContext(content string) (spans [][2]int, prefixes []string) {
	var (
		depth           int
		fnOut, compOut  bool
		start           = -1
		wasActive       bool
		boundaryChanged = func(at int) {
			active := depth > 0 || fnOut || compOut
			switch {
			case active && !wasActive:
				start = at
			case !active && wasActive && start >= 0:
				if at > start {
					spans = append(spans, [2]int{start, at})
				}

				start = -1
			}

			wasActive = active
		}
	)

	for pos := 0; pos < len(content); {
		i := strings.IndexByte(content[pos:], '<')
		if i < 0 {
			break
		}

		i += pos

		if strings.HasPrefix(content[i:], "<!---") {
			pos = skipTagComment(content, i)

			continue
		}

		closing := i+1 < len(content) && content[i+1] == '/'

		nameStart := i + 1
		if closing {
			nameStart++
		}

		nameEnd := nameStart
		for nameEnd < len(content) && isTagNameByte(content[nameEnd]) {
			nameEnd++
		}

		name := strings.ToLower(content[nameStart:nameEnd])
		if !strings.HasPrefix(name, "cf") {
			pos = i + 1

			continue
		}

		end := tagEndIndex(content[i:])
		if end < 0 {
			pos = i + 1

			continue
		}

		tagEnd := i + end + 1
		tag := content[i:tagEnd]
		selfClosing := strings.HasSuffix(strings.TrimRight(tag[:len(tag)-1], " \t\r\n"), "/")

		switch {
		case name == "cfscript" && !closing:
			// A script body holds no tag text, and a "<cfoutput" in a string
			// there is not a tag.
			if c := indexCFTag(content[tagEnd:], "/cfscript>"); c >= 0 {
				pos = tagEnd + c

				continue
			}

			pos = len(content)

			continue
		case name == "cfimport" && !closing:
			if pf := strings.TrimSpace(getAttr(tag, "prefix")); pf != "" {
				prefixes = append(prefixes, strings.ToLower(pf))
			}
		case outputContextTags[name] && closing:
			if depth > 0 {
				depth--

				boundaryChanged(i)
			}
		case outputContextTags[name] && !selfClosing:
			depth++

			boundaryChanged(tagEnd)
		case name == "cffunction" && closing:
			if fnOut {
				fnOut = false

				boundaryChanged(i)
			}
		case name == "cffunction" && isTruthy(getAttr(tag, "output")):
			fnOut = true

			boundaryChanged(tagEnd)
		case name == "cfcomponent" && closing:
			if compOut {
				compOut = false

				boundaryChanged(i)
			}
		case name == "cfcomponent" && isTruthy(getAttr(tag, "output")):
			compOut = true

			boundaryChanged(tagEnd)
		}

		pos = tagEnd
	}

	if wasActive && start >= 0 && start < len(content) {
		spans = append(spans, [2]int{start, len(content)})
	}

	return spans, prefixes
}

// skipTagComment returns the offset just past the nested <!--- ---> comment
// opening at i, or the end of content when it is unclosed.
func skipTagComment(content string, i int) int {
	depth := 0

	for j := i; j < len(content); {
		switch {
		case strings.HasPrefix(content[j:], "<!---"):
			depth++
			j += 5
		case strings.HasPrefix(content[j:], "--->"):
			depth--
			j += 4

			if depth == 0 {
				return j
			}
		default:
			j++
		}
	}

	return len(content)
}

func isTagNameByte(c byte) bool {
	return c == '_' || c == ':' || c == '-' || c == '.' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// customPrefixTag reports whether the tag opening at s is a cfimport-prefixed
// custom tag, <prefix:name ...>, for one of prefixes. Its attributes are
// evaluated wherever it stands.
func customPrefixTag(s string, prefixes []string) bool {
	if len(prefixes) == 0 || len(s) < 3 || s[0] != '<' || s[1] == '/' {
		return false
	}

	colon := strings.IndexByte(s[1:min(len(s), 64)], ':')
	if colon <= 0 {
		return false
	}

	prefix := s[1 : 1+colon]
	for _, c := range []byte(prefix) {
		if !isTagNameByte(c) || c == ':' {
			return false
		}
	}

	for _, p := range prefixes {
		if strings.EqualFold(p, prefix) {
			return true
		}
	}

	return false
}

// markupClosingTag finds evidence of HTML: a closing tag, a doctype or an
// HTML comment. A script fragment can hold "a<b", but not "</div>".
var markupClosingTag = regexp.MustCompile(`(?i)</[a-z][a-z0-9-]*\s*>|<!doctype|<!--[^-]`)

// isMarkupTemplate reports whether a .cfm or .cfml file with no CF tags is an
// HTML template rather than CFScript. ColdFusion runs such a file as literal
// text: with no <cfoutput> it evaluates nothing, and reading it as script
// turned "Rich Text (with Images)" into a call to Text.
func isMarkupTemplate(fileURI, content string) bool {
	ext := strings.ToLower(fileURI[strings.LastIndexByte(fileURI, '.')+1:])
	if ext != "cfm" && ext != "cfml" {
		return false
	}

	return !containsCFTag(content) && markupClosingTag.MatchString(content)
}
