package cfml

import (
	"fmt"
	"strings"
)

type Param struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
}

type Entry struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Syntax      string  `json:"syntax"`
	Returns     string  `json:"returns"`
	Type        string  `json:"type"` // "function" or "tag"
	Params      []Param `json:"params"`
}

func (e *Entry) Doc() string {
	var sb strings.Builder
	sb.WriteString(e.Description)
	if len(e.Params) > 0 {
		if e.Type == "tag" {
			sb.WriteString("\n\n**Attributes:**\n")
		} else {
			sb.WriteString("\n\n**Parameters:**\n")
		}
		for _, p := range e.Params {
			req := ""
			if p.Required {
				req = " *(required)*"
			}
			fmt.Fprintf(&sb, "- `%s` (%s)%s — %s\n", p.Name, p.Type, req, firstLine(p.Description))
		}
	}
	if e.Returns != "" && e.Type == "function" {
		fmt.Fprintf(&sb, "\n**Returns:** `%s`", e.Returns)
	}
	return sb.String()
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i != -1 {
		return s[:i]
	}
	return s
}

var (
	entries  []Entry
	tagMap   map[string]*Entry
	funcMap  map[string]*Entry
)

func init() {
	entries = builtinEntries()
	rebuildMaps()
}

func rebuildMaps() {
	tagMap = make(map[string]*Entry, len(entries))
	funcMap = make(map[string]*Entry, len(entries))
	for i := range entries {
		e := &entries[i]
		switch e.Type {
		case "tag":
			tagMap[strings.ToLower(e.Name)] = e
		case "function":
			funcMap[strings.ToLower(e.Name)] = e
		}
	}
}

func LookupTag(name string) (*Entry, bool) {
	e, ok := tagMap[strings.ToLower(name)]
	return e, ok
}

func LookupFunction(name string) (*Entry, bool) {
	e, ok := funcMap[strings.ToLower(name)]
	return e, ok
}

func AllTags() []*Entry {
	var out []*Entry
	for i := range entries {
		if entries[i].Type == "tag" {
			out = append(out, &entries[i])
		}
	}
	return out
}

func AllFunctions() []*Entry {
	var out []*Entry
	for i := range entries {
		if entries[i].Type == "function" {
			out = append(out, &entries[i])
		}
	}
	return out
}

func TagParams(name string) []Param {
	if e, ok := tagMap[strings.ToLower(name)]; ok {
		return e.Params
	}
	return nil
}

