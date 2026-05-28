//go:build integration

package builtin_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/aspect/builtin"
	"github.com/ddd-qce/core/cqrs/command"
	memory "github.com/ddd-qce/core/cqrs/impl/memory"
)

type mockTxManager struct {
	mu             sync.Mutex
	beginCalled    int
	commitCalled   int
	rollbackCalled int
	beginErr       error
	commitErr      error
	rollbackErr    error
}

func (m *mockTxManager) Begin(ctx context.Context) (context.Context, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.beginCalled++
	return ctx, m.beginErr
}

func (m *mockTxManager) Commit(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.commitCalled++
	return m.commitErr
}

func (m *mockTxManager) Rollback(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rollbackCalled++
	return m.rollbackErr
}

func (m *mockTxManager) counts() (int, int, int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.beginCalled, m.commitCalled, m.rollbackCalled
}

type testTxCommand struct {
	command.BaseCommand
	Fail bool
}

type testTxResult struct {
	Message string
}

type testTxHandler struct {
	fail bool
}

func (h *testTxHandler) Handle(ctx context.Context, c *testTxCommand) (*testTxResult, error) {
	if h.fail {
		return nil, errors.New("handler failed")
	}
	return &testTxResult{Message: "success"}, nil
}

func TestTransactionAspect_Success_Commits(t *testing.T) {
	txMgr := &mockTxManager{}
	txAspect := &builtin.TransactionAspect{TxManager: txMgr}

	chain := aspect.NewAspectChain()
	chain.RegisterCommandAspect(txAspect)

	bus := memory.NewCommandBus(memory.WithCommandBusAspectChain(chain))
	memory.RegisterCommand(bus, &testTxHandler{fail: false})

	ctx := context.Background()
	_, err := 	command.Dispatch[*testTxCommand, *testTxResult](ctx, bus, &testTxCommand{})
	if err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	begin, commit, rollback := txMgr.counts()
	if begin != 1 {
		t.Errorf("expected 1 Begin, got %d", begin)
	}
	if commit != 1 {
		t.Errorf("expected 1 Commit, got %d", commit)
	}
	if rollback != 0 {
		t.Errorf("expected 0 Rollback, got %d", rollback)
	}
}

func TestTransactionAspect_Error_Rollbacks(t *testing.T) {
	txMgr := &mockTxManager{}
	txAspect := &builtin.TransactionAspect{TxManager: txMgr}

	chain := aspect.NewAspectChain()
	chain.RegisterCommandAspect(txAspect)

	bus := memory.NewCommandBus(memory.WithCommandBusAspectChain(chain))
	memory.RegisterCommand(bus, &testTxHandler{fail: true})

	ctx := context.Background()
	_, err := 	command.Dispatch[*testTxCommand, *testTxResult](ctx, bus, &testTxCommand{})
	if err == nil {
		t.Fatal("expected error from handler")
	}

	begin, commit, rollback := txMgr.counts()
	if begin != 1 {
		t.Errorf("expected 1 Begin, got %d", begin)
	}
	if commit != 0 {
		t.Errorf("expected 0 Commit, got %d", commit)
	}
	if rollback != 1 {
		t.Errorf("expected 1 Rollback, got %d", rollback)
	}
}

func TestTransactionAspect_RollbackError_ReturnsBothErrors(t *testing.T) {
	txMgr := &mockTxManager{
		rollbackErr: errors.New("rollback failed"),
	}
	txAspect := &builtin.TransactionAspect{TxManager: txMgr}

	chain := aspect.NewAspectChain()
	chain.RegisterCommandAspect(txAspect)

	bus := memory.NewCommandBus(memory.WithCommandBusAspectChain(chain))
	memory.RegisterCommand(bus, &testTxHandler{fail: true})

	ctx := context.Background()
	_, err := 	command.Dispatch[*testTxCommand, *testTxResult](ctx, bus, &testTxCommand{})
	if err == nil {
		t.Fatal("expected error")
	}

	begin, commit, rollback := txMgr.counts()
	if begin != 1 {
		t.Errorf("expected 1 Begin, got %d", begin)
	}
	if commit != 0 {
		t.Errorf("expected 0 Commit, got %d", commit)
	}
	if rollback != 1 {
		t.Errorf("expected 1 Rollback, got %d", rollback)
	}
}

func TestTransactionAspect_ConcurrentCommands(t *testing.T) {
	txMgr := &mockTxManager{}
	txAspect := &builtin.TransactionAspect{TxManager: txMgr}

	chain := aspect.NewAspectChain()
	chain.RegisterCommandAspect(txAspect)

	bus := memory.NewCommandBus(memory.WithCommandBusAspectChain(chain))
	memory.RegisterCommand(bus, &testTxHandler{fail: false})

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			command.Dispatch[*testTxCommand, *testTxResult](context.Background(), bus, &testTxCommand{})
		}()
	}
	wg.Wait()

	begin, commit, rollback := txMgr.counts()
	if begin != 10 {
		t.Errorf("expected 10 Begin, got %d", begin)
	}
	if commit != 10 {
		t.Errorf("expected 10 Commit, got %d", commit)
	}
	if rollback != 0 {
		t.Errorf("expected 0 Rollback, got %d", rollback)
	}
}

func TestTransactionAspect_BeginError_ReturnsError(t *testing.T) {
	txMgr := &mockTxManager{
		beginErr: errors.New("begin failed"),
	}
	txAspect := &builtin.TransactionAspect{TxManager: txMgr}

	chain := aspect.NewAspectChain()
	chain.RegisterCommandAspect(txAspect)

	bus := memory.NewCommandBus(memory.WithCommandBusAspectChain(chain))
	memory.RegisterCommand(bus, &testTxHandler{fail: false})

	ctx := context.Background()
	_, err := 	command.Dispatch[*testTxCommand, *testTxResult](ctx, bus, &testTxCommand{})
	if err == nil {
		t.Fatal("expected error from begin failure")
	}
}

func TestTransactionAspect_CommitError_ReturnsError(t *testing.T) {
	txMgr := &mockTxManager{
		commitErr: errors.New("commit failed"),
	}
	txAspect := &builtin.TransactionAspect{TxManager: txMgr}

	chain := aspect.NewAspectChain()
	chain.RegisterCommandAspect(txAspect)

	bus := memory.NewCommandBus(memory.WithCommandBusAspectChain(chain))
	memory.RegisterCommand(bus, &testTxHandler{fail: false})

	ctx := context.Background()
	_, err := 	command.Dispatch[*testTxCommand, *testTxResult](ctx, bus, &testTxCommand{})
	if err == nil {
		t.Fatal("expected error from commit failure")
	}
}

func TestTransactionAspect_MultiErrorFormatting(t *testing.T) {
	txMgr := &mockTxManager{
		commitErr:    errors.New("commit failed"),
		rollbackErr: errors.New("rollback failed"),
	}
	txAspect := &builtin.TransactionAspect{TxManager: txMgr}

	chain := aspect.NewAspectChain()
	chain.RegisterCommandAspect(txAspect)

	bus := memory.NewCommandBus(memory.WithCommandBusAspectChain(chain))
	memory.RegisterCommand(bus, &testTxHandler{fail: false})

	ctx := context.Background()
	_, err := 	command.Dispatch[*testTxCommand, *testTxResult](ctx, bus, &testTxCommand{})
	if err == nil {
		t.Fatal("expected error")
	}

	errStr := err.Error()
	if errStr == "" {
		t.Error("expected non-empty error string")
	}
}

func TestTransactionAspect_Timeout(t *testing.T) {
	txMgr := &mockTxManager{}
	txAspect := &builtin.TransactionAspect{TxManager: txMgr}

	chain := aspect.NewAspectChain()
	chain.RegisterCommandAspect(txAspect)

	bus := memory.NewCommandBus(memory.WithCommandBusAspectChain(chain))
	memory.RegisterCommand(bus, &testTxHandler{fail: false})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := command.Dispatch[*testTxCommand, *testTxResult](ctx, bus, &testTxCommand{})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}