package fieldcount

import (
	"flag"
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/ddd-qce/core/lint/convention"
)

var maxFields int

var Analyzer = &analysis.Analyzer{
	Name:     "dddfieldcount",
	Doc:      "check that Command, Query, and Event types have at most 5 fields",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Flags:    flags(),
	Run:      run,
}

func flags() flag.FlagSet {
	var fs flag.FlagSet
	fs.IntVar(&maxFields, "max-fields", 5, "maximum number of fields allowed in Command, Query, and Event types")
	return fs
}

func run(pass *analysis.Pass) (interface{}, error) {
	pkgInfo := convention.ParsePkgPath(pass.Pkg.Path())
	if pkgInfo == nil || pkgInfo.Kind != convention.KindPublic {
		return nil, nil
	}

	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{
		(*ast.TypeSpec)(nil),
	}

	insp.Preorder(nodeFilter, func(n ast.Node) {
		typeSpec := n.(*ast.TypeSpec)
		structType, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			return
		}

		typeName := typeSpec.Name.Name

		var kind string
		switch pkgInfo.SubPkg {
		case "command":
			if strings.HasSuffix(typeName, "Command") {
				kind = "command"
			}
		case "query":
			if strings.HasSuffix(typeName, "Query") {
				kind = "query"
			}
		case "event":
			if strings.HasSuffix(typeName, "Event") {
				kind = "event"
			}
		}

		if kind == "" {
			return
		}

		if strings.HasSuffix(typeName, "Result") {
			return
		}

		fieldCount := countFields(structType)
		if fieldCount > maxFields {
			pass.Reportf(typeSpec.Pos(),
				"dddfieldcount: %s type %q has %d fields, maximum allowed is %d; "+
					"consider extracting fields into a value object or splitting the %s",
				kind, typeName, fieldCount, maxFields, kind)
		}
	})

	return nil, nil
}

func countFields(structType *ast.StructType) int {
	if structType.Fields == nil {
		return 0
	}

	count := 0
	for _, field := range structType.Fields.List {
		if len(field.Names) == 0 {
			continue
		}
		count++
	}
	return count
}