package crossmodule

import (
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/ddd-qce/core/lint/convention"
)

var Analyzer = &analysis.Analyzer{
	Name: "dddcrossmodule",
	Doc:  "check that DDD packages do not import internal packages from other modules",
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	currentInfo := convention.ParsePkgPath(pass.Pkg.Path())
	if currentInfo == nil {
		return nil, nil
	}

	for _, file := range pass.Files {
		for _, imp := range file.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			importInfo := convention.ParsePkgPath(importPath)
			if importInfo == nil {
				continue
			}

			if convention.SameModule(pass.Pkg.Path(), importPath) {
				continue
			}

			if importInfo.Kind == convention.KindInternal {
				pass.Reportf(imp.Pos(),
					"dddcrossmodule: package %q is internal to module %q, "+
						"cross-module import from module %q is forbidden; "+
						"use command/query/event for cross-module communication",
					importPath, importInfo.ModuleName, currentInfo.ModuleName)
			}
		}
	}

	return nil, nil
}