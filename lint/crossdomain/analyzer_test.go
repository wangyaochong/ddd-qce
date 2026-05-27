package crossdomain_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/ddd-qce/core/lint/crossdomain"
)

func TestAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, crossdomain.Analyzer, "myproject/ddd/order/command")
}
