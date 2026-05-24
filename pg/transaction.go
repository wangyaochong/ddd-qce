package pg

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
)

type txKey struct{}

type txState struct {
	tx      *sql.Tx
	depth   int
	aborted bool
	mu      sync.Mutex
}

type PgTransactionManager struct {
	db *sql.DB
}

func NewTransactionManager(db *sql.DB) *PgTransactionManager {
	return &PgTransactionManager{db: db}
}

func (m *PgTransactionManager) Begin(ctx context.Context) (context.Context, error) {
	if state, ok := ctx.Value(txKey{}).(*txState); ok {
		state.mu.Lock()
		defer state.mu.Unlock()
		state.depth++
		spName := fmt.Sprintf("sp_%d", state.depth)
		if _, err := state.tx.ExecContext(ctx, "SAVEPOINT "+spName); err != nil {
			state.depth--
			return nil, fmt.Errorf("create savepoint: %w", err)
		}
		return ctx, nil
	}

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	state := &txState{tx: tx, depth: 1}
	return context.WithValue(ctx, txKey{}, state), nil
}

func (m *PgTransactionManager) Commit(ctx context.Context) error {
	state, ok := ctx.Value(txKey{}).(*txState)
	if !ok {
		return fmt.Errorf("no transaction in context")
	}
	state.mu.Lock()
	defer state.mu.Unlock()

	if state.depth <= 0 {
		return fmt.Errorf("no transaction in context")
	}

	spName := fmt.Sprintf("sp_%d", state.depth)
	state.depth--

	if state.depth > 0 {
		if state.aborted {
			if _, rbErr := state.tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT "+spName); rbErr != nil {
				return fmt.Errorf("rollback to savepoint: %w (transaction was already aborted)", rbErr)
			}
			return fmt.Errorf("transaction aborted by inner rollback")
		}
		_, err := state.tx.ExecContext(ctx, "RELEASE SAVEPOINT "+spName)
		return err
	}

	if state.aborted {
		return state.tx.Rollback()
	}
	return state.tx.Commit()
}

func (m *PgTransactionManager) Rollback(ctx context.Context) error {
	state, ok := ctx.Value(txKey{}).(*txState)
	if !ok {
		return fmt.Errorf("no transaction in context")
	}
	state.mu.Lock()
	defer state.mu.Unlock()

	if state.depth <= 0 {
		return fmt.Errorf("no transaction in context")
	}

	state.aborted = true
	spName := fmt.Sprintf("sp_%d", state.depth)
	state.depth--

	if state.depth > 0 {
		_, err := state.tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT "+spName)
		return err
	}
	return state.tx.Rollback()
}

func GetQuerier(ctx context.Context, db *sql.DB) DBTX {
	if state, ok := ctx.Value(txKey{}).(*txState); ok {
		return state.tx
	}
	return db
}

type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}
