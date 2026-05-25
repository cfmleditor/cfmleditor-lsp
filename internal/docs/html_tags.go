package docs

import "strings"

var globalAttrs = []Param{
	{Name: "id", Description: "Unique identifier", Type: "string"},
	{Name: "class", Description: "CSS class names", Type: "string"},
	{Name: "style", Description: "Inline CSS styles", Type: "string"},
	{Name: "title", Description: "Advisory text", Type: "string"},
	{Name: "hidden", Description: "Hidden element", Type: "boolean"},
	{Name: "tabindex", Description: "Tab order", Type: "string"},
	{Name: "role", Description: "ARIA role", Type: "string"},
}

var tagAttrs = map[string][]Param{
	"a":        {{Name: "href", Description: "URL", Type: "string", Required: true}, {Name: "target", Description: "Browsing context", Type: "string"}, {Name: "rel", Description: "Relationship", Type: "string"}, {Name: "download", Description: "Download filename", Type: "string"}},
	"img":      {{Name: "src", Description: "Image URL", Type: "string", Required: true}, {Name: "alt", Description: "Alternative text", Type: "string", Required: true}, {Name: "width", Description: "Width", Type: "string"}, {Name: "height", Description: "Height", Type: "string"}, {Name: "loading", Description: "Loading strategy", Type: "string"}},
	"input":    {{Name: "type", Description: "Input type", Type: "string", Required: true}, {Name: "name", Description: "Form control name", Type: "string"}, {Name: "value", Description: "Value", Type: "string"}, {Name: "placeholder", Description: "Placeholder text", Type: "string"}, {Name: "required", Description: "Required field", Type: "boolean"}, {Name: "disabled", Description: "Disabled", Type: "boolean"}, {Name: "readonly", Description: "Read only", Type: "boolean"}, {Name: "checked", Description: "Checked state", Type: "boolean"}, {Name: "maxlength", Description: "Maximum length", Type: "string"}, {Name: "min", Description: "Minimum value", Type: "string"}, {Name: "max", Description: "Maximum value", Type: "string"}, {Name: "pattern", Description: "Validation pattern", Type: "string"}},
	"form":     {{Name: "action", Description: "Form submission URL", Type: "string"}, {Name: "method", Description: "HTTP method", Type: "string"}, {Name: "enctype", Description: "Encoding type", Type: "string"}, {Name: "name", Description: "Form name", Type: "string"}, {Name: "target", Description: "Browsing context", Type: "string"}},
	"button":   {{Name: "type", Description: "Button type", Type: "string"}, {Name: "name", Description: "Button name", Type: "string"}, {Name: "value", Description: "Button value", Type: "string"}, {Name: "disabled", Description: "Disabled", Type: "boolean"}},
	"select":   {{Name: "name", Description: "Control name", Type: "string"}, {Name: "multiple", Description: "Allow multiple", Type: "boolean"}, {Name: "required", Description: "Required", Type: "boolean"}, {Name: "disabled", Description: "Disabled", Type: "boolean"}, {Name: "size", Description: "Visible options", Type: "string"}},
	"option":   {{Name: "value", Description: "Option value", Type: "string"}, {Name: "selected", Description: "Selected", Type: "boolean"}, {Name: "disabled", Description: "Disabled", Type: "boolean"}},
	"textarea": {{Name: "name", Description: "Control name", Type: "string"}, {Name: "rows", Description: "Visible rows", Type: "string"}, {Name: "cols", Description: "Visible columns", Type: "string"}, {Name: "placeholder", Description: "Placeholder text", Type: "string"}, {Name: "required", Description: "Required", Type: "boolean"}, {Name: "disabled", Description: "Disabled", Type: "boolean"}, {Name: "readonly", Description: "Read only", Type: "boolean"}, {Name: "maxlength", Description: "Maximum length", Type: "string"}},
	"label":    {{Name: "for", Description: "Associated control ID", Type: "string"}},
	"link":     {{Name: "rel", Description: "Relationship", Type: "string", Required: true}, {Name: "href", Description: "URL", Type: "string", Required: true}, {Name: "type", Description: "MIME type", Type: "string"}, {Name: "media", Description: "Media query", Type: "string"}},
	"meta":     {{Name: "name", Description: "Metadata name", Type: "string"}, {Name: "content", Description: "Metadata value", Type: "string"}, {Name: "charset", Description: "Character encoding", Type: "string"}, {Name: "http-equiv", Description: "HTTP header", Type: "string"}},
	"script":   {{Name: "src", Description: "Script URL", Type: "string"}, {Name: "type", Description: "Script type", Type: "string"}, {Name: "defer", Description: "Defer execution", Type: "boolean"}, {Name: "async", Description: "Async execution", Type: "boolean"}},
	"style":    {{Name: "media", Description: "Media query", Type: "string"}, {Name: "type", Description: "Style type", Type: "string"}},
	"iframe":   {{Name: "src", Description: "Frame URL", Type: "string", Required: true}, {Name: "width", Description: "Width", Type: "string"}, {Name: "height", Description: "Height", Type: "string"}, {Name: "name", Description: "Frame name", Type: "string"}, {Name: "sandbox", Description: "Sandbox restrictions", Type: "string"}, {Name: "loading", Description: "Loading strategy", Type: "string"}},
	"video":    {{Name: "src", Description: "Video URL", Type: "string"}, {Name: "controls", Description: "Show controls", Type: "boolean"}, {Name: "autoplay", Description: "Autoplay", Type: "boolean"}, {Name: "loop", Description: "Loop playback", Type: "boolean"}, {Name: "muted", Description: "Muted", Type: "boolean"}, {Name: "width", Description: "Width", Type: "string"}, {Name: "height", Description: "Height", Type: "string"}, {Name: "poster", Description: "Poster image", Type: "string"}},
	"audio":    {{Name: "src", Description: "Audio URL", Type: "string"}, {Name: "controls", Description: "Show controls", Type: "boolean"}, {Name: "autoplay", Description: "Autoplay", Type: "boolean"}, {Name: "loop", Description: "Loop playback", Type: "boolean"}, {Name: "muted", Description: "Muted", Type: "boolean"}},
	"source":   {{Name: "src", Description: "Media URL", Type: "string", Required: true}, {Name: "type", Description: "MIME type", Type: "string"}, {Name: "media", Description: "Media query", Type: "string"}},
	"table":    {{Name: "border", Description: "Border width", Type: "string"}},
	"td":       {{Name: "colspan", Description: "Column span", Type: "string"}, {Name: "rowspan", Description: "Row span", Type: "string"}},
	"th":       {{Name: "colspan", Description: "Column span", Type: "string"}, {Name: "rowspan", Description: "Row span", Type: "string"}, {Name: "scope", Description: "Header scope", Type: "string"}},
	"ol":       {{Name: "type", Description: "List marker type", Type: "string"}, {Name: "start", Description: "Start value", Type: "string"}, {Name: "reversed", Description: "Reversed order", Type: "boolean"}},
	"details":  {{Name: "open", Description: "Initially open", Type: "boolean"}},
	"dialog":   {{Name: "open", Description: "Dialog is open", Type: "boolean"}},
}

var htmlTags []Entry

func init() {
	names := []struct {
		name string
		desc string
	}{
		{"a", "Hyperlink"}, {"abbr", "Abbreviation"}, {"article", "Article content"},
		{"aside", "Aside content"}, {"audio", "Audio content"}, {"b", "Bold text"},
		{"blockquote", "Block quotation"}, {"body", "Document body"}, {"br", "Line break"},
		{"button", "Button"}, {"canvas", "Canvas for graphics"}, {"code", "Code fragment"},
		{"dd", "Description definition"}, {"details", "Disclosure widget"},
		{"dialog", "Dialog box"}, {"div", "Generic container"}, {"dl", "Description list"},
		{"dt", "Description term"}, {"em", "Emphasis"}, {"fieldset", "Form field group"},
		{"figcaption", "Figure caption"}, {"figure", "Figure with caption"},
		{"footer", "Footer"}, {"form", "Form"},
		{"h1", "Heading level 1"}, {"h2", "Heading level 2"}, {"h3", "Heading level 3"},
		{"h4", "Heading level 4"}, {"h5", "Heading level 5"}, {"h6", "Heading level 6"},
		{"head", "Document head"}, {"header", "Header"}, {"hr", "Horizontal rule"},
		{"html", "HTML document"}, {"i", "Italic text"}, {"iframe", "Inline frame"},
		{"img", "Image"}, {"input", "Form input"}, {"label", "Form label"},
		{"li", "List item"}, {"link", "External resource link"}, {"main", "Main content"},
		{"meta", "Metadata"}, {"nav", "Navigation"}, {"ol", "Ordered list"},
		{"option", "Select option"}, {"p", "Paragraph"}, {"picture", "Picture element"},
		{"pre", "Preformatted text"}, {"progress", "Progress indicator"},
		{"script", "Script"}, {"section", "Section"}, {"select", "Select dropdown"},
		{"small", "Small text"}, {"source", "Media source"}, {"span", "Inline container"},
		{"strong", "Strong importance"}, {"style", "Style information"},
		{"summary", "Details summary"}, {"table", "Table"}, {"tbody", "Table body"},
		{"td", "Table cell"}, {"template", "Template"}, {"textarea", "Multiline text input"},
		{"tfoot", "Table footer"}, {"th", "Table header cell"}, {"thead", "Table header"},
		{"title", "Document title"}, {"tr", "Table row"}, {"ul", "Unordered list"},
		{"video", "Video content"},
	}
	htmlTags = make([]Entry, len(names))

	for i, n := range names {
		htmlTags[i] = Entry{Name: n.name, Description: n.desc, Type: "tag"}
	}
}

// HTMLTags returns common HTML tag entries for autocompletion.
func HTMLTags() []Entry {
	return htmlTags
}

// HTMLTagParams returns attributes for the given HTML tag (tag-specific + globals).
func HTMLTagParams(name string) []Param {
	name = strings.ToLower(name)
	specific := tagAttrs[name]

	if specific == nil && !isHTMLTag(name) {
		return nil
	}

	out := make([]Param, 0, len(specific)+len(globalAttrs))
	out = append(out, specific...)
	out = append(out, globalAttrs...)

	return out
}

func isHTMLTag(name string) bool {
	for i := range htmlTags {
		if htmlTags[i].Name == name {
			return true
		}
	}

	return false
}
