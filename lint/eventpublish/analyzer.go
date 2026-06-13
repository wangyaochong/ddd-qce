package eventpublish

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const eventBusInterface = "EventBus"

var Analyzer = &analysis.Analyzer{
	Name:     "dddeventpublish",
	Doc:      "check that EventBus.Publish and PublishEvent errors are not silently discarded",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	nodeFilter := []ast.Node{
		(*ast.ExprStmt)(nil),
	}

	insp.Preorder(nodeFilter, func(n ast.Node) {
		stmt := n.(*ast.ExprStmt)
		call, ok := stmt.X.(*ast.CallExpr)
		if !ok {
			return
		}

		if !isPublishCall(pass, call) {
			return
		}

		pass.Reportf(call.Pos(),
			"dddeventpublish: return value of %s is not checked; "+
				"event publish errors must be propagated, not silently discarded",
			callStr(call))
	})
	return nil, nil
}

func isPublishCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	fn, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	if fn.Sel.Name != "Publish" && fn.Sel.Name != "PublishEvent" {
		return false
	}

	sig, ok := pass.TypesInfo.TypeOf(call.Fun).(*types.Signature)
	if !ok || sig.Results().Len() == 0 {
		return false
	}

	last := sig.Results().At(sig.Results().Len() - 1)
	if !isErrorType(last.Type()) {
		return false
	}

	if fn.Sel.Name == "PublishEvent" {
		return true
	}

	return implementsEventBus(sig.Recv())
}

func isErrorType(t types.Type) bool {
	named, ok := t.(*types.Named)
	if ok {
		return named.Obj().Name() == "error" && named.Obj().Pkg() == nil
	}
	iface, ok := t.(*types.Interface)
	if ok {
		return iface.NumMethods() == 1 && iface.NumEmbeddeds() == 0
	}
	return t.String() == "error"
}

func implementsEventBus(recv *types.Var) bool {
	if recv == nil {
		return false
	}
	named, ok := recv.Type().(*types.Named)
	if !ok {
		return false
	}
	for i := 0; i < named.NumMethods(); i++ {
		if named.Method(i).Name() == "Publish" {
			return named.Obj().Name() == eventBusInterface
		}
	}
	underlying := named.Underlying()
	iface, ok := underlying.(*types.Interface)
	if ok {
		for i := 0; i < iface.NumMethods(); i++ {
			if iface.Method(i).Name() == "Publish" {
				return true
			}
		}
	}
	ptr, ok := recv.Type().(*types.Pointer)
	if ok {
		elemNamed, ok := ptr.Elem().(*types.Named)
		if ok {
			return elemNamed.Obj().Name() == eventBusInterface
		}
	}
	return named.Obj().Name() == eventBusInterface
}

func callStr(call *ast.CallExpr) string {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok {
			return ident.Name + "." + sel.Sel.Name + "(...)"
		}
		return sel.Sel.Name + "(...)"
	}
	return "function(...)"
}
