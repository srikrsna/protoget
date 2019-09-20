package protoget

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"go/types"
	"reflect"

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
	pointers := map[ast.Expr]bool{}
	ins.Preorder([]ast.Node{(*ast.AssignStmt)(nil), (*ast.UnaryExpr)(nil)}, func(n ast.Node) {
		if a, ok := n.(*ast.AssignStmt); ok && a.Tok != token.DEFINE {
			assignments = append(assignments, a)
			return
		}

		if p, ok := n.(*ast.UnaryExpr); ok && p.Op == token.AND {
			pointers[p.X] = true
		}
	})
	ins.Preorder([]ast.Node{(*ast.SelectorExpr)(nil)}, func(n ast.Node) {
		sel := n.(*ast.SelectorExpr)
		typ := pass.TypesInfo.Types[sel.X].Type

		if typ == nil { // Packages
			return
		}

		if !types.Implements(typ, protoType) {
			return
		}

		st := ((typ.(*types.Pointer)).Elem().(*types.Named)).Underlying().(*types.Struct)
		stop := true
		for i := 0; i < st.NumFields(); i++ {
			if st.Field(i).Name() == sel.Sel.Name {
				stop = false
				break
			}
		}

		if stop {
			return
		}

		if isLHS(assignments, pointers, sel) {
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

var callType = reflect.TypeOf((*ast.CallExpr)(nil))

func isLHS(assignments []*ast.AssignStmt, pointers map[ast.Expr]bool, sel *ast.SelectorExpr) bool {
	temp := reflect.ValueOf(sel)
	old := temp

	for {
		if temp.Elem().Type().Kind() != reflect.Struct {
			break
		}

		if temp.Type() == callType {
			old = temp
			temp = temp.Elem().FieldByName("Fun")
			continue
		}

		if _, ok := temp.Elem().Type().FieldByName("X"); !ok {
			break
		} else {
			old = temp
			temp = temp.Elem().FieldByName("X")
		}
	}

	i := old.Interface()

	if pointers[i.(ast.Expr)] || pointers[temp.Interface().(ast.Expr)] {
		return true
	}

	for _, a := range assignments {
		for _, lh := range a.Lhs {
			if lh == i {
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
