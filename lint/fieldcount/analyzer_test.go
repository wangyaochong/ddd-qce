package fieldcount_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/ddd-qce/core/lint/fieldcount"
)

func TestAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, fieldcount.Analyzer, "myproject/ddd/order/command", "myproject/ddd/order/query", "myproject/ddd/order/event")
}