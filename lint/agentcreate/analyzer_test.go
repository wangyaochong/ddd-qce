package agentcreate_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/ddd-qce/core/lint/agentcreate"
)

func TestAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, agentcreate.Analyzer, "myproject/infra")
}

func TestAnalyzer_CmdServerWhitelist(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, agentcreate.Analyzer, "myproject/cmd/server")
}

func TestAnalyzer_DomainWhitelist(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, agentcreate.Analyzer, "myproject/ddd/agent/command")
}
