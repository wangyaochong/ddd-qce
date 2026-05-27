package infrastructure

import (
	"context"
	"log"
	"sync"

	"github.com/ddd-qce/core/aspect/builtin"
)

type TxAction string

const (
	TxBegin    TxAction = "begin"
	TxCommit   TxAction = "commit"
	TxRollback TxAction = "rollback"
)

type TxRecord struct {
	Action TxAction
}

type AppTransactionManager struct {
	mu       sync.RWMutex
	Records  []TxRecord
	delegate builtin.TransactionManager
}

func NewAppTransactionManager(delegate builtin.TransactionManager) *AppTransactionManager {
	return &AppTransactionManager{delegate: delegate}
}

func (m *AppTransactionManager) Begin(ctx context.Context) (context.Context, error) {
	ctx, err := m.delegate.Begin(ctx)
	if err != nil {
		return ctx, err
	}
	m.mu.Lock()
	m.Records = append(m.Records, TxRecord{Action: TxBegin})
	m.mu.Unlock()
	log.Printf("[Transaction] BEGIN")
	return ctx, nil
}

func (m *AppTransactionManager) Commit(ctx context.Context) error {
	err := m.delegate.Commit(ctx)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.Records = append(m.Records, TxRecord{Action: TxCommit})
	m.mu.Unlock()
	log.Printf("[Transaction] COMMIT")
	return nil
}

func (m *AppTransactionManager) Rollback(ctx context.Context) error {
	err := m.delegate.Rollback(ctx)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.Records = append(m.Records, TxRecord{Action: TxRollback})
	m.mu.Unlock()
	log.Printf("[Transaction] ROLLBACK")
	return nil
}
