package pg

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
)

type txKey struct{}

type txState struct {
	tx        *sql.Tx
	depth     int
	aborted   bool
	mu        sync.Mutex
	nextSpSeq int
	parentSp  []int
}

// PgTransactionManager provides PostgreSQL transaction management with nested transaction support via savepoints.
type PgTransactionManager struct {
	db *sql.DB
}

// NewTransactionManager creates a PgTransactionManager backed by the given database connection.
func NewTransactionManager(db *sql.DB) *PgTransactionManager {
	return &PgTransactionManager{db: db}
}

func (m *PgTransactionManager) Begin(ctx context.Context) (context.Context, error) {
	if state, ok := ctx.Value(txKey{}).(*txState); ok {
		state.mu.Lock()
		defer state.mu.Unlock()
		state.depth++
		state.nextSpSeq++
		spSeq := state.nextSpSeq
		state.parentSp = append(state.parentSp, spSeq)
		spName := fmt.Sprintf("sp_%d", spSeq)
		if _, err := state.tx.ExecContext(ctx, "SAVEPOINT "+spName); err != nil {
			state.depth--
			state.nextSpSeq--
			state.parentSp = state.parentSp[:len(state.parentSp)-1]
			return nil, fmt.Errorf("create savepoint: %w", err)
		}
		state.aborted = false
		return ctx, nil
	}

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	state := &txState{tx: tx, depth: 1, nextSpSeq: 1, parentSp: []int{1}}
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

	spSeq := state.parentSp[len(state.parentSp)-1]
	state.parentSp = state.parentSp[:len(state.parentSp)-1]
	spName := fmt.Sprintf("sp_%d", spSeq)
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
		_ = state.tx.Rollback()
		return fmt.Errorf("transaction aborted by inner rollback")
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
	spSeq := state.parentSp[len(state.parentSp)-1]
	state.parentSp = state.parentSp[:len(state.parentSp)-1]
	spName := fmt.Sprintf("sp_%d", spSeq)
	state.depth--

	if state.depth > 0 {
		_, err := state.tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT "+spName)
		return err
	}
	return state.tx.Rollback()
}

// HasTransaction returns true if ctx carries an active transaction.
func HasTransaction(ctx context.Context) bool {
	_, ok := ctx.Value(txKey{}).(*txState)
	return ok
}

// GetQuerier returns the active transaction from ctx, or falls back to the raw DB.
func GetQuerier(ctx context.Context, db *sql.DB) DBTX {
	if state, ok := ctx.Value(txKey{}).(*txState); ok {
		return state.tx
	}
	return db
}

// DBTX abstracts *sql.DB and *sql.Tx so repositories can use either interchangeably.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}
