package publicleak

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/ddd-qce/core/lint/convention"
)

var Analyzer = &analysis.Analyzer{
	Name:     "dddpublicleak",
	Doc:      "check that public DDD packages do not reference internal domain types",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	currentInfo := convention.ParsePkgPath(pass.Pkg.Path())
	if currentInfo == nil || currentInfo.Kind != convention.KindPublic {
		return nil, nil
	}

	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{
		(*ast.TypeSpec)(nil),
		(*ast.FuncDecl)(nil),
	}

	insp.Preorder(nodeFilter, func(n ast.Node) {
		switch node := n.(type) {
		case *ast.TypeSpec:
			if !node.Name.IsExported() {
				return
			}
			structType, ok := node.Type.(*ast.StructType)
			if !ok {
				return
			}
			if structType.Fields == nil {
				return
			}
			for _, field := range structType.Fields.List {
				checkTypeExpr(pass, field.Type, node.Name.Name)
			}
		case *ast.FuncDecl:
			if !node.Name.IsExported() {
				return
			}
			if node.Type.Params != nil {
				for _, field := range node.Type.Params.List {
					checkTypeExpr(pass, field.Type, node.Name.Name)
				}
			}
			if node.Type.Results != nil {
				for _, field := range node.Type.Results.List {
					checkTypeExpr(pass, field.Type, node.Name.Name)
				}
			}
		}
	})

	return nil, nil
}

func checkTypeExpr(pass *analysis.Pass, expr ast.Expr, contextName string) {
	t := pass.TypesInfo.TypeOf(expr)
	if t == nil {
		return
	}

	currentInfo := convention.ParsePkgPath(pass.Pkg.Path())
	if refersToCrossDomainInternal(t, currentInfo) {
		pass.Reportf(expr.Pos(),
			"dddpublicleak: type in %q references internal domain package from another domain; "+
				"use scalar fields and map domain objects to Result types in handler",
			contextName)
	}
}

func refersToCrossDomainInternal(t types.Type, currentInfo *convention.PkgInfo) bool {
	switch tt := t.(type) {
	case *types.Named:
		pkg := tt.Obj().Pkg()
		if pkg == nil {
			return false
		}
		info := convention.ParsePkgPath(pkg.Path())
		if info == nil {
			return false
		}
		if info.Kind != convention.KindInternal {
			return false
		}
		if currentInfo.DDDPrefix == info.DDDPrefix && currentInfo.DomainName == info.DomainName {
			return false
		}
		return true
	case *types.Pointer:
		return refersToCrossDomainInternal(tt.Elem(), currentInfo)
	case *types.Slice:
		return refersToCrossDomainInternal(tt.Elem(), currentInfo)
	case *types.Array:
		return refersToCrossDomainInternal(tt.Elem(), currentInfo)
	case *types.Map:
		return refersToCrossDomainInternal(tt.Key(), currentInfo) || refersToCrossDomainInternal(tt.Elem(), currentInfo)
	case *types.Chan:
		return refersToCrossDomainInternal(tt.Elem(), currentInfo)
	case interface{ Elem() types.Type }:
		return refersToCrossDomainInternal(tt.Elem(), currentInfo)
	default:
		return false
	}
}
