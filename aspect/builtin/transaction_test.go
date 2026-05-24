package builtin

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/cqrs/command"
	commandmemory "github.com/ddd-qce/core/cqrs/command/memory"
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

func (h *testTxHandler) Handle(ctx context.Context, cmd *testTxCommand) (*testTxResult, error) {
	if h.fail {
		return nil, errors.New("handler failed")
	}
	return &testTxResult{Message: "success"}, nil
}

func TestTransactionAspect_Success_Commits(t *testing.T) {
	txMgr := &mockTxManager{}
	txAspect := &TransactionAspect{TxManager: txMgr}

	chain := aspect.NewAspectChain()
	chain.RegisterCommandAspect(txAspect)

	bus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	commandmemory.RegisterCommand(bus, &testTxHandler{fail: false})

	ctx := context.Background()
	_, err := command.Dispatch[*testTxCommand, *testTxResult](ctx, bus, &testTxCommand{})
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
	txAspect := &TransactionAspect{TxManager: txMgr}

	chain := aspect.NewAspectChain()
	chain.RegisterCommandAspect(txAspect)

	bus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	commandmemory.RegisterCommand(bus, &testTxHandler{fail: true})

	ctx := context.Background()
	_, err := command.Dispatch[*testTxCommand, *testTxResult](ctx, bus, &testTxCommand{})
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
	txAspect := &TransactionAspect{TxManager: txMgr}

	chain := aspect.NewAspectChain()
	chain.RegisterCommandAspect(txAspect)

	bus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	commandmemory.RegisterCommand(bus, &testTxHandler{fail: true})

	ctx := context.Background()
	_, err := command.Dispatch[*testTxCommand, *testTxResult](ctx, bus, &testTxCommand{})
	if err == nil {
		t.Fatal("expected error")
	}

	errMsg := err.Error()
	if errMsg == "" {
		t.Error("expected non-empty error message")
	}
}

func TestTransactionAspect_Query_DoesNothing(t *testing.T) {
	txMgr := &mockTxManager{}
	txAspect := &TransactionAspect{TxManager: txMgr}

	chain := aspect.NewAspectChain()
	chain.RegisterQueryAspect(txAspect)

	ctx := context.Background()
	newCtx, err := txAspect.BeforeQuery(ctx, "test query")
	if err != nil {
		t.Fatalf("BeforeQuery failed: %v", err)
	}
	if newCtx != ctx {
		t.Error("BeforeQuery should return same context")
	}

	err = txAspect.AfterQuery(ctx, "test query", "result", nil, time.Millisecond)
	if err != nil {
		t.Fatalf("AfterQuery failed: %v", err)
	}

	begin, commit, rollback := txMgr.counts()
	if begin != 0 || commit != 0 || rollback != 0 {
		t.Errorf("TransactionAspect should not call any TxManager methods for queries, got begin=%d commit=%d rollback=%d", begin, commit, rollback)
	}
}

func TestTransactionAspect_Event_DoesNothing(t *testing.T) {
	txMgr := &mockTxManager{}
	txAspect := &TransactionAspect{TxManager: txMgr}

	ctx := context.Background()
	newCtx, err := txAspect.BeforePublish(ctx, "test event")
	if err != nil {
		t.Fatalf("BeforePublish failed: %v", err)
	}
	if newCtx != ctx {
		t.Error("BeforePublish should return same context")
	}

	err = txAspect.AfterPublish(ctx, "test event", nil, time.Millisecond)
	if err != nil {
		t.Fatalf("AfterPublish failed: %v", err)
	}

	begin, commit, rollback := txMgr.counts()
	if begin != 0 || commit != 0 || rollback != 0 {
		t.Errorf("TransactionAspect should not call any TxManager methods for events, got begin=%d commit=%d rollback=%d", begin, commit, rollback)
	}
}

func TestTransactionAspect_BeginError(t *testing.T) {
	txMgr := &mockTxManager{
		beginErr: errors.New("begin failed"),
	}
	txAspect := &TransactionAspect{TxManager: txMgr}

	chain := aspect.NewAspectChain()
	chain.RegisterCommandAspect(txAspect)

	bus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	commandmemory.RegisterCommand(bus, &testTxHandler{fail: false})

	ctx := context.Background()
	_, err := command.Dispatch[*testTxCommand, *testTxResult](ctx, bus, &testTxCommand{})
	if err == nil {
		t.Fatal("expected error from Begin")
	}
	if err.Error() != "begin failed" {
		t.Errorf("expected 'begin failed', got '%v'", err)
	}
}
