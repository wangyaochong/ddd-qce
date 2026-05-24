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

type TransactionAspect struct {
	TxManager TransactionManager
}

func NewTransactionAspect(txManager TransactionManager) *TransactionAspect {
	return &TransactionAspect{TxManager: txManager}
}

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
	return t.TxManager.Begin(ctx)
}

func (t *TransactionAspect) AfterCommand(ctx context.Context, cmd any, result any, err error, duration time.Duration) error {
	if err != nil {
		if rbErr := t.TxManager.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("command failed: %v, rollback failed: %v", err, rbErr)
		}
		return err
	}
	return t.TxManager.Commit(ctx)
}

func (t *TransactionAspect) BeforePublish(ctx context.Context, event any) (context.Context, error) {
	return ctx, nil
}

func (t *TransactionAspect) AfterPublish(ctx context.Context, event any, err error, duration time.Duration) error {
	return nil
}
