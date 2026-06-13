package fieldaccess_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/ddd-qce/core/lint/fieldaccess"
)

func TestAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, fieldaccess.Analyzer,
		"myproject/ddd/order/command",
		"myproject/ddd/order/infra/pg",
	)
}
