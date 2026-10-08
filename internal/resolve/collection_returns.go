package resolve

import (
	"path/filepath"
	"strings"

	"github.com/cfmleditor/clif/internal/parser"
	cfpath "github.com/cfmleditor/clif/internal/path"
)

// Compact source contracts let closed indexing defer cross-component method
// lookup. Following them uses normal indexed lookup and a shared work budget.
func (r *Resolver) collectionReturnOf(fd *parser.FunctionDef, depth int, budget *int) string {
	baseDir := filepath.Dir(cfpath.FromURI(string(fd.URI)))
	answer := ""

	for _, source := range fd.ReturnSources {
		if *budget <= 0 {
			return ""
		}

		*budget--

		comp := source.Component
		if path := r.ComponentPath(comp, baseDir); path != "" {
			comp = path
		}

		for _, method := range source.Methods {
			expression, callHop := parser.CallExpression(method)
			if callHop {
				method = wheelsCallName(expression)
			}

			if *budget <= 0 {
				return ""
			}

			*budget--

			if strings.EqualFold(method, "init") {
				continue
			}

			called := r.ResolveFunc(comp, method, baseDir)
			if called == nil {
				return ""
			}

			ret := r.returnComponentOf(called, depth+1, budget)
			if (ret == "" || ret == "$any") && callHop {
				ret = r.producerExpressionValue(called, expression, baseDir, comp, depth+1, budget).component()
			}

			if ret == "" {
				ret = r.receiverReturn(r.ComponentPath(comp, baseDir), method)
			}

			if ret == "" || strings.HasPrefix(ret, "$") {
				return ""
			}

			if self := r.selfTyped(comp, baseDir, called, ret); self != "" {
				ret = self
			}

			comp = ret
		}

		if path := r.ComponentPath(comp, baseDir); path != "" {
			comp = path
		}

		if comp == "" || strings.HasPrefix(comp, "$") || answer != "" && !strings.EqualFold(answer, comp) {
			return ""
		}

		answer = comp
	}

	return answer
}
