package wirecomplexity

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"

	"github.com/ddd-qce/core/lint/convention"
)

var Analyzer = &analysis.Analyzer{
	Name: "dddwirecomplexity",
	Doc:  "check that wire packages are simple and only contain wiring functions",
	Run:  run,
}

const (
	maxFunctionLines = 80
	maxFunctions     = 20
	maxComplexity    = 25
)

func run(pass *analysis.Pass) (interface{}, error) {
	currentInfo := convention.ParsePkgPath(pass.Pkg.Path())
	if currentInfo == nil {
		return nil, nil
	}
	if currentInfo.Kind != convention.KindWire {
		return nil, nil
	}

	exportedFuncCount := 0

	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			funcDecl, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}

			if funcDecl.Name.IsExported() {
				exportedFuncCount++
			}

			// Check function length
			if funcDecl.Body != nil {
				startLine := pass.Fset.Position(funcDecl.Pos()).Line
				endLine := pass.Fset.Position(funcDecl.End()).Line
				lineCount := endLine - startLine + 1

				if lineCount > maxFunctionLines {
					pass.Reportf(funcDecl.Pos(),
						"dddwirecomplexity: function %q has %d lines, max allowed is %d; "+
							"keep wire functions simple",
						funcDecl.Name.Name, lineCount, maxFunctionLines)
				}

				// Check cyclomatic complexity
				complexity := calculateComplexity(funcDecl)
				if complexity > maxComplexity {
					pass.Reportf(funcDecl.Pos(),
						"dddwirecomplexity: function %q has complexity %d, max allowed is %d; "+
							"wire functions should have minimal branching",
						funcDecl.Name.Name, complexity, maxComplexity)
				}
			}
		}
	}

	// Check total exported functions
	if exportedFuncCount > maxFunctions {
		pass.Reportf(token.NoPos,
			"dddwirecomplexity: wire package has %d exported functions, max allowed is %d; "+
				"consider splitting into multiple wire packages",
			exportedFuncCount, maxFunctions)
	}

	return nil, nil
}

func calculateComplexity(funcDecl *ast.FuncDecl) int {
	complexity := 1

	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.IfStmt:
			complexity++
		case *ast.ForStmt:
			complexity++
		case *ast.RangeStmt:
			complexity++
		case *ast.SwitchStmt:
			complexity++
		case *ast.TypeSwitchStmt:
			complexity++
		case *ast.SelectStmt:
			complexity++
		case *ast.CaseClause:
			complexity++
		}
		return true
	})

	return complexity
}
