package crossdomain

import (
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/ddd-qce/core/lint/convention"
)

var Analyzer = &analysis.Analyzer{
	Name: "dddcrossdomain",
	Doc:  "check that DDD domains do not import internal packages from other domains",
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

			if currentInfo.DDDPrefix == importInfo.DDDPrefix &&
				currentInfo.DomainName == importInfo.DomainName {
				continue
			}

			if importInfo.Kind == convention.KindInternal {
				pass.Reportf(imp.Pos(),
					"dddcrossdomain: package %q is internal to domain %q, "+
						"import from domain %q is forbidden; use command/query/event for cross-domain communication",
					importPath, importInfo.DomainName, currentInfo.DomainName)
			}
		}
	}

	return nil, nil
}
