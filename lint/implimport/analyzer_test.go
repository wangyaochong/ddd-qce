package implimport_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/ddd-qce/core/lint/implimport"
)

func TestAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, implimport.Analyzer, "myproject/ddd/order/command", "myproject/ddd/order/wire")
}
