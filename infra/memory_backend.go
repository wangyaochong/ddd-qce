package infra

import (
	"context"
	"fmt"
	"sync"

	"github.com/ddd-qce/core/aspect/builtin"
	jobmemory "github.com/ddd-qce/core/job/memory"
	"github.com/ddd-qce/core/trace"
)

type memTxKey struct{}

type memTxState struct {
	mu      sync.Mutex
	depth   int
	aborted bool
}

// MemoryTransactionManager provides in-memory transaction semantics with nested transaction support.
type MemoryTransactionManager struct{}

// NewMemoryTransactionManager creates a new MemoryTransactionManager.
func NewMemoryTransactionManager() *MemoryTransactionManager {
	return &MemoryTransactionManager{}
}

func (m *MemoryTransactionManager) Begin(ctx context.Context) (context.Context, error) {
	if state, ok := ctx.Value(memTxKey{}).(*memTxState); ok {
		state.mu.Lock()
		state.depth++
		state.mu.Unlock()
		return ctx, nil
	}
	state := &memTxState{depth: 1}
	return context.WithValue(ctx, memTxKey{}, state), nil
}

func (m *MemoryTransactionManager) Commit(ctx context.Context) error {
	state, ok := ctx.Value(memTxKey{}).(*memTxState)
	if !ok {
		return fmt.Errorf("no transaction in context")
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.depth <= 0 {
		return fmt.Errorf("no transaction in context")
	}
	state.depth--

	if state.depth > 0 {
		if state.aborted {
			return fmt.Errorf("transaction aborted by inner rollback")
		}
		return nil
	}
	if state.aborted {
		return fmt.Errorf("transaction aborted by inner rollback")
	}
	return nil
}

func (m *MemoryTransactionManager) Rollback(ctx context.Context) error {
	state, ok := ctx.Value(memTxKey{}).(*memTxState)
	if !ok {
		return fmt.Errorf("no transaction in context")
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.depth <= 0 {
		return fmt.Errorf("no transaction in context")
	}
	state.aborted = true
	state.depth--
	return nil
}

// NewMemoryBackend creates a Backend with all in-memory defaults.
func NewMemoryBackend(opts ...BackendOption) *Backend {
	defaults := []BackendOption{
		WithTransactionManager(NewMemoryTransactionManager()),
		WithJobStore(jobmemory.NewJobStore()),
		WithTraceStore(trace.NewInMemoryTraceStore()),
		WithMessageStore(builtin.NewInMemoryMessageStore()),
		WithMigrator(NopMigrator{}),
		WithBusFactory(NewMemoryBusFactory()),
	}
	return NewBackend(append(defaults, opts...)...)
}
