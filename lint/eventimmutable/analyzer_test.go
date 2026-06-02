package eventimmutable_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/ddd-qce/core/lint/eventimmutable"
)

func TestAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, eventimmutable.Analyzer, "myproject/ddd/order/event")
}
