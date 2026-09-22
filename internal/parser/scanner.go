// Package parser provides a hand-written recursive descent parser for CFML
// and CFScript that extracts function definitions, variable declarations, and
// component references.
package parser

import "strings"

// TokenKind classifies a lexical token.
type TokenKind int

// TokenKind values produced by the scanner.
const (
	TokEOF TokenKind = iota
	TokIdent
	TokString       // "..." or '...'
	TokNumber       // 123, 1.5
	TokLParen       // (
	TokRParen       // )
	TokLBrace       // {
	TokRBrace       // }
	TokLBracket     // [
	TokRBracket     // ]
	TokDot          // .
	TokComma        // ,
	TokSemicolon    // ;
	TokEquals       // =
	TokLT           // <
	TokGT           // >
	TokSlash        // /
	TokHash         // #
	TokColon        // :
	TokQuestion     // ?
	TokBang         // !
	TokAmpersand    // &
	TokPipe         // |
	TokPlus         // +
	TokMinus        // -
	TokStar         // *
	TokPercent      // %
	TokCaret        // ^
	TokAt           // @
	TokNewline      // \n (tracked for line counting)
	TokCFComment    // <!--- ... --->
	TokBlockComment // /* ... */
	TokLineComment  // // ...
	TokOther        // anything else
	TokDoubleColon  // :: (static member access)
)

// Token is a lexical token with position info.
type Token struct {
	Kind   TokenKind
	Value  string
	Offset int // byte offset in source
	Line   int // 0-based line number
}

// Scanner tokenizes CFML/CFScript source.
type Scanner struct {
	src string
	// interpStrings makes a `#` inside a string open an expression, in which a
	// string may be opened with the same quote character. Off by default and
	// turned on only where the text is known to be CFScript — see scanString.
	interpStrings    bool
	pos              int
	line             int
	LastBlockComment string // most recent /** ... */ or /* ... */ comment value

	// One token of lookahead, filled by PeekSkipComments and consumed by
	// NextSkipComments. The parsers reach the scanner through those two and
	// nothing else, and they overwhelmingly peek at a token and then take it:
	// 105 peek sites against 165 next sites, most of them paired. Without this
	// the pair tokenises the same bytes twice and skips the same comments and
	// newlines twice, which made the two of them 57% of a parse.
	//
	// peekComment records what the peeked scan did to LastBlockComment rather
	// than just its value, because a peek's effect on that field has always
	// outlived the peek — PeekSkipComments restores pos and line and leaves
	// LastBlockComment set — and parseFunction reads and clears the field
	// between a peek and the matching next. Replaying the assignment only when
	// the scan actually made one keeps both orderings answering as before: a
	// scan that crossed a block comment re-sets the field over the clear, and
	// one that crossed none leaves whatever the clear left.
	peeked         bool
	peekTok        Token
	peekPos        int
	peekLine       int
	peekComment    string
	peekSetComment bool
	commentSeq     uint32
}

// NewScanner creates a scanner for the given source.
func NewScanner(src string) *Scanner {
	// A UTF-8 BOM is an encoding marker, not a token. It is invisible in an
	// editor and 561 of the 5,629 corpus files carry one, so a scanner that
	// stops at it answers "the first token is not `component`" — which is what
	// ClassifyRegions asks to decide whether a `.cfc` is script or tag syntax.
	// Such a file then went to the tag splitter, and one that mentions
	// `<script>` inside a string literal came apart into regions parsed from
	// the middle of an expression: 800 lines of ColdBox's HTMLHelperSpec with
	// no function and almost no call found.
	sc := &Scanner{src: src}
	if strings.HasPrefix(src, bomUTF8) {
		sc.pos = len(bomUTF8)
	}

	return sc
}

// bomUTF8 is the UTF-8 byte order mark.
const bomUTF8 = "\xef\xbb\xbf"

// Pos returns the current byte offset.
func (s *Scanner) Pos() int { return s.pos }

// Line returns the current line number.
func (s *Scanner) Line() int { return s.line }

// ScannerState holds saved scanner position for backtracking.
type ScannerState struct {
	pos  int
	line int
}

// Save returns the current scanner state for later restoration. A pending
// lookahead needs no saving: PeekSkipComments leaves pos and line where they
// were, so the state recorded here is the one in front of the peeked token.
func (s *Scanner) Save() ScannerState { return ScannerState{pos: s.pos, line: s.line} }

// Restore resets the scanner to a previously saved state, dropping any cached
// lookahead — it describes a position this may be moving away from.
func (s *Scanner) Restore(st ScannerState) { s.pos = st.pos; s.line = st.line; s.peeked = false }

// Peek returns the next raw token, comments and newlines included, without
// advancing.
func (s *Scanner) Peek() Token {
	pos, line := s.pos, s.line
	tok := s.Next()
	s.pos, s.line = pos, line

	return tok
}

// Next returns the next raw token and advances the scanner. It discards any
// token PeekSkipComments has cached: that token is the next token past the
// comments, which is not what this returns, and advancing past it here would
// leave the cached position describing bytes already consumed.
func (s *Scanner) Next() Token {
	s.peeked = false

	return s.next()
}

// next is Next without the cache bookkeeping, for the callers inside this file
// that drive the cache themselves.
func (s *Scanner) next() Token {
	s.skipWhitespaceNoNewline()

	if s.pos >= len(s.src) {
		return Token{Kind: TokEOF, Offset: s.pos, Line: s.line}
	}

	start := s.pos
	startLine := s.line
	ch := s.src[s.pos]

	switch {
	case ch == '\n':
		s.pos++
		s.line++

		return Token{Kind: TokNewline, Value: "\n", Offset: start, Line: startLine}

	case ch == '<' && s.pos+4 < len(s.src) && s.src[s.pos:s.pos+5] == "<!---":
		return s.scanCFComment(start, startLine)

	case ch == '/' && s.pos+1 < len(s.src) && s.src[s.pos+1] == '*':
		return s.scanBlockComment(start, startLine)

	case ch == '/' && s.pos+1 < len(s.src) && s.src[s.pos+1] == '/':
		return s.scanLineComment(start, startLine)

	case ch == '"' || ch == '\'':
		return s.scanString(start, startLine)

	case isDigit(ch):
		return s.scanNumber(start, startLine)

	case isIdentStart(ch):
		return s.scanIdent(start, startLine)

	// Safe navigation. Emitting `?.` as a plain dot is what lets every chain
	// walk in the parser see `svc?.save()` as `svc.save()` — the alternative is
	// a TokQuestion arm in each of the dozen places a chain is walked, and a
	// receiver missed in one of them is recorded as a *bare* call, which then
	// resolves as an unqualified function: a wrong answer rather than a missing
	// one.
	//
	// Adjacency is required, so a ternary keeps its question mark: `a ? .5 : 1`
	// is unaffected, and `a ?.5` is not something anyone writes.
	case ch == '?' && s.pos+1 < len(s.src) && s.src[s.pos+1] == '.':
		s.pos += 2

		return Token{Kind: TokDot, Value: ".", Offset: start, Line: startLine}

	// Static member access. Unlike `?.` this is *not* folded into a dot: the
	// qualifier is a component, not a variable holding one, so the call site it
	// produces carries a component rather than a receiver name.
	case ch == ':' && s.pos+1 < len(s.src) && s.src[s.pos+1] == ':':
		s.pos += 2

		return Token{Kind: TokDoubleColon, Value: "::", Offset: start, Line: startLine}

	default:
		s.pos++
		kind := charToKind(ch)

		return Token{Kind: kind, Value: singleByteStrings[ch], Offset: start, Line: startLine}
	}
}

// singleByteStrings holds the one-character string for every byte value, so
// tokenising punctuation reads one instead of building one.
//
// `string(ch)` on a byte allocates, and the default branch above is every
// operator, brace, paren, comma and semicolon in the file: 29.8 million
// allocations on the script benchmark, 42% of every object the parse allocated.
//
// Built with string(rune(i)) rather than from the byte, because that is what the
// conversion it replaces means: for a byte at or above 0x80 the result is the
// two-byte UTF-8 encoding of that code point, not the byte itself. Building it
// the other way would quietly change the token value for every non-ASCII byte
// the scanner does not otherwise recognise.
var singleByteStrings = func() [256]string {
	var table [256]string

	for i := range table {
		table[i] = string(rune(i))
	}

	return table
}()

// NextSkipComments returns the next non-comment, non-newline token, taking the
// one PeekSkipComments already scanned when there is one.
//
// The scanning loop is written out here rather than shared with
// PeekSkipComments through a helper that also reports what it did to
// LastBlockComment. That helper cost more than the cache saved on this path:
// skipPastScope takes tokens without ever peeking, so it always misses, and
// handing back a Token plus a string and a bool copied 64 bytes through an
// extra frame for every token in every function body the parser skips.
func (s *Scanner) NextSkipComments() Token {
	if s.peeked {
		s.peeked = false
		s.pos, s.line = s.peekPos, s.peekLine

		if s.peekSetComment {
			s.LastBlockComment = s.peekComment
		}

		return s.peekTok
	}

	for {
		tok := s.next()
		switch tok.Kind { //nolint:exhaustive
		case TokBlockComment:
			s.LastBlockComment = tok.Value
			s.commentSeq++

			continue
		case TokCFComment, TokLineComment, TokNewline:
			continue
		default:
			return tok
		}
	}
}

// PeekSkipComments peeks at the next non-comment, non-newline token, caching it
// so the matching NextSkipComments does not scan the same bytes again.
func (s *Scanner) PeekSkipComments() Token {
	if s.peeked {
		return s.peekTok
	}

	pos, line := s.pos, s.line
	seq := s.commentSeq
	tok := s.NextSkipComments()

	s.peekTok, s.peekPos, s.peekLine = tok, s.pos, s.line
	// Whether the scan assigned LastBlockComment, not whether the value came
	// out different: two identical block comments in a row would look like no
	// assignment at all, and the replay below would then let an intervening
	// clear stand where the old re-scan overwrote it.
	s.peekComment, s.peekSetComment = s.LastBlockComment, s.commentSeq != seq
	s.peeked = true
	s.pos, s.line = pos, line

	return tok
}

func (s *Scanner) skipWhitespaceNoNewline() {
	for s.pos < len(s.src) {
		ch := s.src[s.pos]
		if ch == ' ' || ch == '\t' || ch == '\r' {
			s.pos++
		} else {
			break
		}
	}
}

func (s *Scanner) scanCFComment(start, startLine int) Token {
	s.pos += 5 // skip <!---
	depth := 1

	for s.pos < len(s.src) && depth > 0 {
		if s.pos+4 < len(s.src) && s.src[s.pos:s.pos+5] == "<!---" {
			depth++
			s.pos += 5

			continue
		}

		if s.pos+2 < len(s.src) && s.src[s.pos] == '-' && s.src[s.pos+1] == '-' && s.src[s.pos+2] == '>' {
			depth--
			s.pos += 3

			continue
		}

		if s.src[s.pos] == '\n' {
			s.line++
		}

		s.pos++
	}

	return Token{Kind: TokCFComment, Value: s.src[start:s.pos], Offset: start, Line: startLine}
}

func (s *Scanner) scanBlockComment(start, startLine int) Token {
	s.pos += 2 // skip /*
	for s.pos < len(s.src) {
		if s.src[s.pos] == '*' && s.pos+1 < len(s.src) && s.src[s.pos+1] == '/' {
			s.pos += 2

			break
		}

		if s.src[s.pos] == '\n' {
			s.line++
		}

		s.pos++
	}

	return Token{Kind: TokBlockComment, Value: s.src[start:s.pos], Offset: start, Line: startLine}
}

func (s *Scanner) scanLineComment(start, startLine int) Token {
	s.pos += 2 // skip //

	// A bare CR ends a line too. Five of the 5,629 corpus files use CR-only
	// endings, and in one of them the first `//` in the file swallowed
	// everything after it — the whole of ColdBox's EventHandler.cfc, which is
	// 2.5KB on what this scanner read as a single line.
	//
	// Only the comment's *end* moves. Line numbers still count `\n` alone,
	// which is what tree-sitter does, so the two keep agreeing about where a
	// call is; changing that is a question about editor positions rather than
	// about parsing, and these files are 0.09% of the corpus.
	for s.pos < len(s.src) && s.src[s.pos] != '\n' && s.src[s.pos] != '\r' {
		s.pos++
	}

	return Token{Kind: TokLineComment, Value: s.src[start:s.pos], Offset: start, Line: startLine}
}

// maxStringNesting bounds the mutual recursion between scanQuoted and
// scanHashExpr. A string holds an expression holds a string, and the nesting is
// driven by the source, so — as with maxArgNesting — Go's inability to recover
// from stack exhaustion is what makes a cap necessary rather than tidy.
const maxStringNesting = 32

func (s *Scanner) scanString(start, startLine int) Token {
	// A `#` inside a string opens an expression, and a string may be opened
	// inside *that*: `"#DayOfWeek("{ts '2000-1-1'}")#"` is one token, and
	// closing it at the inner quote left `"#DayOfWeek("` — so the interpolation
	// had no closing `#` to find and the call in it was invisible — 3,165 sites
	// over 461 files on the corpus, measured in PARSER-GAPS.md §3.3.
	//
	// It is **off unless the text is known to be CFScript**, because text that
	// is not reaches this function: parseFuncBody hands a tag function's raw
	// body to the script parser, and there a `#` opens nothing. Applied to
	// markup the rule is actively wrong —
	//
	//	<a href="#top">x</a><a href="#bot">y</a>
	//
	// pairs the two fragment hashes and swallows the markup between them into
	// one string token. That is a wrong answer where the old behaviour was
	// merely a coarse one, so the callers that know they hold CFScript opt in
	// with asCFScript and everything else keeps the plain scan.
	//
	// The scan is speculative even then: when the nesting does not close before
	// EOF the attempt is abandoned and the plain quote-to-quote scan runs from
	// the same place, so a token can never be worse than it was.
	if s.interpStrings {
		savedPos, savedLine := s.pos, s.line
		if s.scanQuoted(0) {
			return Token{Kind: TokString, Value: s.src[start:s.pos], Offset: start, Line: startLine}
		}

		s.pos, s.line = savedPos, savedLine
	}

	s.scanQuotedPlain()

	return Token{Kind: TokString, Value: s.src[start:s.pos], Offset: start, Line: startLine}
}

// scanQuotedPlain consumes a quoted string, taking the first matching quote that
// is not doubled as the end.
//
// **CFML escapes a quote by doubling it and has no backslash escape at all.**
// The scanner honoured `\\` instead, which is C's rule and not this language's,
// so a string ending in a backslash — a Windows path, a regex class, the
// `listLast( uri, "/\\" )` idiom — did not close where it ends. It closed at the
// *next* quote anywhere in the file, swallowing every call in between: one
// two-character string cost 333 call sites in one corpus component. Doubling is
// checked from inside the string, so a bare `""` is still the empty string and
// `""""` is a string holding one quote.
func (s *Scanner) scanQuotedPlain() {
	q := s.src[s.pos]
	s.pos++

	for s.pos < len(s.src) {
		switch s.src[s.pos] {
		case q:
			if s.pos+1 < len(s.src) && s.src[s.pos+1] == q {
				s.pos += 2

				continue
			}

			s.pos++

			return
		case '\n':
			s.line++
			s.pos++
		default:
			s.pos++
		}
	}
}

// scanQuoted consumes a quoted string, stepping over any #...# expression in it
// rather than through it. Reports whether the string closed.
func (s *Scanner) scanQuoted(depth int) bool {
	if depth >= maxStringNesting {
		return false
	}

	q := s.src[s.pos]
	s.pos++

	for s.pos < len(s.src) {
		switch s.src[s.pos] {
		case q:
			// A doubled quote is CFML's escape; see scanQuotedPlain.
			if s.pos+1 < len(s.src) && s.src[s.pos+1] == q {
				s.pos += 2

				continue
			}

			s.pos++

			return true
		case '#':
			// `##` is CFML's escaped hash and opens nothing.
			if s.pos+1 < len(s.src) && s.src[s.pos+1] == '#' {
				s.pos += 2

				continue
			}

			if !s.scanHashExpr(depth + 1) {
				return false
			}
		case '\n':
			s.line++
			s.pos++
		default:
			s.pos++
		}
	}

	return false
}

// scanHashExpr consumes a #...# expression from its opening hash, including any
// string opened inside it. Reports whether it closed.
func (s *Scanner) scanHashExpr(depth int) bool {
	if depth >= maxStringNesting {
		return false
	}

	s.pos++ // the opening #

	for s.pos < len(s.src) {
		switch s.src[s.pos] {
		case '#':
			s.pos++

			return true
		case '"', '\'':
			if !s.scanQuoted(depth + 1) {
				return false
			}
		case '\n':
			s.line++
			s.pos++
		default:
			s.pos++
		}
	}

	return false
}

func (s *Scanner) scanNumber(start, startLine int) Token {
	for s.pos < len(s.src) && (isDigit(s.src[s.pos]) || s.src[s.pos] == '.') {
		s.pos++
	}

	return Token{Kind: TokNumber, Value: s.src[start:s.pos], Offset: start, Line: startLine}
}

func (s *Scanner) scanIdent(start, startLine int) Token {
	for s.pos < len(s.src) && isIdentPart(s.src[s.pos]) {
		s.pos++
	}

	return Token{Kind: TokIdent, Value: s.src[start:s.pos], Offset: start, Line: startLine}
}

// Rest returns the remaining unscanned source from current position.
func (s *Scanner) Rest() string {
	if s.pos >= len(s.src) {
		return ""
	}

	return s.src[s.pos:]
}

func isDigit(ch byte) bool { return ch >= '0' && ch <= '9' }
func isIdentStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_' || ch == '$'
}
func isIdentPart(ch byte) bool { return isIdentStart(ch) || isDigit(ch) }

func charToKind(ch byte) TokenKind {
	switch ch {
	case '(':
		return TokLParen
	case ')':
		return TokRParen
	case '{':
		return TokLBrace
	case '}':
		return TokRBrace
	case '[':
		return TokLBracket
	case ']':
		return TokRBracket
	case '.':
		return TokDot
	case ',':
		return TokComma
	case ';':
		return TokSemicolon
	case '=':
		return TokEquals
	case '<':
		return TokLT
	case '>':
		return TokGT
	case '/':
		return TokSlash
	case '#':
		return TokHash
	case ':':
		return TokColon
	case '?':
		return TokQuestion
	case '!':
		return TokBang
	case '&':
		return TokAmpersand
	case '|':
		return TokPipe
	case '+':
		return TokPlus
	case '-':
		return TokMinus
	case '*':
		return TokStar
	case '%':
		return TokPercent
	case '^':
		return TokCaret
	case '@':
		return TokAt
	default:
		return TokOther
	}
}

// identEq does a case-insensitive comparison of a token value to a keyword.
func identEq(val, keyword string) bool {
	return strings.EqualFold(val, keyword)
}
