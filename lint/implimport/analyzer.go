package implimport

import (
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/ddd-qce/core/lint/convention"
)

var Analyzer = &analysis.Analyzer{
	Name: "dddimplimport",
	Doc:  "check that CQRS implementation packages are only imported from wire layer",
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	currentInfo := convention.ParsePkgPath(pass.Pkg.Path())
	if currentInfo == nil {
		return nil, nil
	}
	if currentInfo.Kind == convention.KindWire {
		return nil, nil
	}

	for _, file := range pass.Files {
		for _, imp := range file.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			if isImplPackage(importPath) {
				pass.Reportf(imp.Pos(),
					"dddimplimport: import of implementation package %q is forbidden outside wire layer; "+
						"use interface packages (cqrs/command, cqrs/query, cqrs/event) instead",
					importPath)
			}
		}
	}

	return nil, nil
}

func isImplPackage(importPath string) bool {
	return strings.Contains(importPath, "/cqrs/impl/")
}
