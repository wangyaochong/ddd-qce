package publicleak_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/ddd-qce/core/lint/publicleak"
)

func TestAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, publicleak.Analyzer, "myproject/ddd/order/command", "myproject/ddd/order/event", "myproject/ddd/order/query")
}
