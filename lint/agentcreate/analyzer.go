package agentcreate

import (
	"go/ast"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

var Analyzer = &analysis.Analyzer{
	Name:     "dddagentcreate",
	Doc:      "check that CreatePendingAgentCommand is only called from PoolDispatcher.dispatchSlot(); use EnqueueSlotCommand instead",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{
		(*ast.SelectorExpr)(nil),
	}

	insp.Preorder(nodeFilter, func(n ast.Node) {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return
		}
		if sel.Sel.Name != "CreatePendingAgentCommand" && sel.Sel.Name != "CreatePendingAgentResult" {
			return
		}

		pos := pass.Fset.Position(sel.Pos())
		filename := filepath.Base(pos.Filename)

		if isWhitelisted(pass.Pkg.Path(), filename) {
			return
		}

		pass.Reportf(sel.Pos(),
			"dddagentcreate: CreatePendingAgentCommand must only be called from PoolDispatcher.dispatchSlot(); use EnqueueSlotCommand instead")
	})

	return nil, nil
}

func isWhitelisted(pkgPath string, filename string) bool {
	if strings.Contains(pkgPath, "/ddd/agent/command") {
		return true
	}
	if strings.Contains(pkgPath, "/cmd/server") {
		return true
	}
	if filename == "pool_dispatcher.go" {
		return true
	}
	if filename == "sample_provider.go" {
		return true
	}
	if strings.HasSuffix(filename, "_test.go") {
		return true
	}
	return false
}
