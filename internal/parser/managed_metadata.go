package parser

import "strings"

// Read declarations without a second signature, reference or body parse.
func (pr *ParseResult) managedPropertyMetadata() *ParseResult {
	metadata := &ParseResult{}

	for _, region := range pr.Regions {
		switch region.Kind {
		case RegionScript:
			pr.managedScriptMetadata(region, metadata)
		case RegionTag:
			pr.managedTagMetadata(region, metadata)
		default:
		}
	}

	return metadata
}

func (pr *ParseResult) managedScriptMetadata(region Region, metadata *ParseResult) {
	sp := newScriptParser(region.Text, string(pr.URI), region.StartLine, nil)
	depth, top := 0, 0

	for {
		tok := sp.sc.NextSkipComments()
		if tok.Kind == TokEOF {
			break
		}

		switch tok.Kind {
		case TokLBrace:
			depth++
		case TokRBrace:
			depth--
		case TokIdent:
			if depth != top {
				continue
			}

			if strings.EqualFold(tok.Value, "component") {
				sp.parseComponentAttrs()

				top = depth + 1
			}

			if strings.EqualFold(tok.Value, "property") {
				sp.parseProperty(tok)
			}
		default:
		}
	}

	metadata.Properties = append(metadata.Properties, sp.properties...)
	metadata.Accessors = metadata.Accessors || sp.accessors
	metadata.Persistent = metadata.Persistent || sp.persistent
}

func (pr *ParseResult) managedTagMetadata(region Region, metadata *ParseResult) {
	tp := newTagParser(region.Text, string(pr.URI))

	scanner := NewScanner(region.Text)
	for {
		tok := scanner.NextSkipComments()
		if tok.Kind == TokEOF {
			break
		}

		if tok.Kind != TokLT {
			continue
		}

		word := scanner.NextSkipComments()
		if word.Kind != TokIdent {
			continue
		}

		if !strings.EqualFold(word.Value, "cfcomponent") && !strings.EqualFold(word.Value, "cfproperty") {
			continue
		}

		end := tagEndIndex(region.Text[tok.Offset:])
		if end < 0 {
			continue
		}

		tag := region.Text[tok.Offset : tok.Offset+end+1]
		if strings.EqualFold(word.Value, "cfproperty") {
			tp.parseCFProperty(tag, region.StartLine+tok.Line)
		} else {
			metadata.Accessors = metadata.Accessors || isTruthy(getAttr(tag, "accessors"))
			metadata.Persistent = metadata.Persistent || isTruthy(getAttr(tag, "persistent"))
		}
	}

	metadata.Properties = append(metadata.Properties, tp.properties...)
}
