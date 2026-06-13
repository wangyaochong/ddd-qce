package fieldaccess

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/ddd-qce/core/lint/convention"
)

var protectedFieldPatterns = []struct {
	Suffix string
}{
	{Suffix: "Status"},
	{Suffix: "Mode"},
	{Suffix: "At"},
	{Suffix: "Error"},
	{Suffix: "ID"},
	{Suffix: "Index"},
	{Suffix: "Name"},
	{Suffix: "From"},
	{Suffix: "Reason"},
	{Suffix: "Output"},
}

func isProtectedField(name string) bool {
	switch name {
	case "ID", "Name", "PID":
		return false
	case "Status", "ErrorMessage", "StartedAt", "EndedAt", "CompletedAt":
		return true
	}
	for _, p := range protectedFieldPatterns {
		if strings.HasSuffix(name, p.Suffix) && len(name) > len(p.Suffix) {
			return true
		}
	}
	return false
}

var Analyzer = &analysis.Analyzer{
	Name:     "dddfieldaccess",
	Doc:      "check that domain entity state fields are not directly assigned outside the domain package",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	currentInfo := convention.ParsePkgPath(pass.Pkg.Path())
	if currentInfo == nil {
		return nil, nil
	}

	if currentInfo.Kind == convention.KindInternal && currentInfo.SubPkg == "domain" {
		return nil, nil
	}

	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	nodeFilter := []ast.Node{
		(*ast.AssignStmt)(nil),
	}

	insp.Preorder(nodeFilter, func(n ast.Node) {
		stmt := n.(*ast.AssignStmt)
		for _, lhs := range stmt.Lhs {
			sel, ok := lhs.(*ast.SelectorExpr)
			if !ok {
				continue
			}
			if !isProtectedField(sel.Sel.Name) {
				continue
			}
			selection, ok := pass.TypesInfo.Selections[sel]
			if !ok {
				continue
			}
			if !isDomainEntityField(selection) {
				continue
			}
			fieldOwnerPkg := selection.Obj().Pkg()
			if fieldOwnerPkg == nil {
				continue
			}
			if isExemptPkg(pass.Pkg.Path()) {
				continue
			}
			pass.Reportf(sel.Pos(),
				"dddfieldaccess: domain entity field %q must not be directly assigned outside the domain package; use a domain method instead",
				sel.Sel.Name)
		}
	})
	return nil, nil
}

func isDomainEntityField(selection *types.Selection) bool {
	if selection == nil {
		return false
	}
	if selection.Kind() != types.FieldVal {
		return false
	}
	obj := selection.Obj()
	if obj == nil {
		return false
	}
	pkg := obj.Pkg()
	if pkg == nil {
		return false
	}
	pkgPath := pkg.Path()
	return strings.Contains(pkgPath, "/ddd/") && strings.Contains(pkgPath, "/domain")
}

func isExemptPkg(currentPkgPath string) bool {
	if strings.HasSuffix(currentPkgPath, "_test") {
		return true
	}
	if strings.Contains(currentPkgPath, "/infra/pg") {
		return true
	}
	return false
}
