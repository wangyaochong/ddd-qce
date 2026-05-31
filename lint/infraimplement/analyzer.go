package infraimplement

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/ddd-qce/core/lint/convention"
)

var Analyzer = &analysis.Analyzer{
	Name: "dddinfraimplement",
	Doc:  "check that infra packages correctly implement repository interfaces",
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	currentInfo := convention.ParsePkgPath(pass.Pkg.Path())
	if currentInfo == nil {
		return nil, nil
	}

	// Only check infra packages
	if !isInfraPackage(currentInfo.SubPkg) {
		return nil, nil
	}

	// Find repository interfaces from the same domain
	repoInterfaces := findRepositoryInterfaces(pass)

	// Check implementations
	for _, iface := range repoInterfaces {
		checkImplementation(pass, iface)
	}

	return nil, nil
}

func isInfraPackage(subPkg string) bool {
	return strings.HasPrefix(subPkg, "infra")
}

type repoInterface struct {
	Name    string
	Methods map[string]*types.Func
	PkgPath string
}

func findRepositoryInterfaces(pass *analysis.Pass) []repoInterface {
	var interfaces []repoInterface

	// Look through all imports for repository packages
	for _, imp := range pass.Pkg.Imports() {
		info := convention.ParsePkgPath(imp.Path())
		if info == nil {
			continue
		}

		// Only check repository packages from the same domain
		if info.Kind != convention.KindInternal || info.SubPkg != "repository" {
			continue
		}

		// Check if same domain
		currentInfo := convention.ParsePkgPath(pass.Pkg.Path())
		if currentInfo == nil {
			continue
		}
		if currentInfo.DDDPrefix != info.DDDPrefix || currentInfo.DomainName != info.DomainName {
			continue
		}

		// Find interfaces in the repository package
		scope := imp.Scope()
		for _, name := range scope.Names() {
			obj := scope.Lookup(name)
			if iface, ok := obj.Type().Underlying().(*types.Interface); ok {
				methods := make(map[string]*types.Func)
				for i := 0; i < iface.NumMethods(); i++ {
					method := iface.Method(i)
					methods[method.Name()] = method
				}
				interfaces = append(interfaces, repoInterface{
					Name:    name,
					Methods: methods,
					PkgPath: imp.Path(),
				})
			}
		}
	}

	return interfaces
}

func checkImplementation(pass *analysis.Pass, iface repoInterface) {
	// Find structs in the current package that might implement the interface
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}

			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}

				_, ok = typeSpec.Type.(*ast.StructType)
				if !ok {
					continue
				}

				// Check if this struct implements the interface
				structObj := pass.Pkg.Scope().Lookup(typeSpec.Name.Name)
				if structObj == nil {
					continue
				}

				structPtr := types.NewPointer(structObj.Type())
				if !types.Implements(structPtr, getInterfaceType(pass, iface)) {
					continue
				}

				// Check all methods are implemented
				for methodName, method := range iface.Methods {
					if !hasMethod(pass, typeSpec.Name.Name, methodName) {
						pass.Reportf(typeSpec.Pos(),
							"dddinfraimplement: struct %q does not implement method %q from interface %q",
							typeSpec.Name.Name, methodName, iface.Name)
					}
					_ = method
				}
			}
		}
	}
}

func getInterfaceType(pass *analysis.Pass, iface repoInterface) *types.Interface {
	for _, imp := range pass.Pkg.Imports() {
		if imp.Path() != iface.PkgPath {
			continue
		}
		scope := imp.Scope()
		obj := scope.Lookup(iface.Name)
		if obj != nil {
			if ifaceType, ok := obj.Type().Underlying().(*types.Interface); ok {
				return ifaceType
			}
		}
	}
	return nil
}

func hasMethod(pass *analysis.Pass, structName, methodName string) bool {
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			funcDecl, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}

			if funcDecl.Recv == nil {
				continue
			}

			// Check if receiver is the struct
			recvType := funcDecl.Recv.List[0].Type
			if isStructReceiver(recvType, structName) {
				if funcDecl.Name.Name == methodName {
					return true
				}
			}
		}
	}
	return false
}

func isStructReceiver(expr ast.Expr, structName string) bool {
	switch t := expr.(type) {
	case *ast.StarExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name == structName
		}
	case *ast.Ident:
		return t.Name == structName
	}
	return false
}
