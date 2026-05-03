package cfml

import (
	"regexp"
	"sort"
	"strings"

	"go.lsp.dev/uri"
)

// Argument represents a parameter of a user-defined function.
type Argument struct {
	Name     string
	Type     string // empty if untyped
	Required bool
}

// FunctionDef represents a user-defined function found in a CFC file.
type FunctionDef struct {
	Name      string
	URI       uri.URI
	Line      uint32
	Arguments []Argument
}

// RegionKind classifies a span of CFC content.
type RegionKind int

const (
	RegionScript RegionKind = iota
	RegionTag
)

// Region is a contiguous span of source with a single kind.
// Text is the original content (not stripped).
type Region struct {
	Kind      RegionKind
	StartLine int
	Text      string
}

// span represents a byte range [Start, End) in the source.
type span struct{ Start, End int }

// Tag-based: <cffunction name="myFunc"
var tagFuncRe = regexp.MustCompile(`(?i)<cffunction\s[^>]*\bname\s*=\s*["']([^"']+)["']`)

// Script-based: access? returntype? function name(
var scriptFuncRe = regexp.MustCompile(`(?im)(?:(?:public|private|remote|package)\s+)?(?:\w+\s+)?function\s+(\w+)\s*\(`)

// Tag-based argument: <cfargument name="x" type="string" required="true">
var tagArgRe = regexp.MustCompile(`(?i)<cfargument\s[^>]*>`)
var tagArgNameRe = regexp.MustCompile(`(?i)\bname\s*=\s*["']([^"']+)["']`)
var tagArgTypeRe = regexp.MustCompile(`(?i)\btype\s*=\s*["']([^"']+)["']`)
var tagArgRequiredRe = regexp.MustCompile(`(?i)\brequired\s*=\s*["'](true|yes)["']`)

// Script argument: [required] [type] name [= default]
var scriptArgRe = regexp.MustCompile(`(?i)(?:(required)\s+)?(?:(\w+)\s+)?(\w+)\s*(?:=\s*[^,)]+)?`)

// ParseFunctionDefs extracts function definitions from CFC content using
// region-aware parsing. Comments are identified and matches inside them
// are excluded, preserving original positions.
func ParseFunctionDefs(fileURI uri.URI, content string) []FunctionDef {
	comments := findCommentSpans(content)
	regions := classifyRegions(content, comments)
	var defs []FunctionDef

	for _, r := range regions {
		var re *regexp.Regexp
		if r.Kind == RegionScript {
			re = scriptFuncRe
		} else {
			re = tagFuncRe
		}
		regionOffset := byteOffsetOfLine(content, r.StartLine)
		for _, idx := range re.FindAllStringSubmatchIndex(r.Text, -1) {
			matchStart := regionOffset + idx[0]
			if inComment(matchStart, comments) {
				continue
			}
			name := r.Text[idx[2]:idx[3]]
			line := r.StartLine + strings.Count(r.Text[:idx[0]], "\n")

			var args []Argument
			if r.Kind == RegionScript {
				args = parseScriptArgs(r.Text, idx[1]-1, comments, regionOffset)
			} else {
				args = parseTagArgs(r.Text, idx[1], comments, regionOffset)
			}

			defs = append(defs, FunctionDef{
				Name:      name,
				URI:       fileURI,
				Line:      uint32(line),
				Arguments: args,
			})
		}
	}
	return defs
}

// parseScriptArgs extracts arguments from the parenthesized list after a
// script function name. parenStart is the index of '(' in regionText.
func parseScriptArgs(regionText string, parenStart int, comments []span, regionOffset int) []Argument {
	if parenStart >= len(regionText) || regionText[parenStart] != '(' {
		return nil
	}
	// Find matching close paren, respecting nesting
	depth := 1
	i := parenStart + 1
	for i < len(regionText) && depth > 0 {
		switch regionText[i] {
		case '(':
			depth++
		case ')':
			depth--
		case '"', '\'':
			q := regionText[i]
			i++
			for i < len(regionText) && regionText[i] != q {
				i++
			}
		}
		if depth > 0 {
			i++
		}
	}
	argText := regionText[parenStart+1 : i]
	if strings.TrimSpace(argText) == "" {
		return nil
	}

	var args []Argument
	for _, part := range strings.Split(argText, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// Check if this arg's position is inside a comment
		partOffset := regionOffset + parenStart + 1 + strings.Index(argText, part)
		if inComment(partOffset, comments) {
			continue
		}
		m := scriptArgRe.FindStringSubmatch(part)
		if m == nil {
			continue
		}
		req := strings.EqualFold(m[1], "required")
		typ := m[2]
		name := m[3]
		// If only one token matched (no type), it's just the name
		args = append(args, Argument{Name: name, Type: typ, Required: req})
	}
	return args
}

// parseTagArgs finds <cfargument> tags following a <cffunction> match in tag regions.
func parseTagArgs(regionText string, afterFunc int, comments []span, regionOffset int) []Argument {
	// Scan from afterFunc until we hit </cffunction> or another <cffunction
	rest := regionText[afterFunc:]
	lowerRest := strings.ToLower(rest)

	// Find the boundary — next </cffunction> or <cffunction
	end := len(rest)
	if idx := strings.Index(lowerRest, "</cffunction"); idx >= 0 && idx < end {
		end = idx
	}
	if idx := strings.Index(lowerRest[1:], "<cffunction"); idx >= 0 && idx+1 < end {
		end = idx + 1
	}
	block := rest[:end]

	var args []Argument
	for _, m := range tagArgRe.FindAllStringIndex(block, -1) {
		tagOffset := regionOffset + afterFunc + m[0]
		if inComment(tagOffset, comments) {
			continue
		}
		tag := block[m[0]:m[1]]
		nameMatch := tagArgNameRe.FindStringSubmatch(tag)
		if nameMatch == nil {
			continue
		}
		a := Argument{Name: nameMatch[1]}
		if typeMatch := tagArgTypeRe.FindStringSubmatch(tag); typeMatch != nil {
			a.Type = typeMatch[1]
		}
		if reqMatch := tagArgRequiredRe.FindStringSubmatch(tag); reqMatch != nil {
			a.Required = true
		}
		args = append(args, a)
	}
	return args
}

// ClassifyRegions segments CFC content into script and tag regions.
// Exported for testing.
func ClassifyRegions(content string) []Region {
	return classifyRegions(content, findCommentSpans(content))
}

func classifyRegions(content string, comments []span) []Region {
	if isScriptFile(content, comments) {
		return []Region{{Kind: RegionScript, StartLine: 0, Text: content}}
	}
	return splitCFScriptBlocks(content)
}

// findCommentSpans returns sorted, non-overlapping byte ranges of all comments.
func findCommentSpans(s string) []span {
	var spans []span
	n := len(s)
	i := 0
	for i < n {
		// String literals — skip to avoid false matches
		if s[i] == '"' || s[i] == '\'' {
			q := s[i]
			i++
			for i < n && s[i] != q {
				if s[i] == '\\' && i+1 < n {
					i++
				}
				i++
			}
			if i < n {
				i++
			}
			continue
		}
		// CFML comment: <!--- ... ---> (supports nesting)
		if i+4 < n && s[i] == '<' && s[i+1] == '!' && s[i+2] == '-' && s[i+3] == '-' && s[i+4] == '-' {
			start := i
			depth := 1
			i += 5
			for i < n && depth > 0 {
				if i+4 < n && s[i] == '<' && s[i+1] == '!' && s[i+2] == '-' && s[i+3] == '-' && s[i+4] == '-' {
					depth++
					i += 5
					continue
				}
				if i+2 < n && s[i] == '-' && s[i+1] == '-' && s[i+2] == '>' {
					depth--
					i += 3
					continue
				}
				i++
			}
			spans = append(spans, span{start, i})
			continue
		}
		// Block comment: /* ... */
		if i+1 < n && s[i] == '/' && s[i+1] == '*' {
			start := i
			i += 2
			for i < n {
				if i+1 < n && s[i] == '*' && s[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
			spans = append(spans, span{start, i})
			continue
		}
		// Line comment: //
		if i+1 < n && s[i] == '/' && s[i+1] == '/' {
			start := i
			for i < n && s[i] != '\n' {
				i++
			}
			spans = append(spans, span{start, i})
			continue
		}
		i++
	}
	return spans
}

// inComment returns true if byteOffset falls inside any comment span.
func inComment(byteOffset int, comments []span) bool {
	idx := sort.Search(len(comments), func(i int) bool {
		return comments[i].End > byteOffset
	})
	return idx < len(comments) && comments[idx].Start <= byteOffset
}

// isScriptFile checks whether the first non-whitespace, non-comment token
// indicates a script-based CFC.
func isScriptFile(content string, comments []span) bool {
	i := 0
	n := len(content)
	for i < n {
		// Skip whitespace
		if content[i] == ' ' || content[i] == '\t' || content[i] == '\r' || content[i] == '\n' {
			i++
			continue
		}
		// Skip if inside a comment
		if ci := sort.Search(len(comments), func(j int) bool {
			return comments[j].End > i
		}); ci < len(comments) && comments[ci].Start <= i {
			i = comments[ci].End
			continue
		}
		break
	}
	rest := content[i:]
	return strings.HasPrefix(rest, "component") ||
		strings.HasPrefix(rest, "interface") ||
		strings.HasPrefix(rest, "property ") ||
		strings.HasPrefix(rest, "property\t")
}

// splitCFScriptBlocks splits tag-based content into tag and script regions.
func splitCFScriptBlocks(content string) []Region {
	lower := strings.ToLower(content)
	var regions []Region
	pos := 0

	for {
		openIdx := strings.Index(lower[pos:], "<cfscript>")
		if openIdx < 0 {
			break
		}
		openIdx += pos
		if openIdx > pos {
			regions = appendRegion(regions, RegionTag, content[pos:openIdx], countLines(content, 0, pos))
		}
		bodyStart := openIdx + len("<cfscript>")
		closeIdx := strings.Index(lower[bodyStart:], "</cfscript>")
		if closeIdx < 0 {
			regions = appendRegion(regions, RegionScript, content[bodyStart:], countLines(content, 0, bodyStart))
			pos = len(content)
			break
		}
		closeIdx += bodyStart
		regions = appendRegion(regions, RegionScript, content[bodyStart:closeIdx], countLines(content, 0, bodyStart))
		pos = closeIdx + len("</cfscript>")
	}
	if pos < len(content) {
		regions = appendRegion(regions, RegionTag, content[pos:], countLines(content, 0, pos))
	}
	return regions
}

func appendRegion(regions []Region, kind RegionKind, text string, startLine int) []Region {
	if strings.TrimSpace(text) == "" {
		return regions
	}
	return append(regions, Region{Kind: kind, StartLine: startLine, Text: text})
}

// countLines counts newlines in content[from:to].
func countLines(content string, from, to int) int {
	n := 0
	for i := from; i < to; i++ {
		if content[i] == '\n' {
			n++
		}
	}
	return n
}

// byteOffsetOfLine returns the byte offset of the start of the given line number.
func byteOffsetOfLine(content string, line int) int {
	off := 0
	for l := 0; l < line; l++ {
		idx := strings.IndexByte(content[off:], '\n')
		if idx < 0 {
			return len(content)
		}
		off += idx + 1
	}
	return off
}
