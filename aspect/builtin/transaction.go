package builtin

import (
	"context"
	"fmt"
	"time"
)

// TransactionManager defines the contract for managing database transactions.
// Implementations handle begin, commit, and rollback lifecycle.
type TransactionManager interface {
	Begin(ctx context.Context) (context.Context, error)
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// NoOpTransactionManager is a no-op TransactionManager for use in contexts
// where transactions are not needed (e.g., testing, in-memory stores).
type NoOpTransactionManager struct{}

// NewNoOpTransactionManager creates a no-op transaction manager.
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

// TransactionAspect wraps command execution in a database transaction.
// It begins a transaction before each command and commits on success or
// rolls back on error. Queries and events are not transactionally managed.
type TransactionAspect struct {
	txManager TransactionManager
}

// NewTransactionAspect creates a TransactionAspect with the given manager.
// Returns an error if txManager is nil; use NoOpTransactionManager for
// non-transactional scenarios.
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
			return fmt.Errorf("command failed: %w, rollback failed: %v", err, rbErr)
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
