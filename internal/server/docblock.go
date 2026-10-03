package server

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"go.lsp.dev/protocol"

	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
	"github.com/cfmleditor/cfmleditor-lsp/internal/docs"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
)

// Doc block completion, which the VS Code extension has as DocBlockCompletions:
//
//   - `/**` expands to a comment block for what follows it, with a placeholder
//     for the hint and a tag line for each argument of a function;
//   - inside a `/** … */`, `@` offers the attributes of the tag the comment
//     documents (cffunction, cfproperty, cfcomponent, cfinterface), and the
//     names of the function's arguments; `@arg.` offers a cfargument's
//     attributes.
//
// What the comment documents is what follows its close: a function, a property
// or a component, found by the extension's own patterns. The extension offered
// the tag completions anywhere in a component file; here the cursor must be
// inside a doc block, which is the only place a `@tag` line means anything.

var (
	docFunctionRe  = regexp.MustCompile(`(?i)^(\s*)(?:\b(?:private|package|public|remote|static|final|abstract|default)\s+)?(?:\b(?:private|package|public|remote|static|final|abstract|default)\s+)?(?:\b(?:[A-Za-z0-9_.$]+)\s+)?function\s+([_$a-zA-Z][$\w]*)\s*(?:\((?:=\s*\{|[^{])*)[{;]`)
	docPropertyRe  = regexp.MustCompile(`(?i)^(\s*property)\s+`)
	docComponentRe = regexp.MustCompile(`(?i)^(\s*(component|interface))\b[^{]*\{`)
	docTagWordRe   = regexp.MustCompile(`@([\w$]*)(?:\.([\w$]*))?$`)
)

type docKind int

const (
	docUnknown docKind = iota
	docFunction
	docProperty
	docComponent
	docInterface
)

func (k docKind) String() string {
	return [...]string{"unknown", "function", "property", "component", "interface"}[k]
}

// docTarget is what a doc block documents, from the text that follows it.
func docTarget(after string) docKind {
	switch {
	case docFunctionRe.MatchString(after):
		return docFunction
	case docPropertyRe.MatchString(after):
		return docProperty
	}

	if m := docComponentRe.FindStringSubmatch(after); m != nil {
		if strings.EqualFold(m[2], "interface") {
			return docInterface
		}

		return docComponent
	}

	return docUnknown
}

// docBlockItems answers a completion request that is for a doc block, and
// reports whether it is one.
func (s *Server) docBlockItems(content string, params *protocol.CompletionParams) ([]protocol.CompletionItem, bool) {
	line := int(params.Position.Line)
	text := parser.LineTextAt(content, line)
	col := byteCol(content, line, params.Position.Character)

	if col > len(text) {
		return nil, false
	}

	before := text[:col]

	// The extension's word range for `/**`: the cursor in it or at its end.
	for i := max(0, col-3); i <= col && i+3 <= len(text); i++ {
		if text[i:i+3] == "/**" {
			return s.docBlockExpansion(content, params, text, i), true
		}
	}

	if !strings.Contains(before, "@") {
		return nil, false
	}

	return s.docTagItems(content, params, before)
}

// docBlockExpansion is the `/** */` item for the `/**` at byte column start.
func (s *Server) docBlockExpansion(content string, params *protocol.CompletionParams, text string, start int) []protocol.CompletionItem {
	offset := lineStartOffset(content, int(params.Position.Line)) + start + 3
	after := strings.TrimLeft(content[offset:], " \t")
	// An editor that closed the comment as it opened it leaves ` */` first.
	after = strings.TrimPrefix(after, "*/")
	kind := docTarget(after)

	var args []string
	if kind == docFunction {
		args = docFunctionArguments(after)
	}

	snippet := buildDocBlock(kind, args, s.DocBlock)
	startChar := lineCol(text, start)

	// An editor that closed the comment as it opened it (`/** */`) leaves its
	// closer after the cursor, and the snippet brings its own.
	end := start + 3
	if rest := text[end:]; strings.HasPrefix(strings.TrimLeft(rest, " \t"), "*/") {
		end += len(rest) - len(strings.TrimLeft(rest, " \t")) + 2
	}

	return []protocol.CompletionItem{{
		Label:            "/** */",
		Kind:             protocol.CompletionItemKindSnippet,
		FilterText:       optStr("/**"),
		Documentation:    tooltip("Docblock completion"),
		InsertTextFormat: protocol.InsertTextFormatSnippet,
		TextEdit: &protocol.TextEdit{
			Range:   protocol.Range{Start: protocol.Position{Line: params.Position.Line, Character: startChar}, End: protocol.Position{Line: params.Position.Line, Character: lineCol(text, end)}},
			NewText: snippet,
		},
	}}
}

// buildDocBlock is the snippet for a doc block of kind, as the extension builds
// it: the hint a placeholder, a tag line per argument after a blank line when
// DocBlock.Gap says so, then the configured extra tags.
func buildDocBlock(kind docKind, args []string, cfg config.ResolvedDocBlock) string {
	var b strings.Builder

	n := 1

	placeholder := func(value string) {
		b.WriteString("${" + strconv.Itoa(n) + ":" + escapeSnippet(value) + "}")

		n++
	}

	b.WriteString("/**\n * ")
	placeholder("Undocumented " + kind.String())

	gap := false

	if len(args) > 0 {
		if cfg.Gap {
			b.WriteString("\n *")
		}

		gap = true

		for _, a := range args {
			b.WriteString("\n * @" + a + " ")
			placeholder("")
		}
	}

	var extra []config.DocBlockExtra

	for _, e := range cfg.Extra {
		if len(e.Types) == 0 || slices.Contains(e.Types, kind.String()) {
			extra = append(extra, e)
		}
	}

	if len(extra) > 0 {
		if !gap && cfg.Gap {
			b.WriteString("\n *")
		}

		for _, e := range extra {
			b.WriteString("\n * @" + e.Name + " ")
			placeholder(e.Default)
		}
	}

	b.WriteString("\n */")

	return b.String()
}

// escapeSnippet escapes what a snippet placeholder reads specially.
func escapeSnippet(s string) string {
	return strings.NewReplacer(`\`, `\\`, `$`, `\$`, `}`, `\}`).Replace(s)
}

// docFunctionArguments are the argument names of the function declared at the
// start of after, the text following a doc block. The declaration is parsed on
// its own: the comment being typed is usually not closed yet, and an unclosed
// comment hides the rest of the file from the document's own parse.
func docFunctionArguments(after string) []string {
	loc := docFunctionRe.FindStringIndex(after)
	if loc == nil {
		return nil
	}

	decl := strings.TrimRight(after[:loc[1]], "{;") + "{}"

	pr := parser.Parse("file:///docblock.cfc", "component {\n"+decl+"\n}")
	if len(pr.Funcs) == 0 {
		return nil
	}

	out := make([]string, 0, len(pr.Funcs[0].Arguments))
	for _, a := range pr.Funcs[0].Arguments {
		out = append(out, a.Name)
	}

	return out
}

// docTagItems offers the tags and sub-keys of a doc block the cursor is in.
func (s *Server) docTagItems(content string, params *protocol.CompletionParams, before string) ([]protocol.CompletionItem, bool) {
	m := docTagWordRe.FindStringSubmatchIndex(before)
	if m == nil {
		return nil, false
	}

	line := int(params.Position.Line)
	offset := lineStartOffset(content, line) + len(before)

	open := strings.LastIndex(content[:offset], "/**")
	if open < 0 || strings.Contains(content[open:offset], "*/") {
		return nil, false
	}

	end := strings.Index(content[offset:], "*/")
	if end < 0 {
		return nil, true
	}

	following := content[offset+end+2:]

	kind := docTarget(following)
	if kind == docUnknown {
		return nil, true
	}

	subKey := m[4] >= 0 // `@arg.` rather than `@`

	var items []protocol.CompletionItem

	attrs := func(tag string) {
		params := docs.TagParams(tag)
		for i := range params {
			if params[i].Name != "name" {
				items = append(items, protocol.CompletionItem{Label: params[i].Name, Kind: protocol.CompletionItemKindProperty, Documentation: tooltip(params[i].Description)})
			}
		}
	}

	if subKey {
		if kind != docFunction {
			return nil, true
		}

		arg := before[m[2]:m[3]]

		if !slices.Contains(docFunctionArguments(following), arg) {
			return nil, true
		}

		attrs("cfargument")

		return items, true
	}

	switch kind {
	case docFunction:
		attrs("cffunction")

		for _, a := range docFunctionArguments(following) {
			items = append(items, protocol.CompletionItem{Label: a, Kind: protocol.CompletionItemKindProperty})
		}
	case docProperty:
		attrs("cfproperty")
	case docInterface:
		attrs("cfinterface")
	case docComponent:
		attrs("cfcomponent")
	case docUnknown:
	}

	return items, true
}

// lineStartOffset is the byte offset of the start of line in content.
func lineStartOffset(content string, line int) int {
	off := 0

	for range line {
		i := strings.IndexByte(content[off:], '\n')
		if i < 0 {
			return len(content)
		}

		off += i + 1
	}

	return off
}
