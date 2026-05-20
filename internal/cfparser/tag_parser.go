package cfparser

import (
	"strings"
)

// tagParser extracts definitions from CFML tag-based source.
type tagParser struct {
	src        string
	fileURI    string
	funcs      []FunctionDef
	vars       []VarDef
	refs       []ComponentRef
	properties []propertyDef
	extends    string
	persistent bool
	lineIndex  []int // byte offset of each line start
}

func newTagParser(src, fileURI string) *tagParser {
	p := &tagParser{src: src, fileURI: fileURI}
	p.buildLineIndex()
	return p
}

func (p *tagParser) buildLineIndex() {
	n := 1
	for i := 0; i < len(p.src); i++ {
		if p.src[i] == '\n' {
			n++
		}
	}
	p.lineIndex = make([]int, 1, n)
	for i := 0; i < len(p.src); i++ {
		if p.src[i] == '\n' {
			p.lineIndex = append(p.lineIndex, i+1)
		}
	}
}

// lineAt returns the 0-based line number for a byte offset using binary search.
func (p *tagParser) lineAt(offset int) int {
	lo, hi := 0, len(p.lineIndex)
	for lo < hi {
		mid := (lo + hi) / 2
		if p.lineIndex[mid] <= offset {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo - 1
}

// parse scans through tag-based CFML extracting definitions.
func (p *tagParser) parse() {
	pos := 0

	for pos < len(p.src) {
		// Find next < that could be a CF tag
		idx := strings.IndexByte(p.src[pos:], '<')
		if idx < 0 {
			break
		}
		idx += pos

		// Check for cfscript block
		if idx+10 <= len(p.src) && strings.EqualFold(p.src[idx:idx+10], "<cfscript>") {
			bodyStart := idx + 10
			closeIdx := indexCFTag(p.src[bodyStart:], "/cfscript>")
			var bodyEnd int
			if closeIdx < 0 {
				bodyEnd = len(p.src)
			} else {
				bodyEnd = bodyStart + closeIdx
			}
			baseLine := p.lineAt(bodyStart)
			sp := newScriptParser(p.src[bodyStart:bodyEnd], p.fileURI, baseLine)
			sp.parse()
			p.funcs = append(p.funcs, sp.funcs...)
			p.vars = append(p.vars, sp.vars...)
			p.refs = append(p.refs, sp.refs...)
			if closeIdx < 0 {
				pos = len(p.src)
			} else {
				pos = bodyEnd + 11 // len("</cfscript>")
			}
			continue
		}

		// Check for CF tags we care about
		if idx+3 < len(p.src) && toLowerByte(p.src[idx+1]) == 'c' && toLowerByte(p.src[idx+2]) == 'f' {
			tagEnd := strings.IndexByte(p.src[idx:], '>')
			if tagEnd < 0 {
				pos = idx + 1
				continue
			}
			tagEnd += idx + 1 // past the >
			tag := p.src[idx:tagEnd]
			line := p.lineAt(idx)

			switch {
			case len(tag) > 12 && strings.EqualFold(tag[:13], "<cfcomponent "):
				p.extends = getAttr(tag, "extends")
				if isTruthy(getAttr(tag, "persistent")) {
					p.persistent = true
				}
			case len(tag) > 11 && strings.EqualFold(tag[:12], "<cffunction "):
				p.parseCFFunction(tag, idx, tagEnd, line)
				pos = tagEnd
				continue
			case len(tag) > 11 && strings.EqualFold(tag[:12], "<cfproperty "):
				p.parseCFProperty(tag, line)
			case len(tag) > 5 && strings.EqualFold(tag[:6], "<cfset"):
				p.parseCFSet(tag, line)
			case len(tag) > 8 && strings.EqualFold(tag[:9], "<cfobject"):
				p.parseCFObject(tag, line)
			case len(tag) > 8 && strings.EqualFold(tag[:9], "<cfinvoke"):
				p.parseCFInvoke(tag, line)
			}
			pos = tagEnd
		} else {
			pos = idx + 1
		}
	}
}

// parseCFFunction extracts a function def from <cffunction> and its <cfargument> children.
func (p *tagParser) parseCFFunction(tag string, _, tagEnd, line int) {
	name := getAttr(tag, "name")
	if name == "" {
		return
	}

	// Find arguments between this tag and </cffunction> or next <cffunction
	rest := p.src[tagEnd:]

	end := len(rest)
	if ci := indexCFTag(rest, "/cffunction"); ci >= 0 && ci < end {
		end = ci
	}
	if ci := indexCFTag(rest, "cffunction"); ci >= 0 && ci < end {
		end = ci
	}
	block := rest[:end]

	args := p.parseCFArguments(block)

	p.funcs = append(p.funcs, FunctionDef{
		Name:      name,
		URI:       uriFromString(p.fileURI),
		Line:      uint32(line),
		Arguments: args,
	})
}

// parseCFArguments extracts <cfargument> tags from a block of source.
func (p *tagParser) parseCFArguments(block string) []Argument {
	var args []Argument
	pos := 0
	for {
		idx := indexCFTag(block[pos:], "cfargument")
		if idx < 0 {
			break
		}
		idx += pos
		end := strings.IndexByte(block[idx:], '>')
		if end < 0 {
			break
		}
		tag := block[idx : idx+end+1]
		name := getAttr(tag, "name")
		if name != "" {
			a := Argument{Name: name}
			a.Type = getAttr(tag, "type")
			req := getAttr(tag, "required")
			a.Required = strings.EqualFold(req, "true") || strings.EqualFold(req, "yes")
			args = append(args, a)
		}
		pos = idx + end + 1
	}
	return args
}

// parseCFSet handles <cfset var x = ...>, <cfset local.x = ...>, etc.
func (p *tagParser) parseCFSet(tag string, line int) {
	// Strip <cfset and trailing >
	inner := tag
	if i := strings.IndexByte(inner, ' '); i >= 0 {
		inner = inner[i+1:]
	}
	inner = strings.TrimSuffix(inner, ">")
	inner = strings.TrimSpace(inner)

	switch {
	case hasPrefixFold(inner, "var "):
		rest := strings.TrimSpace(inner[4:])
		name := extractIdent(rest)
		if name != "" {
			p.vars = append(p.vars, VarDef{Name: name, Scope: ScopeLocal, Line: uint32(line)})
			p.checkSetRHS(rest, name, line)
		}
	case hasPrefixFold(inner, "local."):
		rest := inner[6:]
		name, rhs := splitAssign(rest)
		if name != "" {
			p.vars = append(p.vars, VarDef{Name: name, Scope: ScopeLocal, Line: uint32(line)})
			p.checkSetRHSStr(rhs, name, line)
		}
	case hasPrefixFold(inner, "arguments."):
		rest := inner[10:]
		name, rhs := splitAssign(rest)
		if name != "" {
			p.vars = append(p.vars, VarDef{Name: name, Scope: ScopeArguments, Line: uint32(line)})
			p.checkSetRHSStr(rhs, name, line)
		}
	case hasPrefixFold(inner, "this."):
		rest := inner[5:]
		name, rhs := splitAssign(rest)
		if name != "" {
			p.vars = append(p.vars, VarDef{Name: name, Scope: ScopeThis, Line: uint32(line)})
			p.checkSetRHSStr(rhs, name, line)
		}
	case hasPrefixFold(inner, "variables."):
		rest := inner[10:]
		name, rhs := splitAssign(rest)
		if name != "" {
			p.vars = append(p.vars, VarDef{Name: name, Scope: ScopeVariables, Line: uint32(line)})
			p.checkSetRHSStr(rhs, name, line)
		}
	default:
		name, rhs := splitAssign(inner)
		if name != "" && !isKeyword(name) {
			p.vars = append(p.vars, VarDef{Name: name, Scope: ScopeVariables, Line: uint32(line)})
			p.checkSetRHSStr(rhs, name, line)
		}
	}
}

// parseCFObject handles <cfobject component="path" name="var">
func (p *tagParser) parseCFObject(tag string, line int) {
	component := getAttr(tag, "component")
	name := getAttr(tag, "name")
	if component != "" && name != "" {
		p.refs = append(p.refs, ComponentRef{
			Variable:  name,
			Component: component,
			URI:       uriFromString(p.fileURI),
			Line:      uint32(line),
		})
	}
}

// parseCFInvoke handles <cfinvoke component="path" returnvariable="var">
func (p *tagParser) parseCFInvoke(tag string, line int) {
	component := getAttr(tag, "component")
	variable := getAttr(tag, "returnvariable")
	if component != "" && variable != "" {
		p.refs = append(p.refs, ComponentRef{
			Variable:  variable,
			Component: component,
			URI:       uriFromString(p.fileURI),
			Line:      uint32(line),
		})
	}
}

// parseCFProperty handles <cfproperty name="x" type="y" />
func (p *tagParser) parseCFProperty(tag string, line int) {
	name := getAttr(tag, "name")
	if name == "" {
		return
	}
	typeName := getAttr(tag, "type")
	attrs := extractAllAttrs(tag)
	p.properties = append(p.properties, propertyDef{name: name, typeName: typeName, line: uint32(line), attrs: attrs})
}

func (p *tagParser) checkSetRHS(rest, varName string, line int) {
	// Find = and check what's after it
	if i := strings.IndexByte(rest, '='); i >= 0 {
		p.checkSetRHSStr(strings.TrimSpace(rest[i+1:]), varName, line)
	}
}

func (p *tagParser) checkSetRHSStr(rhs, varName string, line int) {
	rhs = strings.TrimSpace(rhs)
	switch {
	case hasPrefixFold(rhs, "new "):
		comp := extractComponentPath(rhs[4:])
		if comp != "" {
			p.refs = append(p.refs, ComponentRef{
				Variable: varName, Component: comp,
				URI: uriFromString(p.fileURI), Line: uint32(line),
			})
		}
	case hasPrefixFold(rhs, "createobject("):
		comp := extractCreateObjectArg(rhs[13:])
		if comp != "" {
			p.refs = append(p.refs, ComponentRef{
				Variable: varName, Component: comp,
				URI: uriFromString(p.fileURI), Line: uint32(line),
			})
		}
	case hasPrefixFold(rhs, "entitynew("):
		comp := extractEntityNewArg(rhs[10:])
		if comp != "" {
			p.refs = append(p.refs, ComponentRef{
				Variable: varName, Component: comp,
				URI: uriFromString(p.fileURI), Line: uint32(line),
			})
		}
	case hasPrefixFold(rhs, "entityload("):
		comp := extractEntityNewArg(rhs[11:])
		if comp != "" {
			p.refs = append(p.refs, ComponentRef{
				Variable: varName, Component: comp,
				URI: uriFromString(p.fileURI), Line: uint32(line),
			})
		}
	}
}

// getAttr extracts an attribute value from a tag string (case-insensitive).
func getAttr(tag, attr string) string {
	lower := strings.ToLower(tag)
	key := strings.ToLower(attr) + "="
	idx := strings.Index(lower, key)
	if idx < 0 {
		// Try with space before
		key = " " + key
		idx = strings.Index(lower, key)
		if idx < 0 {
			return ""
		}
		idx++ // skip the space
	}
	valStart := idx + len(attr) + 1 // past =
	if valStart >= len(tag) {
		return ""
	}
	ch := tag[valStart]
	if ch == '"' || ch == '\'' {
		valStart++
		end := strings.IndexByte(tag[valStart:], ch)
		if end < 0 {
			return ""
		}
		return tag[valStart : valStart+end]
	}
	// Unquoted value — read until space or >
	end := valStart
	for end < len(tag) && tag[end] != ' ' && tag[end] != '>' && tag[end] != '\t' {
		end++
	}
	return tag[valStart:end]
}

func extractIdent(s string) string {
	i := 0
	for i < len(s) && isIdentPart(s[i]) {
		i++
	}
	if i == 0 {
		return ""
	}
	return s[:i]
}

// extractAllAttrs extracts all attribute key=value pairs from a tag string.
// Keys are lowercased.
func extractAllAttrs(tag string) map[string]string {
	attrs := make(map[string]string)
	// Skip tag name
	i := 1 // past '<'
	for i < len(tag) && tag[i] != ' ' && tag[i] != '\t' && tag[i] != '>' {
		i++
	}
	for i < len(tag) {
		// Skip whitespace
		for i < len(tag) && (tag[i] == ' ' || tag[i] == '\t' || tag[i] == '\n' || tag[i] == '\r') {
			i++
		}
		if i >= len(tag) || tag[i] == '>' || tag[i] == '/' {
			break
		}
		// Read attribute name
		start := i
		for i < len(tag) && tag[i] != '=' && tag[i] != ' ' && tag[i] != '>' && tag[i] != '/' {
			i++
		}
		key := strings.ToLower(tag[start:i])
		if i >= len(tag) || tag[i] != '=' {
			continue
		}
		i++ // past '='
		if i >= len(tag) {
			break
		}
		if tag[i] == '"' || tag[i] == '\'' {
			q := tag[i]
			i++
			valStart := i
			for i < len(tag) && tag[i] != q {
				i++
			}
			attrs[key] = tag[valStart:i]
			if i < len(tag) {
				i++ // past closing quote
			}
		} else {
			valStart := i
			for i < len(tag) && tag[i] != ' ' && tag[i] != '>' {
				i++
			}
			attrs[key] = tag[valStart:i]
		}
	}
	return attrs
}

func splitAssign(s string) (name, rhs string) {
	name = extractIdent(s)
	if name == "" {
		return "", ""
	}
	rest := strings.TrimSpace(s[len(name):])
	if len(rest) > 0 && rest[0] == '=' {
		return name, strings.TrimSpace(rest[1:])
	}
	return name, ""
}

func extractComponentPath(s string) string {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return ""
	}
	if s[0] == '"' || s[0] == '\'' {
		q := s[0]
		end := strings.IndexByte(s[1:], q)
		if end < 0 {
			return ""
		}
		return s[1 : 1+end]
	}
	// Unquoted dotted path
	i := 0
	for i < len(s) && (isIdentPart(s[i]) || s[i] == '.') {
		i++
	}
	return s[:i]
}

func extractCreateObjectArg(s string) string {
	// Expects: "component", "path") — we're past the opening (
	s = strings.TrimSpace(s)
	if len(s) == 0 || (s[0] != '"' && s[0] != '\'') {
		return ""
	}
	q := s[0]
	end := strings.IndexByte(s[1:], q)
	if end < 0 {
		return ""
	}
	first := s[1 : 1+end]
	if !strings.EqualFold(first, "component") {
		return ""
	}
	rest := s[2+end:]
	ci := strings.IndexByte(rest, ',')
	if ci < 0 {
		return ""
	}
	rest = strings.TrimSpace(rest[ci+1:])
	if len(rest) == 0 || (rest[0] != '"' && rest[0] != '\'') {
		return ""
	}
	q2 := rest[0]
	end2 := strings.IndexByte(rest[1:], q2)
	if end2 < 0 {
		return ""
	}
	return rest[1 : 1+end2]
}

func extractEntityNewArg(s string) string {
	// Expects: "EntityName") — we're past the opening (
	s = strings.TrimSpace(s)
	if len(s) == 0 || (s[0] != '"' && s[0] != '\'') {
		return ""
	}
	q := s[0]
	end := strings.IndexByte(s[1:], q)
	if end < 0 {
		return ""
	}
	return s[1 : 1+end]
}

func toLowerByte(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + 32
	}
	return b
}

// hasPrefixFold checks if s starts with prefix (case-insensitive, ASCII only).
func hasPrefixFold(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return strings.EqualFold(s[:len(prefix)], prefix)
}
