// Package parser extracts component references from CFML source.
package parser

import (
	"regexp"
	"sort"

	"go.lsp.dev/uri"
)

// ComponentRef represents a variable assigned to a component instance.
type ComponentRef struct {
	Variable  string  // variable name (e.g. "myComponent")
	Component string  // dot-path (e.g. "dir.Entity")
	URI       uri.URI // file where the reference was found
	Line      uint32
}

var newRe = regexp.MustCompile(`(?im)(\w+)\s*=\s*new\s+["']?([\w.]+)["']?\s*(?:\(|[;\r\n])`)
var createRe = regexp.MustCompile(`(?im)(\w+)\s*=\s*(?:[Cc]reate[Oo]bject\(\s*["']component["']\s*,|[Ee]ntity[Nn]ew\()\s*["']([\w.]+)["']`)
var cfobjectRe = regexp.MustCompile(`(?i)<cfobject\s[^>]*(?:\bcomponent\s*=\s*["']([\w.]+)["'][^>]*\bname\s*=\s*["'](\w+)["']|\bname\s*=\s*["'](\w+)["'][^>]*\bcomponent\s*=\s*["']([\w.]+)["'])`)
var cfinvokeRe = regexp.MustCompile(`(?i)<cfinvoke\s[^>]*(?:\bcomponent\s*=\s*["']([\w.]+)["'][^>]*\breturnvariable\s*=\s*["'](\w+)["']|\breturnvariable\s*=\s*["'](\w+)["'][^>]*\bcomponent\s*=\s*["']([\w.]+)["'])`)

// ParseComponentRefs extracts component references from source content.
func ParseComponentRefs(fileURI uri.URI, content string) []ComponentRef {
	var refs []ComponentRef

	for _, m := range newRe.FindAllStringSubmatchIndex(content, -1) {
		refs = append(refs, ComponentRef{
			Variable:  content[m[2]:m[3]],
			Component: content[m[4]:m[5]],
			URI:       fileURI,
			Line:      uint32(countNewlines(content[:m[0]])),
		})
	}

	for _, m := range createRe.FindAllStringSubmatchIndex(content, -1) {
		refs = append(refs, ComponentRef{
			Variable:  content[m[2]:m[3]],
			Component: content[m[4]:m[5]],
			URI:       fileURI,
			Line:      uint32(countNewlines(content[:m[0]])),
		})
	}

	for _, m := range cfobjectRe.FindAllStringSubmatchIndex(content, -1) {
		refs = append(refs, extractTagRef(content, fileURI, m))
	}

	for _, m := range cfinvokeRe.FindAllStringSubmatchIndex(content, -1) {
		refs = append(refs, extractTagRef(content, fileURI, m))
	}

	sort.Slice(refs, func(i, j int) bool { return refs[i].Line < refs[j].Line })
	return refs
}

func extractTagRef(content string, fileURI uri.URI, m []int) ComponentRef {
	var component, variable string
	if m[2] >= 0 {
		component = content[m[2]:m[3]]
		variable = content[m[4]:m[5]]
	} else {
		variable = content[m[6]:m[7]]
		component = content[m[8]:m[9]]
	}
	return ComponentRef{
		Variable:  variable,
		Component: component,
		URI:       fileURI,
		Line:      uint32(countNewlines(content[:m[0]])),
	}
}

func countNewlines(s string) int {
	n := 0
	for _, b := range []byte(s) {
		if b == '\n' {
			n++
		}
	}
	return n
}
