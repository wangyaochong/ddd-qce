package infrastructure

import (
	"context"
	"log"
	"sync"
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
	mu      sync.RWMutex
	Records []TxRecord
}

func NewAppTransactionManager() *AppTransactionManager {
	return &AppTransactionManager{}
}

func (m *AppTransactionManager) Begin(ctx context.Context) (context.Context, error) {
	m.mu.Lock()
	m.Records = append(m.Records, TxRecord{Action: TxBegin})
	m.mu.Unlock()
	log.Printf("[Transaction] BEGIN")
	return ctx, nil
}

func (m *AppTransactionManager) Commit(ctx context.Context) error {
	m.mu.Lock()
	m.Records = append(m.Records, TxRecord{Action: TxCommit})
	m.mu.Unlock()
	log.Printf("[Transaction] COMMIT")
	return nil
}

func (m *AppTransactionManager) Rollback(ctx context.Context) error {
	m.mu.Lock()
	m.Records = append(m.Records, TxRecord{Action: TxRollback})
	m.mu.Unlock()
	log.Printf("[Transaction] ROLLBACK")
	return nil
}
