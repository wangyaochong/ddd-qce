// Package eventimmutable enforces that BaseEvent fields (AggregateID,
// OccurredAt, CorrelationID, CausationID) are not mutated after an event is
// constructed. Use event.NewDomainEvent, event.WithCorrelation, or composite
// literals to set these fields; once the event is in the domain/event
// pipeline, the metadata must be treated as immutable.
//
// Rationale: event metadata is used for cross-aggregate tracing, audit logs,
// and event store persistence. Silent mutation after publish or after
// LoadFromHistory can cause split-brain, lost causation chains, and corrupted
// event store records.
package eventimmutable

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/ddd-qce/core/lint/convention"
)

const baseEventPkgPath = "github.com/ddd-qce/core/cqrs/event"

// baseEventFieldNames is the set of metadata field names whose direct
// assignment is forbidden. Detection is type-driven (see isBaseEventField),
// so renaming these fields only requires updating this slice.
var baseEventFieldNames = map[string]bool{
	"AggregateID":   true,
	"OccurredAt":    true,
	"CorrelationID": true,
	"CausationID":   true,
}

var Analyzer = &analysis.Analyzer{
	Name:     "dddeventimmutable",
	Doc:      "check that BaseEvent fields are not mutated after event construction",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	currentInfo := convention.ParsePkgPath(pass.Pkg.Path())
	if currentInfo == nil {
		return nil, nil
	}

	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	nodeFilter := []ast.Node{
		(*ast.AssignStmt)(nil),
	}

	insp.Preorder(nodeFilter, func(n ast.Node) {
		stmt := n.(*ast.AssignStmt)
		for _, lhs := range stmt.Lhs {
			sel, ok := lhs.(*ast.SelectorExpr)
			if !ok {
				continue
			}
			if !baseEventFieldNames[sel.Sel.Name] {
				continue
			}
			selection, ok := pass.TypesInfo.Selections[sel]
			if !ok {
				continue
			}
			if !isBaseEventField(selection) {
				continue
			}
			pass.Reportf(sel.Pos(),
				"dddeventimmutable: BaseEvent field %q must not be mutated; "+
					"construct events via event.NewDomainEvent / event.WithCorrelation, "+
					"or use a composite literal &XxxEvent{BaseEvent: ...}",
				sel.Sel.Name)
		}
	})
	return nil, nil
}

// isBaseEventField returns true if the selection resolves to a field
// declared in the framework's BaseEvent type. Field renames inside the
// framework are picked up automatically because we walk type information
// rather than string names of the embedding struct.
func isBaseEventField(selection *types.Selection) bool {
	if selection == nil {
		return false
	}
	if selection.Kind() != types.FieldVal {
		return false
	}
	obj := selection.Obj()
	if obj == nil {
		return false
	}
	pkg := obj.Pkg()
	if pkg == nil {
		return false
	}
	return pkg.Path() == baseEventPkgPath
}
