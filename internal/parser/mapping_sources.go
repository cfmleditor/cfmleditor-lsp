package parser

import (
	"path/filepath"
	"strconv"
	"strings"
)

// MappingSources reads statically knowable application mappings in execution
// order, including literal includes and parent constructors. It never runs CFML.
func MappingSources(file string, initial map[string]string, read func(string) ([]byte, error)) map[string]string {
	s := &mappingSourceState{mappings: map[string]string{}, defaults: initial, env: map[string]string{}, seen: map[string]bool{}, read: read, base: file}
	s.file(file, 0)

	return s.mappings
}

type mappingSourceState struct {
	mappings, defaults, env map[string]string
	seen                    map[string]bool
	read                    func(string) ([]byte, error)
	base                    string
}

func mappingParentName(content string) string {
	sc := NewScanner(content)
	for {
		t := sc.NextSkipComments()
		if t.Kind == TokEOF {
			return ""
		}

		if t.Kind != TokIdent || !identEq(t.Value, "component") && !identEq(t.Value, "cfcomponent") {
			continue
		}

		for {
			t = sc.NextSkipComments()
			if t.Kind == TokEOF || t.Kind == TokGT || t.Kind == TokLBrace {
				return ""
			}

			if t.Kind == TokIdent && identEq(t.Value, "extends") && sc.NextSkipComments().Kind == TokEquals {
				t = sc.NextSkipComments()
				if t.Kind == TokString {
					value, ok := stringLiteral(t.Value)
					if ok && !strings.Contains(value, "#") {
						return value
					}
				}
			}
		}
	}
}

func (s *mappingSourceState) file(file string, depth int) {
	file = filepath.Clean(file)
	if depth >= 16 || len(s.seen) >= 64 || s.seen[file] {
		return
	}

	s.seen[file] = true

	data, err := s.read(file)
	if err != nil {
		return
	}

	content := string(data)
	if strings.EqualFold(filepath.Ext(file), ".cfc") {
		if parent := mappingParentName(content); parent != "" {
			path := filepath.Join(filepath.Dir(file), strings.ReplaceAll(parent, ".", string(filepath.Separator))+".cfc")
			s.file(path, depth+1)
		}
	}

	s.content(content, file, depth)
}

func (s *mappingSourceState) content(content, file string, depth int) {
	s.env["@template"] = file
	s.env["@baseTemplate"] = s.base
	sc := NewScanner(content)
	previous := TokEOF

	for {
		t := sc.NextSkipComments()
		before := previous

		previous = t.Kind
		if t.Kind == TokEOF {
			return
		}

		if t.Kind != TokIdent || before == TokDot {
			continue
		}

		if identEq(t.Value, "component") || identEq(t.Value, "cfcomponent") {
			for {
				header := sc.NextSkipComments()
				if header.Kind == TokLBrace || header.Kind == TokGT || header.Kind == TokEOF {
					break
				}
			}

			continue
		}

		if index := indexFold(t.Value, "include"); index >= 0 && s.read != nil {
			start, re := includeFormAt(content, t.Offset+index)
			if re != nil {
				if m := re.FindStringSubmatch(content[start:min(len(content), start+includeWindow)]); m != nil && isIncludable(m[1]) {
					s.file(s.includePath(m[1], file), depth+1)
					s.env["@template"] = file
				}
			}
		}

		cursor := *sc
		cursor.Restore(ScannerState{pos: t.Offset, line: t.Line})

		target, key, ok := mappingSourceTarget(&cursor)
		if !ok || cursor.NextSkipComments().Kind != TokEquals || cursor.PeekSkipComments().Kind == TokEquals {
			continue
		}

		expression := mappingSourceExpression(&cursor)
		*sc = cursor

		s.assignment(target, key, expression, filepath.Dir(file))
	}
}

func mappingSourceTarget(sc *Scanner) (target, key string, ok bool) {
	t := sc.NextSkipComments()
	if t.Kind != TokIdent {
		return "", "", false
	}

	if identEq(t.Value, "var") {
		t = sc.NextSkipComments()
		if t.Kind != TokIdent {
			return "", "", false
		}
	}

	var path chainBuilder
	path.reset(t.Value)

	for sc.PeekSkipComments().Kind == TokDot {
		sc.NextSkipComments()

		t = sc.NextSkipComments()
		if t.Kind != TokIdent {
			return "", "", false
		}

		path.writeDot()
		path.writeString(t.Value)
	}

	target = path.String()

	if sc.PeekSkipComments().Kind == TokLBracket {
		sc.NextSkipComments()

		t = sc.NextSkipComments()
		if t.Kind != TokString {
			return "", "", false
		}

		key = strings.Trim(t.Value, `"'`)
		if strings.Contains(key, "#") || sc.NextSkipComments().Kind != TokRBracket {
			return "", "", false
		}
	}

	return target, key, true
}

func mappingSourceExpression(sc *Scanner) string {
	start := sc.PeekSkipComments().Offset
	depth, lastLine := 0, sc.PeekSkipComments().Line
	previous := TokEOF

	for {
		t := sc.PeekSkipComments()
		if t.Kind == TokEOF || depth == 0 && (t.Kind == TokSemicolon || t.Kind == TokGT || t.Kind == TokRBrace || t.Line > lastLine && previous != TokAmpersand) {
			return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(sc.src[start:t.Offset]), "/"))
		}

		t = sc.NextSkipComments()
		lastLine = t.Line

		previous = t.Kind
		switch t.Kind {
		case TokLBrace, TokLBracket, TokLParen:
			depth++
		case TokRBrace, TokRBracket, TokRParen:
			depth--
		default:
		}
	}
}

func (s *mappingSourceState) assignment(target, key, expr, dir string) {
	if strings.EqualFold(target, "this.mappings") && key == "" {
		entries, _ := mappingLiteral(expr)
		// Evaluate the whole RHS against the old struct, before replacing it.
		// Iteration order cannot make a sibling literal key visible prematurely.
		values := map[string]string{}
		unknownRoot := false

		for k, expression := range entries {
			if value, ok := evalPathExpr(expression, s.env, dir); ok && k != "" {
				values[strings.ToLower(strings.Trim(k, "/"))] = value
			} else if k == "/" {
				unknownRoot = true
			}
		}

		for name := range s.env {
			if strings.HasPrefix(name, "mapping:") {
				delete(s.env, name)
			}
		}

		s.mappings = map[string]string{}

		for name, value := range values {
			s.mappings[name] = cleanMappingPath(value, dir)
			s.env["mapping:"+name] = value
		}

		if unknownRoot {
			s.mappings[""] = ""
		}

		return
	}

	if strings.EqualFold(target, "this.mappings") {
		s.mapping(key, expr, dir)

		return
	}

	if hasPrefixFold(target, "this.mappings.") {
		s.mapping(target[len("this.mappings."):], expr, dir)

		return
	}

	if key != "" {
		return
	}

	value, ok := evalPathExpr(expr, s.env, dir)
	if ok {
		s.env[envName(target)] = value
	} else {
		delete(s.env, envName(target))
	}
}

func (s *mappingSourceState) mapping(key, expr, dir string) {
	if key == "" {
		return
	}

	key = strings.ToLower(strings.Trim(key, "/"))

	value, ok := evalPathExpr(expr, s.env, dir)
	if !ok {
		delete(s.mappings, key)

		if key == "" {
			s.mappings[key] = ""
		}

		delete(s.env, "mapping:"+strings.ToLower(key))

		return
	}

	s.mappings[key] = cleanMappingPath(value, dir)
	s.env["mapping:"+strings.ToLower(key)] = value
}

func mappingLiteral(expr string) (map[string]string, bool) {
	if emptyCollection(expr) {
		return map[string]string{}, true
	}

	sc := NewScanner(expr)
	if sc.NextSkipComments().Kind != TokLBrace {
		return nil, false
	}

	out := map[string]string{}

	for sc.PeekSkipComments().Kind != TokRBrace {
		key := sc.NextSkipComments()
		if key.Kind != TokIdent && key.Kind != TokString {
			return nil, false
		}

		separator := sc.NextSkipComments()
		if separator.Kind != TokColon && separator.Kind != TokEquals {
			return nil, false
		}

		start := sc.PeekSkipComments().Offset
		depth := 0

		for {
			t := sc.PeekSkipComments()
			if t.Kind == TokEOF {
				return nil, false
			}

			if depth == 0 && (t.Kind == TokComma || t.Kind == TokRBrace) {
				out[strings.Trim(key.Value, `"'`)] = strings.TrimSpace(expr[start:t.Offset])

				break
			}

			sc.NextSkipComments()

			switch t.Kind {
			case TokLParen, TokLBrace, TokLBracket:
				depth++
			case TokRParen, TokRBrace, TokRBracket:
				depth--
			default:
			}
		}

		if sc.PeekSkipComments().Kind == TokComma {
			sc.NextSkipComments()
		}
	}

	sc.NextSkipComments()

	return out, sc.NextSkipComments().Kind == TokEOF
}

func (s *mappingSourceState) includePath(raw, file string) string {
	if !strings.HasPrefix(raw, "/") {
		return filepath.Join(filepath.Dir(file), filepath.FromSlash(raw))
	}

	for prefix := strings.TrimPrefix(raw, "/"); prefix != ""; {
		root, ok := s.mappings[strings.ToLower(prefix)]
		if !ok {
			root, ok = s.defaults[strings.ToLower(prefix)]
		}

		if ok {
			return filepath.Join(root, strings.TrimPrefix(strings.TrimPrefix(raw, "/"), prefix))
		}

		index := strings.LastIndexByte(prefix, '/')
		if index < 0 {
			break
		}

		prefix = prefix[:index]
	}

	return filepath.Join(filepath.Dir(s.base), filepath.FromSlash(strings.TrimPrefix(raw, "/")))
}

// Path slicing is common in shared application templates. Only literal length
// arithmetic on an already known path is accepted.
func slicedMappingPath(term string, env map[string]string, dir string) (string, bool) {
	for _, fn := range []string{"left", "right"} {
		args, ok := callArg(term, fn)
		if !ok {
			continue
		}

		parts := splitPathArguments(args)
		if len(parts) != 2 {
			return "", false
		}

		value, ok := evalPathExpr(parts[0], env, dir)
		if !ok {
			return "", false
		}

		n, ok := mappingLength(parts[1], env, dir)

		runes := []rune(value)
		if !ok || n < 0 || n > len(runes) {
			return "", false
		}

		if fn == "left" {
			return string(runes[:n]), true
		}

		return string(runes[len(runes)-n:]), true
	}

	return "", false
}

func mappingLength(expr string, env map[string]string, dir string) (int, bool) {
	expr = strings.TrimSpace(expr)
	if n, err := strconv.Atoi(expr); err == nil {
		return n, true
	}

	index := strings.LastIndexAny(expr, "+-")
	delta := 0

	if index >= 0 {
		n, err := strconv.Atoi(strings.TrimSpace(expr[index+1:]))
		if err != nil {
			return 0, false
		}

		delta = n
		if expr[index] == '-' {
			delta = -n
		}

		expr = strings.TrimSpace(expr[:index])
	}

	arg, ok := callArg(expr, "len")
	if !ok {
		return 0, false
	}

	value, ok := evalPathExpr(arg, env, dir)

	return len([]rune(value)) + delta, ok
}

func splitPathArguments(args string) []string {
	sc := NewScanner(args)
	start, depth := 0, 0

	var out []string

	for {
		t := sc.NextSkipComments()
		if t.Kind == TokEOF {
			return append(out, args[start:])
		}

		if t.Kind == TokComma && depth == 0 {
			out = append(out, args[start:t.Offset])
			start = t.Offset + 1
		}

		switch t.Kind {
		case TokLParen:
			depth++
		case TokRParen:
			depth--
		default:
		}
	}
}
