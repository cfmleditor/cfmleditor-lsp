package frameworkapi

import (
	"strings"
	"sync"
)

// Doc is a stub function's documentation, from the doc comment its
// framework wrote above it.
type Doc struct {
	Summary string
	Params  []ParamDoc
	Returns string
	Throws  string
}

// ParamDoc documents one argument.
type ParamDoc struct {
	Name, Text string
}

// Param is the documentation for the named argument, or "".
func (d *Doc) Param(name string) string {
	for _, p := range d.Params {
		if strings.EqualFold(p.Name, name) {
			return p.Text
		}
	}

	return ""
}

// Markdown renders the documentation for a hover or a completion item.
func (d *Doc) Markdown() string {
	var b strings.Builder

	if d.Summary != "" {
		b.WriteString(d.Summary)
		b.WriteString("\n\n")
	}

	if len(d.Params) > 0 {
		b.WriteString("**Arguments**\n\n")

		for _, p := range d.Params {
			b.WriteString("- `" + p.Name + "`")

			if p.Text != "" {
				b.WriteString(" — " + p.Text)
			}

			b.WriteString("\n")
		}

		b.WriteString("\n")
	}

	if d.Returns != "" {
		b.WriteString("**Returns** " + d.Returns + "\n\n")
	}

	if d.Throws != "" {
		b.WriteString("**Throws** " + d.Throws + "\n\n")
	}

	b.WriteString("_From the bundled framework API: the source is not in the workspace._")

	return b.String()
}

// docs holds each stub's documentation by the line it documents, parsed the
// first time the stub is asked about.
var docs sync.Map // stub path → map[uint32]*Doc

// DocAt is the documentation for the declaration on line (0-based) of the
// stub at path.
func DocAt(path string, line uint32) (*Doc, bool) {
	if !IsStub(path) {
		return nil, false
	}

	byLine, ok := docs.Load(path)
	if !ok {
		text, found := StubText(path)
		if !found {
			return nil, false
		}

		byLine, _ = docs.LoadOrStore(path, parseDocs(text))
	}

	d, ok := byLine.(map[uint32]*Doc)[line]

	return d, ok
}

// parseDocs reads every /** */ block in a stub and files it under the line
// that follows it.
func parseDocs(text string) map[uint32]*Doc {
	out := map[uint32]*Doc{}
	lines := strings.Split(text, "\n")

	for i := 0; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "/**" {
			continue
		}

		var body []string

		j := i + 1
		for ; j < len(lines) && strings.TrimSpace(lines[j]) != "*/"; j++ {
			l := strings.TrimSpace(lines[j])
			l = strings.TrimSpace(strings.TrimPrefix(l, "*"))
			body = append(body, l)
		}

		if j+1 < len(lines) {
			out[uint32(j+1)] = parseDoc(body)
		}

		i = j
	}

	return out
}

// parseDoc reads a doc comment's lines: the summary up to the first tag,
// then @return, @throws and, for any other tag, an argument — ColdBox writes
// `@name The key name` where others write `@param name The key name`.
func parseDoc(lines []string) *Doc {
	d := &Doc{}

	var summary []string

	var last *string

	for _, l := range lines {
		if !strings.HasPrefix(l, "@") {
			switch {
			case last != nil && l != "":
				*last += " " + l
			case last == nil:
				summary = append(summary, l)
			}

			continue
		}

		tag, rest, _ := strings.Cut(l[1:], " ")
		rest = strings.TrimSpace(rest)

		switch strings.ToLower(tag) {
		case "return", "returns":
			d.Returns = rest
			last = &d.Returns
		case "throws", "throw":
			d.Throws = rest
			last = &d.Throws
		case "param", "argument", "arg":
			name, text, _ := strings.Cut(rest, " ")
			d.Params = append(d.Params, ParamDoc{Name: name, Text: strings.TrimSpace(text)})
			last = &d.Params[len(d.Params)-1].Text
		case "author", "see", "since", "deprecated", "hint", "output", "cachedwithin", "doc_generic":
			last = nil
		default:
			d.Params = append(d.Params, ParamDoc{Name: tag, Text: rest})
			last = &d.Params[len(d.Params)-1].Text
		}
	}

	d.Summary = strings.TrimSpace(strings.Join(summary, "\n"))

	return d
}
