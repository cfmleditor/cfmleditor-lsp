package config

import "github.com/cfmleditor/cfmleditor-lsp/internal/formatter"

// DefaultResolvedFormatting returns the documented default for every knob,
// for callers that format without a config to read. Enabled stays false,
// because the LSP formatting capability is opt-in.
//
// Note that [Resolve] deliberately does not use this: an absent "formatting"
// block resolves to the zero value there, pinned by
// TestResolve_FormattingAbsentIsZeroValueNotDefaults. That is inert for the
// server, which gates on Enabled first, but it means a zero-valued
// ResolvedFormatting must not be handed to FormatterOptions expecting
// defaults.
func DefaultResolvedFormatting() ResolvedFormatting {
	return ResolvedFormatting{
		SelfCloseTags:          true,
		WhitespaceOnly:         true,
		QueryFormat:            false,
		LowercaseTags:          true,
		LowercaseAttributes:    true,
		DoubleQuoteAttributes:  true,
		QueryUppercaseKeywords: true,
		LineWidth:              100,
		AttrBreakThreshold:     4,
		IndentWidth:            4,
	}
}

// FormatterOptions maps resolved formatting config onto formatter options,
// starting from formatter defaults.
//
// This is the single translation from config to formatter behaviour, shared by
// the LSP's textDocument/formatting handler and the `format` subcommand. When
// the two built their own option structs, `format -w` silently ignored every
// `formatting` key in .cfmleditor.json and produced different bytes than the
// editor did for the same file.
//
// The zero-valued int fields are treated as "unset" rather than as literal
// zeros, so a config that specifies only some keys keeps the defaults for the
// rest. Parse hooks are left nil: they belong to the caller, which is what
// holds the tree-sitter language handles.
func (r ResolvedFormatting) FormatterOptions() formatter.Options {
	o := formatter.DefaultOptions()

	o.SelfCloseTags = r.SelfCloseTags
	o.WhitespaceOnly = r.WhitespaceOnly
	o.QueryFormat = r.QueryFormat
	o.LowercaseTags = r.LowercaseTags
	o.LowercaseAttributes = r.LowercaseAttributes
	o.DoubleQuoteAttributes = r.DoubleQuoteAttributes
	o.QueryUppercaseKeywords = r.QueryUppercaseKeywords
	o.ScopeCase = r.ScopeCase
	o.CommaPosition = r.CommaPosition
	o.QueryCommaPosition = r.QueryCommaPosition

	if r.LineWidth > 0 {
		o.LineWidth = r.LineWidth
	}

	if r.AttrBreakThreshold > 0 {
		o.AttrBreakThreshold = r.AttrBreakThreshold
	}

	if r.IndentWidth > 0 {
		o.IndentWidth = r.IndentWidth
	}

	return o
}
