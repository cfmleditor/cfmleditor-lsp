package resolve

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// TestEveryAcceptPathRecordsATarget is the structural guard described on
// [callTrace]: canResolveCall accepts a call by returning "" from one of twenty-odd
// places, and every one of them must first tell the recorder where the call landed.
//
// A behavioural test cannot cover this — it would only catch the accept paths whose
// absence it happens to assert, and the ones nobody thought to write a fixture for
// are exactly the ones that get added later and forgotten. So this reads the source
// instead: find `return ""` inside canResolveCall, and require a tr.hit(...) already
// reached in the same block or an enclosing one.
//
// A missing hit is not a crash — the target comes back TargetNone and the call graph
// silently loses an edge it should have drawn. That is precisely the failure mode
// worth a structural test.
func TestEveryAcceptPathRecordsATarget(t *testing.T) {
	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, "resolve.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing resolve.go: %v", err)
	}

	fn := findFunc(file, "canResolveCall")
	if fn == nil {
		t.Fatal("canResolveCall not found in resolve.go — did it move or get renamed?")
	}

	var missing []string

	walkAccepts(fn.Body, false, func(pos token.Pos, hitSeen bool) {
		if !hitSeen {
			missing = append(missing, fset.Position(pos).String())
		}
	})

	if len(missing) > 0 {
		t.Errorf("%d accept path(s) in canResolveCall return \"\" without calling tr.hit first:\n  %s\n\n"+
			"Each `return \"\"` accepts a call, so the call graph needs to know what it accepted it as. "+
			"Add a tr.hit(Target..., component, def) before the return — see the kinds in target.go.",
			len(missing), strings.Join(missing, "\n  "))
	}

	// Guard the guard: a test that found nothing to check would pass forever.
	var accepts int

	walkAccepts(fn.Body, false, func(token.Pos, bool) { accepts++ })

	if accepts < 15 {
		t.Errorf("only found %d `return \"\"` accept paths in canResolveCall; expected at least 15 — "+
			"the walk is probably no longer finding them", accepts)
	}
}

// TestNoTargetKindIsUnclassified pins Definite against the full kind list, so a kind
// added without a case falls into the default arm rather than silently reporting
// itself as definite.
func TestNoTargetKindIsUnclassified(t *testing.T) {
	definite := map[TargetKind]bool{
		TargetNone:      false,
		TargetSameFile:  true,
		TargetExtends:   true,
		TargetComponent: true,
		TargetDynamic:   false,
		TargetBuiltin:   false,
		TargetMember:    false,
	}

	for kind, want := range definite {
		if got := kind.Definite(); got != want {
			t.Errorf("TargetKind(%q).Definite() = %v, want %v", kind, got, want)
		}
	}

	if TargetKind("something-new").Definite() {
		t.Error("an unrecognised TargetKind reports itself definite; the default arm must return false")
	}
}

func findFunc(file *ast.File, name string) *ast.FuncDecl {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == name {
			return fn
		}
	}

	return nil
}

// walkAccepts calls report for every `return ""` in the block, saying whether a
// tr.hit(...) was already reached on the path to it. hitSeen carries down into
// nested blocks because a hit in an enclosing block covers the returns beneath it.
func walkAccepts(block *ast.BlockStmt, hitSeen bool, report func(token.Pos, bool)) {
	if block == nil {
		return
	}

	for _, stmt := range block.List {
		if isHitCall(stmt) {
			hitSeen = true

			continue
		}

		if isEmptyStringReturn(stmt) {
			report(stmt.Pos(), hitSeen)

			continue
		}

		for _, nested := range nestedBlocks(stmt) {
			walkAccepts(nested, hitSeen, report)
		}
	}
}

func nestedBlocks(stmt ast.Stmt) []*ast.BlockStmt {
	switch s := stmt.(type) {
	case *ast.IfStmt:
		blocks := []*ast.BlockStmt{s.Body}
		switch e := s.Else.(type) {
		case *ast.BlockStmt:
			blocks = append(blocks, e)
		case *ast.IfStmt:
			blocks = append(blocks, &ast.BlockStmt{List: []ast.Stmt{e}})
		}

		return blocks
	case *ast.ForStmt:
		return []*ast.BlockStmt{s.Body}
	case *ast.RangeStmt:
		return []*ast.BlockStmt{s.Body}
	case *ast.BlockStmt:
		return []*ast.BlockStmt{s}
	case *ast.SwitchStmt:
		return []*ast.BlockStmt{s.Body}
	case *ast.TypeSwitchStmt:
		return []*ast.BlockStmt{s.Body}
	case *ast.CaseClause:
		return []*ast.BlockStmt{{List: s.Body}}
	default:
		return nil
	}
}

func isHitCall(stmt ast.Stmt) bool {
	expr, ok := stmt.(*ast.ExprStmt)
	if !ok {
		return false
	}

	call, ok := expr.X.(*ast.CallExpr)
	if !ok {
		return false
	}

	sel, ok := call.Fun.(*ast.SelectorExpr)

	return ok && sel.Sel.Name == "hit"
}

func isEmptyStringReturn(stmt ast.Stmt) bool {
	ret, ok := stmt.(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return false
	}

	lit, ok := ret.Results[0].(*ast.BasicLit)

	return ok && lit.Kind == token.STRING && lit.Value == `""`
}
