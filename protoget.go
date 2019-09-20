package protoget

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer analyses that all proto message fields are accessed using
// generated getters and returns any false cases.
var Analyzer = &analysis.Analyzer{
	Name:             "protoget",
	Doc:              "Checks for any directly accesed fields on proto message",
	Run:              run,
	RunDespiteErrors: true,
	Requires:         []*analysis.Analyzer{inspect.Analyzer},
}

func run(pass *analysis.Pass) (interface{}, error) {
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	var assignments []*ast.AssignStmt
	ins.Preorder([]ast.Node{(*ast.AssignStmt)(nil)}, func(n ast.Node) {
		assignments = append(assignments, n.(*ast.AssignStmt))
	})
	ins.Preorder([]ast.Node{(*ast.SelectorExpr)(nil)}, func(n ast.Node) {
		sel := n.(*ast.SelectorExpr)
		typ := pass.TypesInfo.Types[sel].Type

		if !types.Implements(typ, protoType) {
			return
		}

		if isLHS(assignments, sel) {
			return
		}

		pass.Report(analysis.Diagnostic{
			Pos:     sel.Pos(),
			End:     sel.End(),
			Message: fmt.Sprintf("protoget: %q", render(pass.Fset, sel)),
			SuggestedFixes: []analysis.SuggestedFix{
				{
					Message: "User the getter instead",
					TextEdits: []analysis.TextEdit{
						{
							Pos:     sel.Sel.Pos(),
							End:     sel.Sel.End(),
							NewText: []byte("Get" + sel.Sel.Name + "()"),
						},
					},
				},
			},
		})
	})
	return nil, nil
}

func isLHS(assignments []*ast.AssignStmt, sel *ast.SelectorExpr) bool {
	temp := sel
loop:
	for {
		switch v := temp.X.(type) {
		case *ast.Ident:
			break loop
		case *ast.SelectorExpr:
			temp = v
		}
	}

	for _, a := range assignments {
		for _, lh := range a.Lhs {
			if lh == temp {
				return true
			}
		}
	}

	return false
}

func render(fset *token.FileSet, x interface{}) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, x); err != nil {
		panic(err)
	}
	return buf.String()
}

var protoType *types.Interface

func init() {
	emptyFunc := types.NewSignature(nil, nil, nil, false)
	methods := []*types.Func{
		types.NewFunc(token.NoPos, nil, "ProtoMessage", emptyFunc),
	}
	protoType = types.NewInterface(methods, nil).Complete()
}
