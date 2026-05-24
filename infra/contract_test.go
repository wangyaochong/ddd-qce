package infra_test

import (
	"context"
	"testing"

	"github.com/ddd-qce/core/aspect/builtin/builtintest"
	"github.com/ddd-qce/core/infra"
	"github.com/ddd-qce/core/infra/infratest"
)

func TestMemoryTransactionManager_Contract(t *testing.T) {
	tm := infra.NewMemoryTransactionManager()
	builtintest.TestTransactionManagerContract(t, tm, func() context.Context {
		return context.Background()
	})
}

func TestMemoryBackend_Contract(t *testing.T) {
	b := infra.NewMemoryBackend()
	infratest.TestBackendContract(t, b)
}
