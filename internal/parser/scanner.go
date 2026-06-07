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
	src  string
	pos  int
	line int
}

// NewScanner creates a scanner for the given source.
func NewScanner(src string) *Scanner {
	return &Scanner{src: src}
}

// Pos returns the current byte offset.
func (s *Scanner) Pos() int { return s.pos }

// Line returns the current line number.
func (s *Scanner) Line() int { return s.line }

// ScannerState holds saved scanner position for backtracking.
type ScannerState struct {
	pos  int
	line int
}

// Save returns the current scanner state for later restoration.
func (s *Scanner) Save() ScannerState { return ScannerState{pos: s.pos, line: s.line} }

// Restore resets the scanner to a previously saved state.
func (s *Scanner) Restore(st ScannerState) { s.pos = st.pos; s.line = st.line }

// Peek returns the next token without advancing.
func (s *Scanner) Peek() Token {
	pos, line := s.pos, s.line
	tok := s.Next()
	s.pos, s.line = pos, line

	return tok
}

// Next returns the next token and advances the scanner.
func (s *Scanner) Next() Token {
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

	default:
		s.pos++
		kind := charToKind(ch)

		return Token{Kind: kind, Value: string(ch), Offset: start, Line: startLine}
	}
}

// NextSkipComments returns the next non-comment, non-newline token.
func (s *Scanner) NextSkipComments() Token {
	for {
		tok := s.Next()
		switch tok.Kind { //nolint:exhaustive
		case TokCFComment, TokBlockComment, TokLineComment, TokNewline:
			continue
		default:
			return tok
		}
	}
}

// PeekSkipComments peeks at the next non-comment, non-newline token.
func (s *Scanner) PeekSkipComments() Token {
	pos, line := s.pos, s.line
	tok := s.NextSkipComments()
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
	for s.pos < len(s.src) && s.src[s.pos] != '\n' {
		s.pos++
	}

	return Token{Kind: TokLineComment, Value: s.src[start:s.pos], Offset: start, Line: startLine}
}

func (s *Scanner) scanString(start, startLine int) Token {
	q := s.src[s.pos]
	s.pos++

	for s.pos < len(s.src) && s.src[s.pos] != q {
		if s.src[s.pos] == '\\' && s.pos+1 < len(s.src) {
			s.pos++
		}

		if s.src[s.pos] == '\n' {
			s.line++
		}

		s.pos++
	}

	if s.pos < len(s.src) {
		s.pos++ // closing quote
	}

	return Token{Kind: TokString, Value: s.src[start:s.pos], Offset: start, Line: startLine}
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
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
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
