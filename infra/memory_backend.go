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
	depth   int
	aborted bool
}

type MemoryTransactionManager struct {
	mu sync.Mutex
}

func NewMemoryTransactionManager() *MemoryTransactionManager {
	return &MemoryTransactionManager{}
}

func (m *MemoryTransactionManager) Begin(ctx context.Context) (context.Context, error) {
	if state, ok := ctx.Value(memTxKey{}).(*memTxState); ok {
		m.mu.Lock()
		state.depth++
		m.mu.Unlock()
		return ctx, nil
	}
	state := &memTxState{depth: 1}
	return context.WithValue(ctx, memTxKey{}, state), nil
}

func (m *MemoryTransactionManager) Commit(ctx context.Context) error {
	state, ok := ctx.Value(memTxKey{}).(*memTxState)
	if !ok || state.depth <= 0 {
		return fmt.Errorf("no transaction in context")
	}
	m.mu.Lock()
	state.depth--
	m.mu.Unlock()

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
	if !ok || state.depth <= 0 {
		return fmt.Errorf("no transaction in context")
	}
	m.mu.Lock()
	state.aborted = true
	state.depth--
	m.mu.Unlock()
	return nil
}

func NewMemoryBackend(opts ...BackendOption) *Backend {
	defaults := []BackendOption{
		WithTransactionManager(NewMemoryTransactionManager()),
		WithJobStore(jobmemory.NewJobStore()),
		WithTraceStore(trace.NewInMemoryTraceStore()),
		WithMessageStore(builtin.NewNopMessageStore()),
		WithMigrate(func() error { return nil }),
	}
	return NewBackend(append(defaults, opts...)...)
}
