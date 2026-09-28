package parser

import "strings"

// LineSpan is a run of lines, 0-based and inclusive at both ends.
type LineSpan struct {
	Start int
	End   int
}

// CommentSpans returns the lines each comment spanning more than one line
// covers, in document order: a `/* */` in CFScript and a `<!--- --->` in tag
// syntax, and an HTML `<!-- -->` in markup. A comment on one line is left
// out, since it has nothing to fold.
//
// Which syntax a stretch of the file is in comes from ClassifyRegions, the same
// split the parse makes. Neither kind of comment is looked for in the other's
// region: `/*` in markup is CSS or prose, and the scan of a script region has
// to track strings to tell `"src/*.cfc"` from a comment, which in markup would
// pair the apostrophes of ordinary text. A RegionSkip is a literal <script>
// block of JavaScript, whose comments are not CFML's.
func CommentSpans(content string) []LineSpan {
	regions, idx := ClassifyRegionsIdx(content)
	if idx == nil {
		idx = buildLineIdx(content)
	}

	var spans []LineSpan

	add := func(from, to int) {
		start, end := lineAtOffset(idx, from), lineAtOffset(idx, to)
		if end > start {
			spans = append(spans, LineSpan{Start: start, End: end})
		}
	}

	for i := range regions {
		r := &regions[i]

		switch r.Kind {
		case RegionScript:
			scriptComments(r.Text, r.Offset, add)
		case RegionTag:
			tagComments(r.Text, r.Offset, add)
		case RegionSkip:
		}
	}

	return spans
}

// scriptComments reports each comment in CFScript text as the byte offsets of
// its first and last characters plus base. It reads the text with the parse's
// own scanner in CFScript mode, so a string is stepped over exactly as the
// parse steps over it — including one inside a `#...#` inside a string, as in
// `"#listFirst(s, "/")#/*"`, which a plain quote-to-quote scan pairs wrongly:
// it then reads that `/*` as a comment, or every later comment as code.
func scriptComments(text string, base int, add func(from, to int)) {
	sc := NewScanner(text)
	sc.interpStrings = true

	for {
		tok := sc.next()

		switch tok.Kind {
		case TokEOF:
			return
		case TokBlockComment, TokCFComment:
			add(base+tok.Offset, base+tok.Offset+len(tok.Value)-1)
		default:
		}
	}
}

// tagComments reports each `<!--- --->` and HTML `<!-- -->` in tag-syntax
// text. CFML comments nest, and neither kind is string-aware — an engine ends
// one at the first closing delimiter whatever quotes precede it, and so does a
// browser — so neither is this.
func tagComments(text string, base int, add func(from, to int)) {
	for i := 0; i < len(text); {
		k := strings.Index(text[i:], "<!--")
		if k < 0 {
			return
		}

		start := i + k

		var end int

		if strings.HasPrefix(text[start:], "<!---") {
			end = skipCFMLComment(text, start)
		} else if c := strings.Index(text[start+4:], "-->"); c >= 0 {
			end = start + 4 + c + 3
		} else {
			end = len(text)
		}

		add(base+start, base+end-1)
		i = end
	}
}
