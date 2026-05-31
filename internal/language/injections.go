package language

import (
	_ "embed"
	"sync"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

//go:embed queries/injections.scm
var injectionsSource string

// InjectionMatch represents a matched injection point in a tree.
type InjectionMatch struct {
	Node     *sitter.Node
	Language string
}

var (
	injectionsQuery *sitter.Query
	injectionsOnce  sync.Once
)

func getInjectionsQuery() *sitter.Query {
	injectionsOnce.Do(func() {
		q, err := sitter.NewQuery(Language(CFML), injectionsSource)
		if err != nil {
			return
		}

		injectionsQuery = q
	})

	return injectionsQuery
}

// FindInjections runs the injections.scm query against the tree and returns
// all injection content nodes with their target language.
func FindInjections(tree *sitter.Tree, src []byte) []InjectionMatch {
	q := getInjectionsQuery()
	if q == nil {
		return nil
	}

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()

	matches := cursor.Matches(q, tree.RootNode(), src)
	captureNames := q.CaptureNames()

	var results []InjectionMatch

	for match := matches.Next(); match != nil; match = matches.Next() {
		// Get the injection.language from property settings
		var lang string

		for _, prop := range q.PropertySettings(match.PatternIndex) {
			if prop.Key == "injection.language" && prop.Value != nil {
				lang = *prop.Value
			}
		}

		if lang == "" {
			continue
		}

		// Find the injection.content capture
		for _, capture := range match.Captures {
			name := captureNames[capture.Index]
			if name == "injection.content" {
				results = append(results, InjectionMatch{
					Node:     &capture.Node,
					Language: lang,
				})
			}
		}
	}

	return results
}

// GrammarForLanguage maps an injection language name to a Grammar constant.
func GrammarForLanguage(lang string) Grammar {
	switch lang {
	case "cfscript":
		return CFScript
	case "cfquery":
		return CFQuery
	default:
		return -1
	}
}
