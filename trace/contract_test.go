package trace_test

import (
	"testing"

	"github.com/ddd-qce/core/trace"
	"github.com/ddd-qce/core/trace/tracetest"
)

func TestInMemoryTraceStore_Contract(t *testing.T) {
	tracetest.TestTraceStoreContract(t, func() trace.TraceStore {
		return trace.NewInMemoryTraceStore()
	})
}
