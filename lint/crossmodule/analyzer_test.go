package crossmodule_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/ddd-qce/core/lint/crossmodule"
)

func TestAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, crossmodule.Analyzer, "module_a/ddd/order/command", "module_a/ddd/order/service")
}
