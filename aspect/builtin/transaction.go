package builtin

import (
	"context"
	"fmt"
	"time"
)

type TransactionManager interface {
	Begin(ctx context.Context) (context.Context, error)
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type NoOpTransactionManager struct{}

func NewNoOpTransactionManager() *NoOpTransactionManager {
	return &NoOpTransactionManager{}
}

func (t *NoOpTransactionManager) Begin(ctx context.Context) (context.Context, error) {
	return ctx, nil
}

func (t *NoOpTransactionManager) Commit(ctx context.Context) error {
	return nil
}

func (t *NoOpTransactionManager) Rollback(ctx context.Context) error {
	return nil
}

var _ TransactionManager = (*NoOpTransactionManager)(nil)

type TransactionAspect struct {
	txManager TransactionManager
}

func NewTransactionAspect(txManager TransactionManager) (*TransactionAspect, error) {
	if txManager == nil {
		return nil, fmt.Errorf("transaction: TxManager must not be nil, use NoOpTransactionManager for non-transactional scenarios")
	}
	return &TransactionAspect{txManager: txManager}, nil
}

func (t *TransactionAspect) GetTxManager() TransactionManager { return t.txManager }

func (t *TransactionAspect) Name() string {
	return "transaction"
}

func (t *TransactionAspect) Order() int {
	return 10
}

func (t *TransactionAspect) BeforeQuery(ctx context.Context, query any) (context.Context, error) {
	return ctx, nil
}

func (t *TransactionAspect) AfterQuery(ctx context.Context, query any, result any, err error, duration time.Duration) error {
	return nil
}

func (t *TransactionAspect) BeforeCommand(ctx context.Context, cmd any) (context.Context, error) {
	return t.txManager.Begin(ctx)
}

func (t *TransactionAspect) AfterCommand(ctx context.Context, cmd any, result any, err error, duration time.Duration) error {
	if err != nil {
		if rbErr := t.txManager.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("command failed: %v, rollback failed: %v", err, rbErr)
		}
		return err
	}
	return t.txManager.Commit(ctx)
}

func (t *TransactionAspect) BeforePublish(ctx context.Context, event any) (context.Context, error) {
	return ctx, nil
}

func (t *TransactionAspect) AfterPublish(ctx context.Context, event any, err error, duration time.Duration) error {
	return nil
}
