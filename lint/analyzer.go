package lint

import (
	"github.com/ddd-qce/core/lint/crossdomain"
	"github.com/ddd-qce/core/lint/implimport"
	"github.com/ddd-qce/core/lint/publicleak"
	"golang.org/x/tools/go/analysis"
)

var AllAnalyzers = []*analysis.Analyzer{
	crossdomain.Analyzer,
	publicleak.Analyzer,
	implimport.Analyzer,
}
