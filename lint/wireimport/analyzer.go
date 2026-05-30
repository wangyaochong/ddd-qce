package wireimport

import (
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/ddd-qce/core/lint/convention"
)

var Analyzer = &analysis.Analyzer{
	Name: "dddwireimport",
	Doc:  "check that wire packages only import allowed packages",
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
		for _, imp := range file.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)

			if !isAllowedImport(importPath, currentInfo) {
				pass.Reportf(imp.Pos(),
					"dddwireimport: import %q is not allowed in wire package; "+
						"wire can only import cqrs/impl/*, other wire packages, standard library, "+
						"or packages from the same domain",
					importPath)
			}
		}
	}

	return nil, nil
}

func isAllowedImport(importPath string, currentInfo *convention.PkgInfo) bool {
	// Allow standard library
	if !strings.Contains(importPath, ".") {
		return true
	}

	// Allow cqrs/impl/* (memory implementations, etc.)
	if strings.Contains(importPath, "/cqrs/impl/") {
		return true
	}

	// Allow ddd-qce/core packages (cqrs interfaces, domain interfaces, etc.)
	if strings.HasPrefix(importPath, "github.com/ddd-qce/core/") {
		return true
	}

	// Parse the import path to check if it's a DDD package
	importInfo := convention.ParsePkgPath(importPath)
	if importInfo == nil {
		// Non-DDD package, allow (could be external library)
		return true
	}

	// Allow same domain packages
	if currentInfo.DDDPrefix == importInfo.DDDPrefix &&
		currentInfo.DomainName == importInfo.DomainName {
		return true
	}

	// Allow other domains' wire packages
	if importInfo.Kind == convention.KindWire {
		return true
	}

	// Forbid importing other domains' internal packages
	return false
}
