// Package docs provides built-in CFML tag and function documentation.
package docs

import (
	"fmt"
	"strings"
)

// Param describes a single parameter or attribute of a CFML tag or function.
type Param struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Values      []string `json:"values"`
}

// ParamValues returns allowed values from the Values field, falling back to type inference.
func (p *Param) ParamValues() []string {
	if len(p.Values) > 0 {
		return p.Values
	}

	if p.Type == "boolean" {
		return []string{"true", "false"}
	}

	return nil
}

// Entry holds documentation for a single CFML tag or function.
type Entry struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Syntax      string  `json:"syntax"`
	Member      string  `json:"member"`
	Script      string  `json:"script"`
	Returns     string  `json:"returns"`
	Type        string  `json:"type"` // "function" or "tag"
	Params      []Param `json:"params"`
}

// Doc returns a formatted markdown documentation string for the entry.
func (e *Entry) Doc() string {
	var sb strings.Builder

	sb.WriteString(e.Description)

	if e.Member != "" {
		fmt.Fprintf(&sb, "\n\n**Member:** `%s`", e.Member)
	}

	if e.Script != "" {
		fmt.Fprintf(&sb, "\n\n**Script:** `%s`", e.Script)
	}

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
	if before, _, ok := strings.Cut(s, "\n"); ok {
		return before
	}

	return s
}

var (
	entries []Entry
	tagMap  map[string]*Entry
	funcMap map[string]*Entry
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

// LookupTag returns the documentation entry for a CFML tag by name.
func LookupTag(name string) (*Entry, bool) {
	e, ok := tagMap[strings.ToLower(name)]

	return e, ok
}

// LookupFunction returns the documentation entry for a CFML function by name.
func LookupFunction(name string) (*Entry, bool) {
	e, ok := funcMap[strings.ToLower(name)]

	return e, ok
}

// AllTags returns all documented CFML tag entries.
func AllTags() []*Entry {
	var out []*Entry

	for i := range entries {
		if entries[i].Type == "tag" {
			out = append(out, &entries[i])
		}
	}

	return out
}

// AllFunctions returns all documented CFML function entries.
func AllFunctions() []*Entry {
	var out []*Entry

	for i := range entries {
		if entries[i].Type == "function" {
			out = append(out, &entries[i])
		}
	}

	return out
}

// TagParams returns the parameters for the named CFML tag, or nil if not found.
func TagParams(name string) []Param {
	if e, ok := tagMap[strings.ToLower(name)]; ok {
		return e.Params
	}

	return nil
}
