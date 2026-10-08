package resolve

import (
	"path/filepath"
	"strings"

	"github.com/cfmleditor/clif/internal/parser"
	cfpath "github.com/cfmleditor/clif/internal/path"
)

// Only assignments to this expose a field on another component. Variables
// fields and generated getX() methods do not imply a public x field. An
// unknown/changed write shadows an inherited field rather than borrowing it.
func (r *Resolver) publicPropertyComponent(component, name, baseDir string) string {
	path := r.ComponentPath(component, baseDir)

	seen := make(map[string]bool)
	for len(seen) < 8 && path != "" && !seen[path] {
		seen[path] = true
		r.EnsureIndexed(path)
		refs := r.Index.RefsForFile(cfpath.ToURI(path))

		data, err := r.fs().ReadFile(path)
		if err != nil {
			return ""
		}

		writes := publicPropertyWrites(string(data), name)
		if len(writes) > 0 {
			value := ""

			for line, count := range writes {
				if count != 1 {
					return "$any"
				}

				atLine := ""

				for _, ref := range refs {
					if !ref.This || !strings.EqualFold(ref.Variable, name) || int(ref.Line) != line {
						continue
					}

					if atLine != "" && !strings.EqualFold(atLine, ref.Component) {
						return "$any"
					}

					atLine = ref.Component
				}

				if atLine == "" || value != "" && !strings.EqualFold(value, atLine) {
					return "$any"
				}

				value = atLine
			}

			return value
		}

		next, ok := r.extendsOf(path, cfpath.ToURI(path))
		if !ok {
			return ""
		}

		path = r.ComponentPath(next, filepath.Dir(path))
	}

	return ""
}

func publicPropertyWrites(content, name string) map[int]int {
	writes := make(map[int]int)

	sc := parser.NewScanner(content)
	for {
		tok := sc.NextSkipComments()
		if tok.Kind == parser.TokEOF {
			return writes
		}

		if tok.Kind != parser.TokIdent || !strings.EqualFold(tok.Value, "this") {
			continue
		}

		state := sc.Save()
		field := ""

		switch sc.NextSkipComments().Kind {
		case parser.TokDot:
			member := sc.NextSkipComments()
			if member.Kind == parser.TokIdent {
				field = member.Value
			}
		case parser.TokLBracket:
			member := sc.NextSkipComments()
			if member.Kind == parser.TokString && !strings.Contains(member.Value, "#") {
				field = strings.Trim(member.Value, `"'`)
			} else {
				field = name
			}

			depth := 1
			for depth > 0 {
				token := sc.NextSkipComments()
				switch token.Kind {
				case parser.TokEOF:
					return writes
				case parser.TokLBracket:
					depth++
				case parser.TokRBracket:
					depth--
				default:
				}
			}
		default:
		}

		if strings.EqualFold(field, name) && sc.NextSkipComments().Kind == parser.TokEquals && sc.PeekSkipComments().Kind != parser.TokEquals {
			writes[tok.Line]++
		}

		sc.Restore(state)
	}
}
