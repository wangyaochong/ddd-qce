package wirelogic

import (
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/ddd-qce/core/lint/convention"
)

var Analyzer = &analysis.Analyzer{
	Name: "dddwirelogic",
	Doc:  "check that wire packages do not contain business logic",
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	currentInfo := convention.ParsePkgPath(pass.Pkg.Path())
	if currentInfo == nil {
		return nil, nil
	}
	if currentInfo.Kind != convention.KindWire {
		return nil, nil
	}

	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			funcDecl, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}

			if funcDecl.Body == nil {
				continue
			}

			checkFunction(pass, funcDecl)
		}
	}

	return nil, nil
}

func checkFunction(pass *analysis.Pass, funcDecl *ast.FuncDecl) {
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.ForStmt:
			pass.Reportf(node.Pos(),
				"dddwirelogic: loop statement in wire function %q; "+
					"wire functions should not contain loops",
				funcDecl.Name.Name)

		case *ast.RangeStmt:
			pass.Reportf(node.Pos(),
				"dddwirelogic: range loop in wire function %q; "+
					"wire functions should not contain loops",
				funcDecl.Name.Name)

		case *ast.SwitchStmt:
			pass.Reportf(node.Pos(),
				"dddwirelogic: switch statement in wire function %q; "+
					"wire functions should not contain switch statements",
				funcDecl.Name.Name)

		case *ast.TypeSwitchStmt:
			pass.Reportf(node.Pos(),
				"dddwirelogic: type switch in wire function %q; "+
					"wire functions should not contain type switches",
				funcDecl.Name.Name)

		case *ast.SelectStmt:
			pass.Reportf(node.Pos(),
				"dddwirelogic: select statement in wire function %q; "+
					"wire functions should not contain select statements",
				funcDecl.Name.Name)

		case *ast.IfStmt:
			// Allow simple error checking: if err != nil { return err }
			if !isSimpleErrorCheck(node) {
				pass.Reportf(node.Pos(),
					"dddwirelogic: if statement in wire function %q; "+
						"wire functions should only contain simple error checking (if err != nil { return err })",
					funcDecl.Name.Name)
			}

		case *ast.CallExpr:
			// Check for domain object creation
			if isDomainObjectCreation(node) {
				pass.Reportf(node.Pos(),
					"dddwirelogic: domain object creation in wire function %q; "+
						"wire functions should only wire dependencies, not create domain objects",
					funcDecl.Name.Name)
			}
		}

		return true
	})
}

func isSimpleErrorCheck(ifStmt *ast.IfStmt) bool {
	// Check pattern: if err != nil { return ... }
	binExpr, ok := ifStmt.Cond.(*ast.BinaryExpr)
	if !ok {
		return false
	}

	// Check for err != nil
	if binExpr.Op != token.NEQ {
		return false
	}

	// Check if one side is nil
	if !isNil(binExpr.X) && !isNil(binExpr.Y) {
		return false
	}

	// Check if body only contains return
	if len(ifStmt.Body.List) != 1 {
		return false
	}

	_, isReturn := ifStmt.Body.List[0].(*ast.ReturnStmt)
	return isReturn
}

func isNil(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == "nil"
}

func isDomainObjectCreation(callExpr *ast.CallExpr) bool {
	// Check for patterns like:
	// - domain.NewXxx(...)
	// - &domain.Xxx{...}
	// - domain.CreateXxx(...)

	funIdent, ok := callExpr.Fun.(*ast.Ident)
	if ok {
		name := funIdent.Name
		// Check for New*, Create* patterns that might be domain object creation
		if strings.HasPrefix(name, "New") || strings.HasPrefix(name, "Create") {
			// This is a heuristic - could be a wiring function too
			// We'll be conservative and not flag these
			return false
		}
	}

	// Check for qualified calls like domain.NewXxx(...)
	selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	// Check if the selector is on a domain package
	xIdent, ok := selExpr.X.(*ast.Ident)
	if !ok {
		return false
	}

	// Check for domain.New*, domain.Create* patterns
	name := selExpr.Sel.Name
	if strings.HasPrefix(name, "New") || strings.HasPrefix(name, "Create") {
		// Check if the package name looks like a domain package
		pkgName := xIdent.Name
		if pkgName == "domain" || strings.HasSuffix(pkgName, "domain") {
			return true
		}
	}

	return false
}
