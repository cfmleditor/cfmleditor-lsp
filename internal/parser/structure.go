package parser

import "strings"

// SpanKind says what a Span is.
type SpanKind uint8

// The kinds of span StructureSpans reports.
const (
	// SpanComment is a comment. Its End is its last line, which carries
	// content of its own.
	SpanComment SpanKind = iota
	// SpanBlock is a `{ }`: a function or closure body, a control block, a
	// struct literal, the component body, a script-syntax tag's body. Its
	// End is the line holding the `}`.
	SpanBlock
	// SpanList is a multi-line argument list, parameter list or array
	// literal. Its End is the line holding the `)` or `]`.
	SpanList
	// SpanCase is one `case` or `default` of a switch. Its End is the line
	// of its last token.
	SpanCase
)

// Span is a construct spanning more than one line, 0-based and inclusive at
// both ends.
type Span struct {
	Start int
	End   int
	Kind  SpanKind
}

// StructureSpans returns the multi-line constructs of a document, for
// folding: every comment, every `{ }` in CFScript, every argument list,
// parameter list and array literal in CFScript, and every switch case.
//
// Which syntax a stretch of the file is in comes from ClassifyRegions, the same
// split the parse makes. A script region is read with the parse's own scanner
// in CFScript mode, so strings — including one inside a `#...#` inside a
// string — are stepped over exactly as the parse steps over them. A tag region
// is searched for comments only, for now: `/*` in markup is CSS or prose, and
// pairing its apostrophes as quotes would be wrong. A RegionSkip is a literal
// <script> block of JavaScript, whose comments and braces are not CFML's.
func StructureSpans(content string) []Span {
	regions, idx := ClassifyRegionsIdx(content)
	if idx == nil {
		idx = buildLineIdx(content)
	}

	var spans []Span

	for i := range regions {
		r := &regions[i]

		switch r.Kind {
		case RegionScript:
			spans = scriptStructure(r.Text, r.StartLine, spans)
		case RegionTag:
			tagComments(r.Text, func(from, to int) {
				start, end := lineAtOffset(idx, r.Offset+from), lineAtOffset(idx, r.Offset+to)
				if end > start {
					spans = append(spans, Span{Start: start, End: end, Kind: SpanComment})
				}
			})
		case RegionSkip:
		}
	}

	return spans
}

// structFrame is one open bracket in scriptStructure's stack.
type structFrame struct {
	// chain carries an `if`/`else` or `try`/`catch` chain into the block of
	// its next branch; chain.blocks is zero for a block that continues none.
	chain structChain
	// start is the line the construct's fold starts on: the first line of
	// the statement or argument the bracket belongs to. line is the line of
	// the bracket itself.
	start, line int
	// caseStart is the line of the open case in a switch body, or -1.
	caseStart int
	// outerStart, outerTokens, outerFirst and outerAttrs are the enclosing
	// statement's state, restored when a `(` or `[` closes and the statement
	// goes on.
	outerStart  int
	outerTokens int
	outerFirst  wordClass
	open        TokenKind
	outerAttrs  bool
	// fold is false for a bracket whose span is not reported: a grouping or
	// condition parenthesis, an index.
	fold bool
	// switchBody marks the `{` of a switch, inside which `case` and
	// `default` open a SpanCase.
	switchBody bool
}

// structChain is an `if`/`else` or `try`/`catch`/`finally` chain: where it
// started, where its last block so far ends, how many blocks it has had, the
// stack depth its blocks sit at, and where each `else` branch started.
type structChain struct {
	elses                     []int
	start, end, blocks, depth int
}

// structScan is scriptStructure's state between tokens.
type structScan struct {
	spans []Span
	stack []structFrame
	// pending is a chain whose last block just closed, and carry one the
	// next branch's `else`, `catch` or `finally` has continued, waiting for
	// that branch's `{`. Held by value, each valid when its blocks is not
	// zero, so closing a block allocates nothing.
	pending, carry structChain
	// stmtStart is the first line of the statement, argument or element
	// being read, and stmtTokens how many tokens it has had.
	stmtStart  int
	stmtTokens int
	// lastLine is the line of the last significant token, where an open case
	// ends when the next one begins.
	lastLine int
	// tentLine and tentPrevLine are the line of a word that began a new line
	// inside an attribute statement and the line of the token before it.
	tentLine, tentPrevLine int
	// stmtFirst is the class of the statement's first word, prevClass that
	// of the last significant token when it was a word, and tentClass that
	// of the tentative word.
	stmtFirst, prevClass, tentClass wordClass
	// prevKind is the last significant token's kind, which with prevClass
	// tells a call's `(` from a condition's and an array's `[` from an index.
	prevKind TokenKind
	// atStart is set where a statement ends — after `;`, `{`, `}`, `(`, `[`,
	// `,` and a case's `:` — so the next token begins the next one.
	atStart bool
	// attrs marks a statement that is a run of attributes — `component
	// extends="x"`, a script-syntax tag such as `lock name="x"` — where a
	// line break does not end the statement, since each attribute may sit
	// on a line of its own.
	attrs bool
	// tentative is set for a word that began a new line inside an attribute
	// statement. It is another attribute if `=` follows it; the next token
	// says.
	tentative bool
	// caseColon is set by `case` and `default` in a switch body until their
	// `:`, which begins the case's first statement.
	caseColon bool
}

// scriptStructure appends the spans of one CFScript region, whose first line
// is baseLine, to spans.
func scriptStructure(text string, baseLine int, spans []Span) []Span {
	sc := NewScanner(text)
	sc.interpStrings = true

	st := structScan{spans: spans, atStart: true}

	for {
		tok := sc.next()

		switch tok.Kind {
		case TokEOF:
			st.flushChain()

			return st.spans
		case TokNewline, TokLineComment:
			continue
		case TokBlockComment, TokCFComment:
			line := baseLine + tok.Line
			if end := line + strings.Count(tok.Value, "\n"); end > line {
				st.spans = append(st.spans, Span{Start: line, End: end, Kind: SpanComment})
			}

			continue
		default:
		}

		st.token(tok, baseLine+tok.Line)
	}
}

// token advances the scan over one significant token on line.
func (st *structScan) token(tok Token, line int) {
	var class wordClass
	if tok.Kind == TokIdent {
		class = classify(tok.Value)
	}

	if st.pending.blocks > 0 {
		if class&wordChain != 0 {
			st.carry, st.pending = st.pending, structChain{}
		} else {
			st.flushChain()
		}
	}

	if st.tentative {
		st.settleTentative(tok.Kind, line)
	}

	if !st.atStart && st.endsStatementByNewline(tok.Kind, class, line) {
		st.atStart = true
	}

	switch {
	case st.atStart && !isCloser(tok.Kind):
		st.stmtStart, st.stmtFirst, st.atStart, st.stmtTokens = line, class, false, 0
		st.attrs = class&wordAttrs != 0
	case st.stmtTokens == 1 && tok.Kind == TokIdent && st.prevKind == TokIdent && startsAttributes(st.stmtFirst, class):
		// `word word`: a script-syntax tag and its first attribute.
		st.attrs = true
	default:
	}

	st.stmtTokens++

	switch tok.Kind {
	case TokLBrace, TokLParen, TokLBracket:
		st.push(tok.Kind, line)
	case TokRBrace, TokRParen, TokRBracket:
		st.pop(tok.Kind, line)
	case TokSemicolon:
		st.endAttrStatement(line)
		st.atStart = true
	case TokComma:
		st.atStart = true
	case TokIdent:
		if class&wordCase != 0 {
			st.caseLabel(line)
		}
	case TokColon:
		if st.caseColon {
			st.caseColon, st.atStart = false, true
		}
	default:
	}

	st.prevKind, st.prevClass, st.lastLine = tok.Kind, class, line
}

// endsStatementByNewline reports whether a token of kind and class, on a new
// line, begins a new statement although no `;` ended the last one. CFScript
// does not require the semicolon — cfwheels and Lucee's script-syntax tags
// leave it out throughout — and without this every fold in such code started
// on some earlier statement's line. Inside a `(` or `[` a newline ends
// nothing; in a block it ends the statement when the last token could end an
// expression and this one is a word that is not an operator.
func (st *structScan) endsStatementByNewline(kind TokenKind, class wordClass, line int) bool {
	if line <= st.lastLine || kind != TokIdent || class&wordOperator != 0 {
		return false
	}

	if n := len(st.stack); n > 0 && st.stack[n-1].open != TokLBrace {
		return false
	}

	if st.attrs {
		// Another attribute or a new statement: the next token decides.
		st.tentative, st.tentLine, st.tentClass, st.tentPrevLine = true, line, class, st.lastLine

		return false
	}

	// A lone word is not a statement, bar the few that stand alone: `admin`
	// on a line of its own is a script-syntax tag whose attributes follow.
	if st.stmtTokens == 1 && st.prevKind == TokIdent && st.prevClass&wordAlone == 0 {
		return false
	}

	switch st.prevKind {
	case TokString, TokNumber, TokRParen, TokRBracket:
		return true
	case TokIdent:
		return st.prevClass&(wordOperator|wordLeading) == 0
	default:
		return false
	}
}

// settleTentative decides what the word before a token of kind, on line, was.
// Followed by `=` it was another attribute, keyword or not: `default="x"` is
// the commonest property attribute there is. Otherwise a keyword — `property`,
// `function`, `if` — began a new statement, and any other word was a valueless
// attribute — `singleton`, `threadsafe` — when the body's `{` or the next line
// follows, or a new statement, a script-syntax tag after one with no
// semicolon, when something else on its own line does. A new statement ends
// the attribute statement before it on the line before.
func (st *structScan) settleTentative(kind TokenKind, line int) {
	st.tentative = false

	if kind == TokEquals {
		return
	}

	keyword := st.tentClass != 0 && st.tentClass&wordOperator == 0
	if !keyword && (kind == TokLBrace || line > st.tentLine) {
		return
	}

	st.endAttrStatement(st.tentPrevLine)

	st.stmtStart, st.stmtFirst, st.stmtTokens = st.tentLine, st.tentClass, 1
	st.attrs = st.tentClass&wordAttrs != 0
}

// endAttrStatement folds an attribute statement that spans lines and ends
// without a block — `admin action="x"` with each attribute on a line of its
// own, ending in `;` on end — as tree-sitter folded a tag statement. One that
// ends in `{` is folded as that block instead.
func (st *structScan) endAttrStatement(end int) {
	if st.attrs && end > st.stmtStart {
		st.spans = append(st.spans, Span{Start: st.stmtStart, End: end, Kind: SpanBlock})
	}

	st.attrs = false
}

// wordClass is what a word means to the structure scan, as a set of flags.
type wordClass uint16

const (
	// wordOperator is one of CFML's word operators, which continue an
	// expression across a line break.
	wordOperator wordClass = 1 << iota
	// wordLeading begins a statement that goes on past it, so a line break
	// after it does not end the statement.
	wordLeading
	// wordGrouping is followed by a condition or a grouped expression
	// rather than an argument list.
	wordGrouping
	// wordAlone is a whole statement by itself.
	wordAlone
	// wordChain continues an `if` or `try` after its block.
	wordChain
	// wordElse is `else`, whose branches fold to the end of their chain.
	wordElse
	// wordAttrs begins a statement that is a run of attributes.
	wordAttrs
	// wordSwitch opens a switch, and wordCase labels one of its cases.
	wordSwitch
	wordCase
	// wordReturn is `return`, after which a `[` is an array, not an index.
	wordReturn
)

// classify says what word means to the structure scan: one switch on the
// word lowercased without allocating, rather than a list compared with
// EqualFold word by word, which was a fifth of the scan.
func classify(word string) wordClass {
	if len(word) > len("component") {
		return 0
	}

	var buf foldScratch

	switch string(buf.lowerFold(word)) {
	case "and", "or", "not", "xor", "eqv", "imp", "eq", "neq", "gt", "lt", "gte", "lte",
		"ge", "le", "contains", "mod", "equal", "less", "greater", "than", "does":
		return wordOperator
	case "is", "in":
		return wordOperator | wordGrouping
	case "return":
		return wordLeading | wordGrouping | wordAlone | wordReturn
	case "var", "new", "throw", "function", "final", "static", "public", "private",
		"package", "remote", "import":
		return wordLeading
	case "else":
		return wordLeading | wordChain | wordElse
	case "catch":
		return wordChain | wordGrouping
	case "finally":
		return wordChain
	case "if", "while", "for":
		return wordGrouping
	case "switch":
		return wordGrouping | wordSwitch
	case "break", "continue", "abort", "exit", "retry", "rethrow":
		return wordAlone
	case "component", "interface", "property":
		return wordAttrs
	case "case", "default":
		return wordCase
	default:
		return 0
	}
}

// startsAttributes reports whether a statement whose first two words have
// classes first and second is a script-syntax tag and its first attribute, as
// `lock name` and `admin action` are, rather than a keyword and what it
// governs, as `var x` and `else if` are.
func startsAttributes(first, second wordClass) bool {
	return first&(wordLeading|wordOperator|wordCase) == 0 && second&wordOperator == 0
}

// flushChain reports a pending chain that had more than one block — the
// whole `if … else …` or `try … catch …`, which tree-sitter folded as one
// statement as well as branch by branch.
//
// An `else` branch that is not the last folds to the end of the chain too:
// `else if (…) {…} else {…}` is one statement nested in the first, which is
// how tree-sitter folded it. A last `else` gives its own block's range again,
// which the server's dedupe collapses.
func (st *structScan) flushChain() {
	c := &st.pending
	if c.blocks > 1 && c.end > c.start {
		st.spans = append(st.spans, Span{Start: c.start, End: c.end, Kind: SpanBlock})

		for _, e := range c.elses {
			if c.end > e {
				st.spans = append(st.spans, Span{Start: e, End: c.end, Kind: SpanBlock})
			}
		}
	}

	st.pending = structChain{}
}

func isCloser(k TokenKind) bool {
	return k == TokRBrace || k == TokRParen || k == TokRBracket
}

// push opens a bracket. Its fold starts where its statement does, so `if (a
// &&\n b) {` folds from the `if` and `var cfg = {` from the `var`.
func (st *structScan) push(open TokenKind, line int) {
	f := structFrame{
		open:        open,
		start:       st.stmtStart,
		line:        line,
		caseStart:   -1,
		outerStart:  st.stmtStart,
		outerFirst:  st.stmtFirst,
		outerTokens: st.stmtTokens,
		outerAttrs:  st.attrs,
	}

	switch open {
	case TokLBrace:
		f.fold = true
		f.switchBody = st.stmtFirst&wordSwitch != 0

		if st.carry.blocks > 0 && st.carry.depth == len(st.stack) {
			f.chain, st.carry = st.carry, structChain{}
			if st.stmtFirst&wordElse != 0 {
				f.chain.elses = append(f.chain.elses, f.start)
			}
		}
	case TokLParen:
		f.fold = st.opensArgumentList()
	case TokLBracket:
		f.fold = st.opensArrayLiteral()
	default:
	}

	st.stack = append(st.stack, f)
	st.atStart = true
}

// opensArgumentList reports whether a `(` holds arguments or parameters —
// `f(`, `obj.m(`, `function(`, `f()(` — rather than grouping an expression or
// a condition: `if (`, `while (`, `= (`, `&& (`. A multi-line condition or
// grouped expression is not worth a fold of its own.
func (st *structScan) opensArgumentList() bool {
	switch st.prevKind {
	case TokRParen, TokRBracket:
		return true
	case TokIdent:
		return st.prevClass&(wordGrouping|wordOperator) == 0
	default:
		return false
	}
}

// opensArrayLiteral reports whether a `[` begins an array rather than an
// index: after a word or a closing bracket it indexes, unless the word is
// `return`.
func (st *structScan) opensArrayLiteral() bool {
	switch st.prevKind {
	case TokIdent:
		return st.prevClass&(wordReturn|wordOperator) != 0
	case TokRParen, TokRBracket, TokString, TokNumber:
		return false
	default:
		return true
	}
}

// pop closes the innermost bracket of kind closing, discarding any left open
// above it — which is how an unbalanced document mid-edit degrades to fewer
// spans rather than wrong ones. A closer with nothing to match is ignored.
func (st *structScan) pop(closing TokenKind, line int) {
	var open TokenKind

	switch closing {
	case TokRBrace:
		open = TokLBrace
	case TokRParen:
		open = TokLParen
	default:
		open = TokLBracket
	}

	i := len(st.stack) - 1
	for i >= 0 && st.stack[i].open != open {
		i--
	}

	if i < 0 {
		return
	}

	f := st.stack[i]
	st.stack = st.stack[:i]

	if f.switchBody {
		st.closeCase(&f)
	}

	// Folded from the statement's first line whenever the statement spans
	// lines, even if the bracket does not: a chain written a call per line —
	// `table` then `.integer( "order" )` then `.default( 0 )` — has its fold
	// from the last call's `)`.
	if f.fold && line > f.start {
		kind := SpanBlock
		if open != TokLBrace {
			kind = SpanList
		}

		st.spans = append(st.spans, Span{Start: f.start, End: line, Kind: kind})

		// A bracket that is not on its statement's first line — the
		// parameters or the condition wrapped, or a call sits further down a
		// chain — folds from its own line as well, so the body or the
		// arguments can be folded with what leads up to them left on screen.
		if f.line > f.start && line > f.line {
			st.spans = append(st.spans, Span{Start: f.line, End: line, Kind: kind})
		}
	}

	if open == TokLBrace {
		// A block ends a statement, so what follows begins one — `} else {`
		// folds its else branch from the `else`. Whether that token goes on
		// with an `if` or `try` chain is for the next token to say.
		st.atStart = true

		st.pending = structChain{start: f.start, end: line, blocks: 1, depth: len(st.stack)}
		if f.chain.blocks > 0 {
			st.pending.start, st.pending.blocks, st.pending.elses = f.chain.start, f.chain.blocks+1, f.chain.elses
		}

		return
	}

	// A list is part of the statement around it, which goes on.
	st.stmtStart, st.stmtFirst, st.atStart = f.outerStart, f.outerFirst, false
	st.stmtTokens, st.attrs = f.outerTokens, f.outerAttrs
}

// caseLabel opens a case, on line, when the innermost bracket is a switch
// body, closing the one before it.
func (st *structScan) caseLabel(line int) {
	if len(st.stack) == 0 {
		return
	}

	f := &st.stack[len(st.stack)-1]
	if !f.switchBody {
		return
	}

	st.closeCase(f)
	f.caseStart = line
	st.caseColon = true
}

// closeCase ends f's open case at the last token before the next label or
// the closing `}`.
func (st *structScan) closeCase(f *structFrame) {
	if f.caseStart >= 0 && st.lastLine > f.caseStart {
		st.spans = append(st.spans, Span{Start: f.caseStart, End: st.lastLine, Kind: SpanCase})
	}

	f.caseStart = -1
}

// tagComments reports each `<!--- --->` and HTML `<!-- -->` in tag-syntax
// text, as the byte offsets of its first and last characters. CFML comments
// nest, and neither kind is string-aware — an engine ends one at the first
// closing delimiter whatever quotes precede it, and so does a browser — so
// neither is this.
func tagComments(text string, add func(from, to int)) {
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

		add(start, end-1)
		i = end
	}
}
