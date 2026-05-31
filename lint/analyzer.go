package lint

import (
	"github.com/ddd-qce/core/lint/crossdomain"
	"github.com/ddd-qce/core/lint/implimport"
	"github.com/ddd-qce/core/lint/infraimplement"
	"github.com/ddd-qce/core/lint/publicleak"
	"github.com/ddd-qce/core/lint/wirecomplexity"
	"github.com/ddd-qce/core/lint/wireimport"
	"github.com/ddd-qce/core/lint/wirelogic"
	"golang.org/x/tools/go/analysis"
)

var AllAnalyzers = []*analysis.Analyzer{
	crossdomain.Analyzer,
	publicleak.Analyzer,
	implimport.Analyzer,
	wirecomplexity.Analyzer,
	wireimport.Analyzer,
	wirelogic.Analyzer,
	infraimplement.Analyzer,
}
